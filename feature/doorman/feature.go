package doorman

import (
	"sync"
	"time"

	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/feature/doorman/core"
	"github.com/shyandsy/aurora/feature/doorman/service"
	"github.com/shyandsy/aurora/logger"
	"gorm.io/gorm"
)

// 决策流水(doorman_decision)是只增日志,doorman 自己负责保留 —— 起后台 goroutine 定期清老数据,
// 不依赖任何项目的 schedule 服务(库自包含,复制即用)。
const (
	defaultDecisionRetention = 90 * 24 * time.Hour // 默认保留 90 天(WithDecisionRetention 可改,<=0 关闭)
	prunePeriod              = 6 * time.Hour       // 每 6 小时清一次
	pruneStartDelay          = time.Minute         // 启动后先等 1 分钟再首刷,避开启动高峰
	pruneBatchSize           = 1000                // 每批删多少(分批不锁大表)
)

// ── aurora Feature 装配 ──
//
// 把 doorman 作为一个 aurora Feature 接入:app.AddFeature(doorman.NewFeature(...)),
// Setup 时装好注册表(内置中性插件 + 业务传入的插件)与看门人,并以 Doorman 接口注入 DI 容器,
// 业务侧 `Doorman doorman.Doorman \`inject:""\`` 直接拿来用。规则来源默认走 DB 表(service 层)。

type featureConfig struct {
	src       RuleSource          // 规则来源;不传 = 用默认 DB 存储
	ttl       time.Duration       // 编译/策略缓存热加载间隔
	conds     []Condition         // 业务补充的条件插件(通用条件已内置)
	actions   map[string][]string // (向后兼容)scope → 动作名清单;新接入用 scopes
	scopes    []ScopeDef          // 业务注册的 scope 定义(标签 + 动作目录 + 动作类型)
	retention time.Duration       // doorman_decision 保留期(<=0 关闭自动清理);默认 defaultDecisionRetention
}

// Option 配置项(功能选项模式)。
type Option func(*featureConfig)

// WithRuleSource 指定规则来源(业务的 DB 表实现)。不传 = 用默认 DB 存储(推荐),此时还会注入 Console(配置页后端)。
// 传了则用业务自带存储(高级 override),不注入 Console / 策略 / 流水。
func WithRuleSource(s RuleSource) Option { return func(c *featureConfig) { c.src = s } }

// WithTTL 指定编译缓存热加载间隔(默认 30s)。
func WithTTL(d time.Duration) Option { return func(c *featureConfig) { c.ttl = d } }

// WithCondition 追加一个业务专属条件插件(通用条件已内置)。doorman 无「动作」插件——动作在业务侧。
func WithCondition(cd Condition) Option {
	return func(c *featureConfig) { c.conds = append(c.conds, cd) }
}

// WithDecisionRetention 设 doorman_decision 的保留期(自动清理早于该时长的决策流水)。
// 不传 = 默认 90 天;传 <=0 = 关闭自动清理(如你想交给外部 job 管)。清理由 feature 自起的后台 goroutine 做。
func WithDecisionRetention(d time.Duration) Option {
	return func(c *featureConfig) { c.retention = d }
}

// WithScope 注册一个 scope 定义:id(业务 Assess 时用的字符串)+ 展示标签 + 该 scope 的动作目录(用 Action 声明)。
// 这是接入 doorman 的**单一注册点**:配置页据它渲染 tab / 「风险→动作」下拉;保存规则/策略据它校验 scope 与动作名;
// 漏斗据动作类型(Friction/Terminal)决定要不要追踪。名字必须和业务代码里的 Assess/动作执行一致(代码是权威)。
//
//	doorman.WithScope("register", "注册",
//	    doorman.Action("email_verify", "要求邮件激活", doorman.ActionFriction),
//	    doorman.Action("block", "直接拦截", doorman.ActionTerminal),
//	)
func WithScope(id, label string, actions ...ActionDef) Option {
	return func(c *featureConfig) {
		c.scopes = append(c.scopes, ScopeDef{ID: id, Label: label, Actions: actions})
	}
}

// Action 声明一个动作(名 + 标签 + 类型),给 WithScope 用。
func Action(name, label string, kind ActionKind) ActionDef {
	return ActionDef{Name: name, Label: label, Kind: kind}
}

// WithActions 向后兼容:只登记动作名(标签回退到名、类型默认 terminal)。新接入请用 WithScope。
func WithActions(scope string, types ...string) Option {
	return func(c *featureConfig) {
		if c.actions == nil {
			c.actions = map[string][]string{}
		}
		c.actions[scope] = append(c.actions[scope], types...)
	}
}

