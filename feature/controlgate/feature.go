// Package controlgate 是 aurora 的「可信授权门禁」feature:**包住(wrap)sealkit/guard 的续约传输与
// crypto,不吞并它**——回连 control 做续期授权、按下发策略降级,并提供 SignViaControl 代签。
//
// 分工(与 control 面的 doctrine 一致):
//   - control       = 策略定义 + 项目私钥签名(随续约裁决下发)
//   - sealkit/guard  = 续约 HTTP 传输 + ed25519 公钥解码(中立可移植库:节点 / C 也在用)
//   - 本 feature     = 执行:验签裁决 → 按策略随机注入 500(errorRate)/ 过期宽限(allowExpiredUse+
//     maxGraceDays)/ 吊销立即停 + NTS 可信时钟 + 启动宽限 + 持久化恢复
//
// 依赖方向**单向**:controlgate → sealkit,永不反过来;绝不把 sealkit/guard 搬进 aurora。
//
// verdict 契约先自包含在本包;把契约下沉 sealkit(与 guard.RenewVerdict 合流)、信封装运营材料、
// principal+type 均为后续阶段。
//
// 配置模型:
//   - 坐标走 env(CONTROL_*,Setup 内 config.ResolveConfig 灌;env 名冻结、勿缩写/改名,见 config.go);
//   - ServiceName 默认 app.Name()、DebugLogs 按 RunLevel 自动(prd 收敛);其余经 NewControlgateFeature
//     的 options(WithMetricNamespace/WithServiceName/WithRequireGate/WithRegisterer)设;
//   - prd 的 garble/-literals/DCE 特征卫生是**消费方 build 时**的事,不进中立库;
//   - metrics namespace 可配(中立库不自造项目前缀,空 = 不启观测)。
//
// 安全语义**一位不丢**:fail-closed(超启动期时间门 / pending / 吊销 / 过期)、NTS 可信时钟
// (防墙钟回拨 + 认证时间门/air-gap)、启动期时间门(默认 2min 可配;配了 NTS 源时 never-seen/重启/删 state
// 都须先拿 fresh 认证时间才服务,堵"删 state 白嫖")、降级(exemptPath 之外按 faultRate 随机 500)。
//
// 注册约束:**须在 server feature 之后、且在 RegisterRoutes 之前**(见 controlgateFeature)。
package controlgate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/shyandsy/aurora/config"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/logger"
	"github.com/shyandsy/sealkit/guard"
)

// Gate 是门禁对外的注入契约:服务经 `inject:""` 拿到它查询当前降级概率(观测/自诊断用)。
// 中间件由 feature 在 Setup 里自动挂到 *gin.Engine,消费方无需手动 Use。
//
//	type someService struct {
//	    Gate controlgate.Gate `inject:""`
//	}
type Gate interface {
	// FaultRate 返回本次请求注入 500 的概率(0-100)。授权有效时为 0。
	FaultRate() int
	// ServingState 返回当前服务状态(serving/shedding/tripped/isolated),供健康/状态页读授权状态。
	ServingState() ServingState
	// AuthorizedUntil 返回当前授权到期 unix 秒;0 = 从未授权 / pending / 门禁关闭。
	AuthorizedUntil() int64
	// SignViaControl 把 payload 送 control 代签,返回 control 用其持有私钥产的签名。
	// key 永不下发、本包不持不缓存;走已有的 control 通道(RenewBaseURL + Project/Deployment 身份)。
	// 授权失败/被吊销/后端不可达 → 返回 error(fail-closed,绝不返回伪造/占位签名)。
	SignViaControl(ctx context.Context, payload []byte) (sig []byte, err error)
}

// disabledGate 门禁关闭(eng 未接入 control)时提供的空实现:恒放行、无到期。
// 保证消费方 `inject:""` Gate 在 eng 未配坐标时也能解析,不因缺 provider 崩启动。
// **状态返回 Disabled(不是 Serving)**:让状态页/告警能明确看见「闸关了」,不被伪装成健康。
type disabledGate struct{}

func (disabledGate) FaultRate() int             { return 0 }
func (disabledGate) ServingState() ServingState { return Disabled }
func (disabledGate) AuthorizedUntil() int64     { return 0 }
func (disabledGate) SignViaControl(context.Context, []byte) ([]byte, error) {
	return nil, fmt.Errorf("controlgate: 门禁关闭 / 未接入 control,无法代签(SignViaControl)")
}

// controlgateFeature 把可信授权门禁装进 aurora DI:注册后 ProvideAs 出 Gate + 自动挂 gin 中间件。
//
// opt-in feature(不在 bootstrap 默认集):只有 prd 要接 control 授权的服务显式 AddFeature。
// **须在 server feature 之后、且在 RegisterRoutes 之前注册**:要 Resolve 已 Provide 的 *gin.Engine,
// 且 gin 中间件只对之后注册的路由生效(Setup 里有 assertNoRoutes 自守卫,顺序反了直接 fail-startup)。
type controlgateFeature struct {
	cfg    Config
	cancel context.CancelFunc // 后台续约/观测循环的取消(Close 时触发)
}

