// Package loginguard 是 aurora 防护三件套里的「登录前」:登录暴力破解防护 —— 按 IP + 按账号的
// 失败计数、短期锁定、成功清理。是限流/锁定策略与 Redis key 的单一真源。
//
// 与「登录后」的 tokenguard 相反,loginguard 全程 **fail-open**:Redis 不可用时预检一律放行、记录一律
// no-op —— 登录限流是**次级**防护,绝不能因 Redis 抖动把所有人锁在登录之外(可用性优先)。而 tokenguard
// 的撤销/IP 是**主级**安全控制,故 fail-close。两者哲学相反,都是刻意的。
//
// 对外只暴露 Guard 接口 + LoginPolicy/Provider + NewLoginGuardFeature;key 格式、计数原语全不导出。
//
// # 一套代码,三种产品形态(靠 config,不靠分支)
//
//   - deploy user / homeserver user:静态默认(StaticPolicy),阈值编译期定;
//   - homeserver customer:运行时可调(provider 包住其 SystemSettingService,内存缓存不碰 DB 热路径);
//   - 账号锁 vs 只计数:由 AcctLockSeconds 编码(见 LoginPolicy),不设额外开关;
//   - 账号标识明文 or 哈希:由**调用方**传入的 account 决定(deploy 传明文便于 redis 手动解锁;
//     homeserver 传 email 哈希不落客户明文)。loginguard 不掺和。
package loginguard

import (
	"context"
	"strconv"
	"time"

	"github.com/shyandsy/aurora/logger"
)

// (阈值策略 LoginPolicy / LoginPolicyProvider / StaticPolicy 见 policy.go。)

// Decision 预检结论。Blocked 决定是否放行;Remaining* 仅供调用方设「还剩几次」提示头,不影响放行。
type Decision struct {
	Blocked bool
	Reason  string // ip_locked / ip_hour_cap / acct_locked
	// 还能失败/登录几次(不为负;对应维度关闭时为 0)。供 X-Login-*-Remaining 之类的提示。
	IPFailRemaining   int
	IPHourRemaining   int
	AcctFailRemaining int
}

// Guard 登录暴力破解防护(DI 注入用接口)。IP 维度在 handler 前(中间件,仅有 IP)预检;账号维度在拿到
// 账号后(服务层或中间件)预检。成功/失败由登录流程在**权威判定点**调用记录(不靠 HTTP 状态码猜)。
type Guard interface {
	PrecheckIP(ctx context.Context, ip string) Decision
	PrecheckAccount(ctx context.Context, account string) Decision

	// 记录侧只说**登录结局**(不说"清/加哪个计数"——那是本包的领域知识,藏在内部):
	//   - RecordFailure 凭据失败:计失败,达阈值按策略锁 IP/账号;
	//   - RecordSuccess 完成登录:清失败 + 计入每小时成功数;
	//   - RecordPending 密码已验对但登录**未完成**(如待 2FA):清失败,但**不计成功**。
	RecordFailure(ctx context.Context, ip, account string)
	RecordSuccess(ctx context.Context, ip, account string)
	RecordPending(ctx context.Context, ip, account string)
}

// redisOps 是 loginguard 所需的最小 Redis 能力(aurora RedisService 天然满足;单测用内存假实现)。
type redisOps interface {
	Get(ctx context.Context, key string) (string, error)
	Incr(ctx context.Context, key string) (int64, error)
	Expire(ctx context.Context, key string, ttl time.Duration) error
	Exists(ctx context.Context, key string) (bool, error)
	Delete(ctx context.Context, keys ...string) (int64, error)
	SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error)
}

type guard struct {
	redis    redisOps
	provider LoginPolicyProvider
}

func newGuard(redis redisOps, provider LoginPolicyProvider) *guard {
	return &guard{redis: redis, provider: provider}
}

func (g *guard) policy() LoginPolicy { return g.provider.LoginPolicy() }

// ---- key 收口(不导出)----
func ipFailKey(ip string) string  { return "rate_limit:login:ip:" + ip + ":fail" }
func ipLockKey(ip string) string  { return "rate_limit:login:ip:" + ip + ":lock" }
func ipHourKey(ip string) string  { return "rate_limit:login:ip:" + ip + ":hour" }
func acctFailKey(a string) string { return "rate_limit:login:acct:" + a + ":fail" }
func acctLockKey(a string) string { return "rate_limit:login:acct:" + a + ":lock" }

func remaining(limit, used int64) int {
	if limit <= 0 {
		return 0
	}
	if r := limit - used; r > 0 {
		return int(r)
	}
	return 0
}