type doormanFeature struct {
	cfg      featureConfig
	stop     chan struct{} // 关闭它 → 停掉后台清理 goroutine(Close 时)
	stopOnce sync.Once
}

// NewFeature 构造 doorman feature。用法(通常直接空参,自建 DB 存储 + 内置条件即可):
//
//	app.AddFeature(doorman.NewFeature())
//	app.AddFeature(doorman.NewFeature(
//	    doorman.WithScope("register", "注册", doorman.Action("email_verify", "要求邮件激活", doorman.ActionFriction)),
//	))
func NewFeature(opts ...Option) contracts.Features {
	fc := featureConfig{retention: defaultDecisionRetention} // 默认开启 90 天保留;WithDecisionRetention 可覆盖/关闭
	for _, o := range opts {
		if o != nil {
			o(&fc)
		}
	}
	return &doormanFeature{cfg: fc, stop: make(chan struct{})}
}

func (f *doormanFeature) Name() string { return "doorman" }

// Close 停掉后台清理 goroutine(编译缓存随 GC,无需释放)。为满足 contracts.Features 接口。
func (f *doormanFeature) Close() error {
	f.stopOnce.Do(func() { close(f.stop) })
	return nil
}

// dbHolder 用注入拿默认存储要用的 *gorm.DB。
type dbHolder struct {
	DB *gorm.DB `inject:""`
}

// Setup 装配注册表(内置 + 业务插件)+ 存储 + 看门人,注入 DI。
//   - 默认(未传 WithRuleSource):用 DB 存储 —— 注入 Doorman + Console(配置页后端),并起后台清理 goroutine。
//     ⚠️ 不自己建表:三张 doorman_* 表由宿主服务的 goose 建(真相源 migrations/doorman_schema.sql,
//     接入方复制进自己**跑 goose 的那个服务**的迁移目录)。
//   - 传了 WithRuleSource:用业务自带存储(高级 override),只注入 Doorman,不注入 Console。
func (f *doormanFeature) Setup(app contracts.App) error {
	reg := core.NewRegistry()
	core.RegisterBuiltins(reg)
	for _, c := range f.cfg.conds {
		reg.RegisterCondition(c)
	}
	for scope, types := range f.cfg.actions { // 向后兼容
		reg.RegisterActions(scope, types)
	}
	for _, s := range f.cfg.scopes {
		reg.RegisterScope(s.ID, s.Label, s.Actions)
	}

	if f.cfg.src != nil {
		// 高级 override:业务自带规则来源,不落 doorman 表(无 Console / 策略 / 流水)。
		app.ProvideAs(service.NewWithSource(reg, f.cfg.src, f.cfg.ttl), (*Doorman)(nil))
		return nil
	}

	// 默认:DB 存储。表不存在会在首次读写时报错——那是"接入方漏了复制 goose 迁移",应显式暴露,不被 AutoMigrate 悄悄兜住。
	var h dbHolder
	if err := app.Resolve(&h); err != nil {
		return err
	}
	d, con, pruner := service.NewWithStore(reg, h.DB, f.cfg.ttl)
	app.ProvideAs(d, (*Doorman)(nil))
	app.ProvideAs(con, (*Console)(nil)) // 配置页后端

	// 决策流水保留:起后台 goroutine 定期清老数据(库自包含,不依赖外部 job)。
	// 注意:doorman 若被多个服务(admin+customer)各自嵌入,会各起一个清理 goroutine 对同一张表清,
	// DELETE 幂等、无害,只是略有重复;如只想让一处清,给其中一处 WithDecisionRetention(0) 关掉即可。
	if f.cfg.retention > 0 {
		go f.runPrune(pruner)
	}
	return nil
}

// runPrune 后台定期清理早于保留期的决策流水,直到 Close 停止。best-effort:失败只记日志、继续。
func (f *doormanFeature) runPrune(pruner service.Pruner) {
	prune := func() {
		before := time.Now().Add(-f.cfg.retention)
		n, err := pruner(before, pruneBatchSize)
		if err != nil {
			logger.Errorf("doorman: 清理决策流水失败: %+v", err)
		} else if n > 0 {
			logger.Infof("doorman: 已清理 %d 条早于 %s 的决策流水", n, f.cfg.retention)
		}
	}

	// 启动后先等一会再首刷(避开启动高峰),之后按周期清。
	select {
	case <-time.After(pruneStartDelay):
		prune()
	case <-f.stop:
		return
	}
	t := time.NewTicker(prunePeriod)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			prune()
		case <-f.stop:
			return
		}
	}
}
