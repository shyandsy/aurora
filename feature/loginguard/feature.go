package loginguard

import (
	"fmt"

	"github.com/shyandsy/aurora/config"
	"github.com/shyandsy/aurora/contracts"
	auroraFeature "github.com/shyandsy/aurora/feature"
	"github.com/shyandsy/aurora/feature/ratelimit"
)

// loginGuardFeature 把 Guard 装进 aurora DI。redis 与策略来源(LoginPolicyProvider)**都经 DI 注入**
// (provider 是服务/依赖,和 redis 同等对待,不当构造参数手塞)。注册后登录服务经 `inject:""` 拿 Guard。
//
// 计数机制不自己实现:内部持有一个 ratelimit 引擎(NewEngine),登录桶声明在其上,阈值由 LoginPolicy
// 适配成引擎的 LimitsProvider。namespace 给引擎做 key 前缀 rate_limit:<ns>:...,多服务共用一个 Redis DB
// 也不撞键(尤其账号维度:两服务同名账号是不同的人)。**namespace = 哪个服务,留空默认取 SERVICE_NAME**。
//
// opt-in feature(不在 bootstrap 默认集):只有**处理登录**的服务需要它。app 侧两步:
//
//	app.ProvideAs(myProvider, (*loginguard.LoginPolicyProvider)(nil)) // 静态用 loginguard.StaticPolicy(...)
//	app.AddFeature(loginguard.NewLoginGuardFeature(""))               // 空 → 自动 SERVICE_NAME;须在 redis + provider 之后
type loginGuardFeature struct {
	Redis    auroraFeature.RedisService `inject:""`
	Provider LoginPolicyProvider        `inject:""`

	namespace string
}

// NewLoginGuardFeature 构造登录暴力破解防护 feature。namespace 为底层 ratelimit 引擎的 key 前缀(= 哪个服务),
// **留空则默认取 SERVICE_NAME**;redis + LoginPolicyProvider 走 DI(app 先把 provider ProvideAs 进容器)。
func NewLoginGuardFeature(namespace string) contracts.Features {
	return &loginGuardFeature{namespace: namespace}
}

func (f *loginGuardFeature) Name() string { return "loginguard" }

func (f *loginGuardFeature) Setup(app contracts.App) error {
	// 配置/依赖错误一律 fail-startup(启动即暴露);注意这与**运行时** fail-open 不冲突:
	// 显式注册了 loginguard 却没配 redis / 没 ProvideAs provider,是装配 bug,该当场炸;运行时 redis 抖动才 fail-open。
	if err := app.Resolve(f); err != nil {
		return fmt.Errorf("loginguard: 解析依赖失败(redis / LoginPolicyProvider): %w", err)
	}
	if f.Redis == nil {
		return fmt.Errorf("loginguard: RedisService 未注入 —— 须在 loginguard 之前注册 redis feature")
	}
	if f.namespace == "" {
		var sc config.ServerConfig
		_ = config.ResolveConfig(&sc)
		f.namespace = sc.Name // 空 → 自动取 SERVICE_NAME(按服务解耦,零手传)
	}
	if f.namespace == "" {
		return fmt.Errorf("loginguard: namespace 为空且 SERVICE_NAME 未配 —— 无法确定 key 前缀(空前缀会让多服务共库时 key 相撞)")
	}
	if f.Provider == nil {
		return fmt.Errorf("loginguard: LoginPolicyProvider 未注入 —— 须先 app.ProvideAs 一个(静态用 StaticPolicy(...))")
	}
	if p := f.Provider.LoginPolicy(); !p.Valid() {
		return fmt.Errorf("loginguard: 初始 LoginPolicy 不自洽(开了限额/锁却缺窗口或时长): %+v", p)
	}

	// 建在 ratelimit 引擎上:登录桶的阈值由 LoginPolicy 适配成引擎的 LimitsProvider(见 guard.go)。
	engine := ratelimit.NewEngine(f.Redis, f.namespace, policyLimits{p: f.Provider})
	app.ProvideAs(newGuard(engine, f.Provider), (*Guard)(nil))
	return nil
}

func (f *loginGuardFeature) Close() error { return nil }
