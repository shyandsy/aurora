package ratelimit

import (
	"fmt"

	"github.com/shyandsy/aurora/contracts"
	auroraFeature "github.com/shyandsy/aurora/feature"
)

// ── aurora Feature 装配 ──
//
// opt-in feature(不在 bootstrap 默认集):需要限流/防滥用的服务才装。用法:
//
//	// 桶声明为包变量(注册与调用点复用同一句柄):
//	var LoginIPFail = ratelimit.NewFailLockBucket("login_ip_fail", "ip")
//
//	app.ProvideAs(myLimitsProvider, (*ratelimit.LimitsProvider)(nil)) // 阈值来源(依赖,走 DI);静态用 ratelimit.StaticLimits(...)
//	app.AddFeature(ratelimit.NewRateLimitFeature("",                  // namespace 传空 → 自动取 SERVICE_NAME
//	    ratelimit.WithBucket(LoginIPFail),
//	))
//	// 服务里:  RL ratelimit.Service `inject:""`  → s.RL.Fail(ctx, LoginIPFail, dims)
//
// 三样按性质用不同机制(对齐 aurora 惯例):
//   - namespace(标量配置)= NewRateLimitFeature **位置参数**;**留空则默认取 SERVICE_NAME**(见下);
//   - LimitsProvider(依赖/服务)= **DI 注入**(和 redis 同等,同 loginguard 的 provider);
//   - buckets(0..N 个声明)     = 可重复 **Option** WithBucket(同 doorman 的 WithScope)。
//
// **namespace = 服务身份**:所有 key 前缀 `rate_limit:<namespace>:...`,把不同服务的计数由构造隔离
// (多服务共用一个 Redis DB 时,没有它 user 与 customer 的同名桶/API 会撞 key)。ratelimit 是公共组件,
// 分区键语义上就是「哪个服务」,故**留空自动取 SERVICE_NAME、按服务天然解耦**;要自定义 realm 才显式传。
// 空且 SERVICE_NAME 也未配 = Setup fail-startup。
type ratelimitFeature struct {
	Redis    auroraFeature.RedisService `inject:""`
	Provider LimitsProvider             `inject:""` // 阈值来源:依赖,和 redis 同等走 DI(consumer 先 ProvideAs)

	namespace string
	buckets   []Bucket
}

// Option 配置项(目前只有 WithBucket)。
type Option func(*ratelimitFeature)

// WithBucket 注册一个桶句柄(NewCountBucket / NewFailLockBucket / NewCooldownBucket 之一)。可多次调用。
// 注册的桶会在 Setup 校验其 LimitsProvider 已配阈值(否则 fail-startup),避免"声明了桶却没配阈值 → 静默不限流"。
func WithBucket(b Bucket) Option {
	return func(f *ratelimitFeature) { f.buckets = append(f.buckets, b) }
}

// NewRateLimitFeature 构造限流 feature。namespace 为 key 前缀(= 哪个服务),**留空则默认取 SERVICE_NAME**;
// buckets 经 WithBucket 声明;redis 与 LimitsProvider 走 DI(consumer 先 app.ProvideAs 一个 ratelimit.LimitsProvider)。
func NewRateLimitFeature(namespace string, opts ...Option) contracts.Features {
	f := &ratelimitFeature{namespace: namespace}
	for _, o := range opts {
		if o != nil {
			o(f)
		}
	}
	return f
}

func (f *ratelimitFeature) Name() string { return "ratelimit" }

// Setup 校验依赖/配置(错即 fail-startup;与运行时 fail-open 不矛盾),装好 Service 注入 DI。
func (f *ratelimitFeature) Setup(app contracts.App) error {
	if err := app.Resolve(f); err != nil {
		return fmt.Errorf("ratelimit: 解析依赖失败(redis / LimitsProvider): %w", err)
	}
	if f.Redis == nil {
		return fmt.Errorf("ratelimit: RedisService 未注入 —— 须在 ratelimit 之前注册 redis feature")
	}
	// namespace 留空 → 自动取 SERVICE_NAME,兜底集中在 newService(见 service.go),此处不再各自解析。
	if f.Provider == nil {
		return fmt.Errorf("ratelimit: LimitsProvider 未注入 —— 须先 app.ProvideAs 一个 ratelimit.LimitsProvider(静态用 StaticLimits(...))")
	}

	// 每个注册的桶都要在 LimitsProvider 里配上阈值(按种类查对应字段),否则是"声明了却没配 = 静默不限流"的坑。
	for _, b := range f.buckets {
		d := b.def()
		if d.name == "" {
			return fmt.Errorf("ratelimit: 桶缺 name")
		}
		lim := f.Provider.Limits(d.name)
		switch d.kind {
		case kindCount:
			if lim.Window <= 0 || lim.Limit <= 0 {
				return fmt.Errorf("ratelimit: 计数桶 %q 未配阈值(需 Window>0 且 Limit>0)", d.name)
			}
		case kindFailLock:
			if lim.Window <= 0 || lim.Limit <= 0 || lim.LockSeconds <= 0 {
				return fmt.Errorf("ratelimit: 失败锁桶 %q 未配阈值(需 Window/Limit/LockSeconds 均>0)", d.name)
			}
		case kindCooldown:
			if lim.Gap <= 0 {
				return fmt.Errorf("ratelimit: 冷却桶 %q 未配阈值(需 Gap>0)", d.name)
			}
		}
	}

	app.ProvideAs(newService(f.namespace, f.Provider, newLimiter(f.Redis)), (*Service)(nil))
	return nil
}

func (f *ratelimitFeature) Close() error { return nil }