// Option 配置 controlgate feature 的可选项(函数式 options,与 doorman/geoip 等 feature 一致)。
// control 坐标(CONTROL_*)**不走 option**:它们是部署配置,Setup 里统一经 config.ResolveConfig 从 env 灌。
type Option func(*Config)

// WithServiceName 覆盖门禁名(日志 / metrics 的 service label)。不设 → 默认取 app.Name()。
func WithServiceName(name string) Option { return func(c *Config) { c.ServiceName = name } }

// WithMetricNamespace 设 prometheus 指标 namespace(项目前缀,如 "myapp")。
// 不设 = 不启用观测(aurora 中立,不自造项目前缀)。
func WithMetricNamespace(ns string) Option { return func(c *Config) { c.MetricNamespace = ns } }

// WithRequireGate 逃生阀:任何 RunLevel 下坐标全空都 fail-startup(默认仅 production 全空才 fail)。
func WithRequireGate(require bool) Option { return func(c *Config) { c.RequireGate = require } }

// WithRegisterer 注入自定义 prometheus registerer(默认 DefaultRegisterer;测试可注入独立 registry)。
func WithRegisterer(r prometheus.Registerer) Option { return func(c *Config) { c.Registerer = r } }

// NewControlgateFeature 构造可信授权门禁 feature。坐标走 CONTROL_* env(Setup 内部 ResolveConfig);
// ServiceName 默认 app.Name()、DebugLogs 默认按 RunLevel —— 消费方通常只需传 WithMetricNamespace:
//
//	app.AddFeature(controlgate.NewControlgateFeature(
//	    controlgate.WithMetricNamespace("myapp"),
//	))
//
// 坐标全空 → 门禁关闭(eng 便利),部分缺失 → Setup fail-startup(暴露误配)。
func NewControlgateFeature(opts ...Option) contracts.Features {
	f := &controlgateFeature{}
	for _, opt := range opts {
		opt(&f.cfg)
	}
	return f
}

func (f *controlgateFeature) Name() string { return "controlgate" }

