package ratelimit

import (
	"context"

	auroraFeature "github.com/shyandsy/aurora/feature"
)

// Service 是业务用的接口:方法收**类型化桶句柄**(CountBucket / FailLockBucket / CooldownBucket)+ 维度值。
// 句柄自带形状,故:①桶名不可能 typo(句柄是声明好的变量)②调错方法(如对 CountBucket 调 Fail)= 编译错。
// 内部据句柄的形状 + LimitsProvider(阈值)组 namespaced key、调内部计数引擎。经 DI 注入使用:
//
//	var loginIPFail = ratelimit.NewFailLockBucket("login_ip_fail", "ip")   // 声明一次(包变量)
//	type someService struct { RL ratelimit.Service `inject:""` }
//	s.RL.Fail(ctx, loginIPFail, map[string]string{"ip": ip})               // 失败锁桶
//	if locked, _ := s.RL.Locked(ctx, loginIPFail, dims); locked { ... }
type Service interface {
	// Hit 计数桶:计一次并返回结果(Over 表示已超限)。
	Hit(ctx context.Context, b CountBucket, dims map[string]string) Outcome
	// Peek 计数桶:只读不计(预检 / 取 remaining)。
	Peek(ctx context.Context, b CountBucket, dims map[string]string) Outcome
	// Fail 失败锁桶:计一次失败,超阈值则上锁。
	Fail(ctx context.Context, b FailLockBucket, dims map[string]string) Outcome
	// Locked 失败锁桶:查是否被锁 + 还要等多久。
	Locked(ctx context.Context, b FailLockBucket, dims map[string]string) (bool, int64)
	// PeekFail 失败锁桶:只读当前失败计数(预检取 remaining,不记一次失败)。
	PeekFail(ctx context.Context, b FailLockBucket, dims map[string]string) Outcome
	// ClearFail 失败锁桶:清失败计数(如登录成功)。
	ClearFail(ctx context.Context, b FailLockBucket, dims map[string]string)
	// Cooldown 冷却桶:gap 内重复 → ok=false。
	Cooldown(ctx context.Context, b CooldownBucket, dims map[string]string) (ok bool, retryAfter int64)
}

const (
	failSuffix = ":fail"
	lockSuffix = ":lock"
)

type service struct {
	ns       string // feature 级 namespace
	provider LimitsProvider
	lim      *limiter
}

func newService(ns string, provider LimitsProvider, lim *limiter) *service {
	return &service{ns: ns, provider: provider, lim: lim}
}

// NewEngine 直接构造一个限流引擎(Service),供**兄弟 feature 在自己包内复用这份计数地基**
// (如 loginguard:声明自己的登录桶 + 用 LoginPolicy 适配成 LimitsProvider,内部持有一个引擎)。
// 不经 app/DI 装配 —— app 级用法仍走 NewRateLimitFeature(那条会在 Setup 校验每个桶已配阈值)。
//
// namespace 同样必填、禁空:所有 key 前缀 rate_limit:<namespace>:...,多服务共用一个 Redis DB 也不撞键。
// 走此路径时"桶是否配了阈值"由调用方(provider)自负,引擎不再代为 fail-startup 校验。
func NewEngine(redis auroraFeature.RedisService, namespace string, provider LimitsProvider) Service {
	return newService(namespace, provider, newLimiter(redis))
}

// 句柄自带形状,直接用;不再靠字符串查 registry、不存在"未命中"→ 没有 fail-open-on-编程错。
// (声明层错在编译期消灭;"声明了却没配阈值"在 Setup fail-startup;运行时只对 Redis 抖动 fail-open。)

func (s *service) Hit(ctx context.Context, b CountBucket, dims map[string]string) Outcome {
	d := b.d
	lim := s.provider.Limits(d.name)
	return s.lim.Hit(ctx, keyBase(s.ns, d, dims), lim.Window, lim.Limit)
}

func (s *service) Peek(ctx context.Context, b CountBucket, dims map[string]string) Outcome {
	d := b.d
	lim := s.provider.Limits(d.name)
	return s.lim.Peek(ctx, keyBase(s.ns, d, dims), lim.Window, lim.Limit)
}

func (s *service) Fail(ctx context.Context, b FailLockBucket, dims map[string]string) Outcome {
	d := b.d
	lim := s.provider.Limits(d.name)
	base := keyBase(s.ns, d, dims)
	return s.lim.Fail(ctx, base+failSuffix, base+lockSuffix, lim.Window, lim.Limit, lim.LockSeconds)
}

func (s *service) Locked(ctx context.Context, b FailLockBucket, dims map[string]string) (bool, int64) {
	return s.lim.LockState(ctx, keyBase(s.ns, b.d, dims)+lockSuffix)
}

func (s *service) PeekFail(ctx context.Context, b FailLockBucket, dims map[string]string) Outcome {
	d := b.d
	lim := s.provider.Limits(d.name)
	return s.lim.Peek(ctx, keyBase(s.ns, d, dims)+failSuffix, lim.Window, lim.Limit)
}

func (s *service) ClearFail(ctx context.Context, b FailLockBucket, dims map[string]string) {
	s.lim.Clear(ctx, keyBase(s.ns, b.d, dims)+failSuffix)
}

func (s *service) Cooldown(ctx context.Context, b CooldownBucket, dims map[string]string) (bool, int64) {
	d := b.d
	lim := s.provider.Limits(d.name)
	return s.lim.Cooldown(ctx, keyBase(s.ns, d, dims), lim.Gap)
}