func (g *guard) PrecheckIP(ctx context.Context, ip string) Decision {
	if g == nil || g.redis == nil || ip == "" { // fail-open
		return Decision{}
	}
	p := g.policy()
	if p.IPFailLimit > 0 {
		if locked, err := g.redis.Exists(ctx, ipLockKey(ip)); err == nil && locked {
			return Decision{Blocked: true, Reason: "ip_locked"}
		}
	}
	hour := g.count(ctx, ipHourKey(ip))
	if p.IPPerHour > 0 && hour >= int64(p.IPPerHour) {
		return Decision{Blocked: true, Reason: "ip_hour_cap"}
	}
	return Decision{
		IPFailRemaining: remaining(int64(p.IPFailLimit), g.count(ctx, ipFailKey(ip))),
		IPHourRemaining: remaining(int64(p.IPPerHour), hour),
	}
}

func (g *guard) PrecheckAccount(ctx context.Context, account string) Decision {
	if g == nil || g.redis == nil || account == "" { // fail-open
		return Decision{}
	}
	p := g.policy()
	if p.AcctFailLimit <= 0 { // 账号维度关闭
		return Decision{}
	}
	if p.AcctLockSeconds > 0 { // 仅硬锁模式才有锁可查
		if locked, err := g.redis.Exists(ctx, acctLockKey(account)); err == nil && locked {
			return Decision{Blocked: true, Reason: "acct_locked"}
		}
	}
	return Decision{AcctFailRemaining: remaining(int64(p.AcctFailLimit), g.count(ctx, acctFailKey(account)))}
}

func (g *guard) RecordFailure(ctx context.Context, ip, account string) {
	if g == nil || g.redis == nil { // fail-open
		return
	}
	p := g.policy()
	window := time.Duration(p.IPWindowSeconds) * time.Second
	if ip != "" && p.IPFailLimit > 0 {
		if c := g.incr(ctx, ipFailKey(ip), window); c > int64(p.IPFailLimit) && p.IPLockSeconds > 0 {
			_, _ = g.redis.SetNX(ctx, ipLockKey(ip), "1", time.Duration(p.IPLockSeconds)*time.Second)
		}
	}
	if account != "" && p.AcctFailLimit > 0 {
		// 账号维度:始终计数(供提示/监控);仅 AcctLockSeconds>0 才上锁(否则 count-only,防 DoS 锁他人)。
		if c := g.incr(ctx, acctFailKey(account), window); c > int64(p.AcctFailLimit) && p.AcctLockSeconds > 0 {
			_, _ = g.redis.SetNX(ctx, acctLockKey(account), "1", time.Duration(p.AcctLockSeconds)*time.Second)
		}
	}
}

// RecordPending 密码已验对但登录**未完成**(如待 2FA):清失败计数(密码已被证明对,爆破计数停),
// 但**不计入每小时成功数**(登录尚未真正完成)。
func (g *guard) RecordPending(ctx context.Context, ip, account string) {
	g.clearFailures(ctx, ip, account)
}

func (g *guard) RecordSuccess(ctx context.Context, ip, account string) {
	if g == nil || g.redis == nil {
		return
	}
	g.clearFailures(ctx, ip, account)
	if ip != "" && g.policy().IPPerHour > 0 {
		g.incr(ctx, ipHourKey(ip), time.Hour)
	}
}

// clearFailures 清 IP/账号失败计数(RecordSuccess / RecordPending 共用的内部机制)。
func (g *guard) clearFailures(ctx context.Context, ip, account string) {
	if g == nil || g.redis == nil {
		return
	}
	if ip != "" {
		_, _ = g.redis.Delete(ctx, ipFailKey(ip))
	}
	if account != "" {
		_, _ = g.redis.Delete(ctx, acctFailKey(account))
	}
}

// incr 自增并在首次创建时设 TTL。fail-open:出错记日志、返回 0(不触发锁)。
func (g *guard) incr(ctx context.Context, key string, ttl time.Duration) int64 {
	c, err := g.redis.Incr(ctx, key)
	if err != nil {
		logger.Errorf("loginguard: incr %s 失败(fail-open): %v", key, err)
		return 0
	}
	if c == 1 && ttl > 0 {
		_ = g.redis.Expire(ctx, key, ttl)
	}
	return c
}

func (g *guard) count(ctx context.Context, key string) int64 {
	v, err := g.redis.Get(ctx, key)
	if err != nil || v == "" {
		return 0
	}
	n, _ := strconv.ParseInt(v, 10, 64)
	return n
}