func (f *controlgateFeature) Setup(app contracts.App) error {
	cfg := f.cfg
	// 坐标走 env:CONTROL_* 经 config.ResolveConfig 灌(只填带 env 标签的字段,不覆盖 options 设的 ServiceName/
	// MetricNamespace/RequireGate/Registerer)。消费方装配处因此不必自己 ResolveConfig。
	_ = config.ResolveConfig(&cfg)
	// ServiceName 默认取服务名(app.Name()),DebugLogs 按 RunLevel 推 —— 都不劳消费方在装配处手设。
	if cfg.ServiceName == "" {
		cfg.ServiceName = app.Name()
	}
	// 覆盖与默认都空 → 门禁没名字(日志/metrics label 空),拒绝静默裸奔:fail-startup,暴露误配
	//(正常 app.Name() 必非空——aurora SERVICE_NAME 必填 + envDefault;到这说明 App 实现没给名)。
	if cfg.ServiceName == "" {
		return fmt.Errorf("controlgate: 无法确定门禁名 —— 未传 WithServiceName 且 app.Name() 为空(服务未配 SERVICE_NAME?)")
	}
	cfg.DebugLogs = app.RunLevel() == config.RunLevelEng
	name := cfg.ServiceName

	// 坐标全空的处置分 level:
	//   - prd(production)或 RequireGate=true → **fail-startup**:prd 坐标全空 = 误配(secret 挂坏 /
	//     漏 env),绝不能静默裸奔(disabledGate 恒放行、监控还看不出)——当场炸,暴露给部署方。
	//   - 其余(local/eng/stage)→ 门禁关闭(开发便利,不 brick),ProvideAs 一个 disabledGate
	//     (ServingState()=Disabled,状态页能看见闸关了),让消费方注入不失败。
	if cfg.coordsAllEmpty() {
		if cfg.failStartupOnEmptyCoords(app.RunLevel()) {
			return fmt.Errorf("controlgate(%s): 坐标全空但要求门禁开启(RunLevel=%q, RequireGate=%v)—— prd 坐标全空视为误配(CONTROL_* secret/env 挂坏),拒绝静默裸奔",
				name, app.RunLevel(), cfg.RequireGate)
		}
		if cfg.DebugLogs {
			logger.Debugf("controlgate(%s): 坐标全空,门禁关闭(RunLevel=%q,非 prd 开发便利)", name, app.RunLevel())
		}
		app.ProvideAs(&disabledGate{}, (*Gate)(nil))
		return nil
	}

	// 坐标部分缺失 = 明显误配 → fail-startup(显式暴露,绝不静默半开)。
	if !cfg.coordsComplete() {
		return fmt.Errorf("controlgate(%s): 坐标不全(需 CONTROL_RENEW_BASE_URL/PROJECT_UUID/DEPLOYMENT_ID/PROJECT_PUBKEY 齐全,或全空以关闭门禁)", name)
	}

	pub, err := guard.DecodePub(cfg.ProjectPubKey)
	if err != nil {
		return fmt.Errorf("controlgate(%s): CONTROL_PROJECT_PUBKEY 解析失败: %w", name, err)
	}

	// gin.Engine 由 server feature Provide;controlgate 须在其后注册。
	var eng struct {
		Engine *gin.Engine `inject:""`
	}
	if err := app.Resolve(&eng); err != nil || eng.Engine == nil {
		return fmt.Errorf("controlgate(%s): 无法解析 *gin.Engine(须在 server feature 之后注册): %w", name, err)
	}

	// 路由时序自守卫:中间件只对**之后**注册的路由生效。若此刻 engine 已有路由,说明它们在
	// controlgate 之前就注册了,会**静默绕过门禁** → fail-startup,逼调用方把 AddFeature(controlgate)
	// 提到 RegisterRoutes 之前。标准用法(AddFeature×N → RegisterRoutes → Run)本就满足,这是防误用护栏。
	if err := assertNoRoutes(eng.Engine, name); err != nil {
		return err
	}

	// 持久化路径(可选):StateDir 指向一个可写、最好跨重启保留(k8s 挂卷)的目录。
	// 未配 → 持久化关闭,退化为纯启动宽限(仍 fail-closed,只是重启不保留剩余授权)。
	statePath := ""
	if cfg.StateDir != "" {
		_ = os.MkdirAll(cfg.StateDir, 0o700) // best-effort
		statePath = filepath.Join(cfg.StateDir, name+".cache")
	}
	sealK := sealKey(pub) // 落盘加密密钥(从 pubkey 派生;混淆非保密,见 seal.go)

	// NTS 源:始终注册(是否启用 air-gap 由 Available()——control 是否下发过非空名单——决定)。
	nts := newNtsSource(name, cfg.DebugLogs)

	var metrics *gateMetrics
	if cfg.MetricNamespace != "" {
		metrics = newGateMetrics(cfg.MetricNamespace, cfg.Registerer)
	}

	g := &gate{
		name:         name,
		debug:        cfg.DebugLogs,
		startupGrace: cfg.startupGraceOrDefault(),
		clock:        newTrustedClock(),
		sources:      []timeSource{nts},
		nts:          nts,
		metrics:      metrics,
		exempt:       newExemptSet(cfg.ExemptPaths), // 默认基础设施端点 + 配置追加
		controlBase:  cfg.RenewBaseURL,              // SignViaControl 复用同一 control 认证通道坐标
		projectUUID:  cfg.ProjectUUID,
		deploymentID: cfg.DeploymentID,
	}

	// 启动时先尝试从本地恢复上次签名裁决:重启不退回 never-seen,避免"重启时正好 deploy 抖动"被误锁。
	if v, ok := loadPersisted(statePath, sealK, pub, cfg.DeploymentID); ok {
		g.applyLoaded(v)
	}

	ctx, cancel := context.WithCancel(context.Background())
	f.cancel = cancel
	go g.run(ctx, cfg.RenewBaseURL, cfg.ProjectUUID, cfg.DeploymentID, pub, cfg.renewEveryOrDefault(), statePath, sealK)
	go g.exportMetrics(ctx) // metrics 为空则立即返回

	app.ProvideAs(g, (*Gate)(nil))

	// 中间件:执行策略 —— 按 faultRate 随机注入 500。平常(授权有效)faultRate=0,零影响。
	eng.Engine.Use(g.middleware())

	if cfg.DebugLogs {
		logger.Infof("controlgate(%s): 门禁已启用 control=%s deployment=%s 续约=%s(等首次续约拿授权 + NTS 名单)",
			name, cfg.RenewBaseURL, cfg.DeploymentID, cfg.renewEveryOrDefault())
	}
	return nil
}

func (f *controlgateFeature) Close() error {
	if f.cancel != nil {
		f.cancel()
	}
	return nil
}

// assertNoRoutes 路由时序自守卫:engine 已注册路由时报错。gin 中间件(engine.Use)只对**之后**注册
// 的路由生效,故已存在的路由会静默绕过门禁 → fail-startup,逼调用方把 controlgate 提到 RegisterRoutes 之前。
func assertNoRoutes(engine *gin.Engine, name string) error {
	if n := len(engine.Routes()); n > 0 {
		return fmt.Errorf("controlgate(%s): 检测到 engine 已注册 %d 条路由 —— controlgate 须在 RegisterRoutes 之前注册,否则这些路由会绕过门禁(把 AddFeature(controlgate) 提到路由注册之前)", name, n)
	}
	return nil
}
