package loginguard

import (
	"fmt"

	"github.com/shyandsy/aurora/contracts"
	auroraFeature "github.com/shyandsy/aurora/feature"
)

// loginGuardFeature 把 Guard 装进 aurora DI。redis 与策略来源(LoginPolicyProvider)**都经 DI 注入**
// (provider 是服务/依赖,和 redis 同等对待,不当构造参数手塞)。注册后登录服务经 `inject:""` 拿 Guard。
//
// opt-in feature(不在 bootstrap 默认集):只有**处理登录**的服务需要它。app 侧两步:
//
//	app.ProvideAs(myProvider, (*loginguard.LoginPolicyProvider)(nil)) // 静态用 loginguard.StaticPolicy(...)
//	app.AddFeature(loginguard.NewLoginGuardFeature())                 // 须在 redis + 上面的 provider 之后
type loginGuardFeature struct {
	Redis    auroraFeature.RedisService `inject:""`
	Provider LoginPolicyProvider        `inject:""`
}

// NewLoginGuardFeature 构造登录暴力破解防护 feature。依赖(redis + LoginPolicyProvider)全走 DI,
// 故不收任何参数 —— app 只需先把自己的 LoginPolicyProvider ProvideAs 进容器(见类型注释)。
func NewLoginGuardFeature() contracts.Features {
	return &loginGuardFeature{}
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
	if f.Provider == nil {
		return fmt.Errorf("loginguard: LoginPolicyProvider 未注入 —— 须先 app.ProvideAs 一个(静态用 StaticPolicy(...))")
	}
	if p := f.Provider.LoginPolicy(); !p.Valid() {
		return fmt.Errorf("loginguard: 初始 LoginPolicy 不自洽(开了限额/锁却缺窗口或时长): %+v", p)
	}

	app.ProvideAs(newGuard(f.Redis, f.Provider), (*Guard)(nil))
	return nil
}

func (f *loginGuardFeature) Close() error { return nil }
