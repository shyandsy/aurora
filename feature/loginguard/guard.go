// Package loginguard 是 aurora 防护三件套里的「登录前」:登录暴力破解防护 —— 按 IP + 按账号的
// 失败计数、短期锁定、成功清理。它是**登录领域的外壳**:懂登录语义(账号 vs IP、待 2FA、成功/失败),
// 对外只暴露 Guard;**计数机制不自己实现,建在 ratelimit 引擎上**(全体系只有一份计数地基)。
//
// 与「登录后」的 tokenguard 相反,loginguard 全程 **fail-open**:Redis 不可用时预检一律放行、记录一律
// no-op —— 登录限流是**次级**防护,绝不能因 Redis 抖动把所有人锁在登录之外(可用性优先)。而 tokenguard
// 的撤销/IP 是**主级**安全控制,故 fail-close。两者哲学相反,都是刻意的。(fail-open 由 ratelimit 引擎兜。)
//
// # 一套代码,三种产品形态(靠 config,不靠分支)
//
//   - deploy user / homeserver user:静态默认(StaticPolicy),阈值编译期定;
//   - homeserver customer:运行时可调(provider 包住其 SystemSettingService,内存缓存不碰 DB 热路径);
//   - 账号锁 vs 只计数:由 AcctLockSeconds 编码(见 LoginPolicy),不设额外开关(=0 → 账号桶只计数不上锁);
//   - 账号标识明文 or 哈希:由**调用方**传入的 account 决定(deploy 传明文便于运维解锁;
//     homeserver 传 email 哈希不落客户明文)。loginguard 不掺和。
package loginguard

import (
	"context"
	"time"

	"github.com/shyandsy/aurora/feature/ratelimit"
)

// (阈值策略 LoginPolicy / LoginPolicyProvider / StaticPolicy 见 policy.go。)

// 登录用的三个桶(声明在 ratelimit 引擎上,形状由句柄承载):IP 失败锁、IP 每小时成功上限、账号失败锁。
// 维度名即 key 维度;账号桶是否真上锁由阈值(AcctLockSeconds)决定,不体现在桶形状里。
var (
	bucketIPFail   = ratelimit.NewFailLockBucket("login_ip_fail", "ip")
	bucketIPHour   = ratelimit.NewCountBucket("login_ip_hour", "ip")
	bucketAcctFail = ratelimit.NewFailLockBucket("login_acct_fail", "account")
)

// Decision 预检结论。Blocked 决定是否放行;Remaining* 仅供调用方设「还剩几次」提示头,不影响放行。
type Decision struct {
	Blocked bool
	Reason  string // ip_locked / ip_hour_cap / acct_locked
	// 还能失败/登录几次(不为负;对应维度关闭时为 0)。供 X-Login-*-Remaining 之类的提示。
	IPFailRemaining   int
	IPHourRemaining   int
	AcctFailRemaining int
	// RetryAfter 被锁/超限时建议等待的秒数(供 Retry-After 头);未拦时为 0。
	RetryAfter int64
}

// Guard 登录暴力破解防护(DI 注入用接口)。IP 维度在 handler 前(中间件,仅有 IP)预检;账号维度在拿到
// 账号后(服务层或中间件)预检。成功/失败由登录流程在**权威判定点**调用记录(不靠 HTTP 状态码猜)。
type Guard interface {
	PrecheckIP(ctx context.Context, ip string) Decision
	PrecheckAccount(ctx context.Context, account string) Decision

	// 记录侧只说**登录结局**(不说"清/加哪个计数"——那是本包的领域知识,藏在内部):
	//   - RecordFailure 凭据失败:计失败,达阈值按策略锁 IP/账号;
	//   - RecordSuccess 完成登录:清失败 + 计入每小时成功数;
	//   - RecordPending 密码已验对但登录**未完成**(如待 2FA):只清**账号**失败(保留 IP),不计成功。
	RecordFailure(ctx context.Context, ip, account string)
	RecordSuccess(ctx context.Context, ip, account string)
	RecordPending(ctx context.Context, ip, account string)

	// Unlock 运维强制解锁:清掉给定 IP 与账号的失败计数 + 锁(供后台"被锁列表"的解锁按钮)。
	// 传空串则跳过该维度。与 RecordSuccess 不同:Unlock 连锁 key 一起清(RecordSuccess 只清失败计数)。
	Unlock(ctx context.Context, ip, account string)
}

// guard 持有一个 ratelimit 引擎(计数地基)+ 策略来源(读阈值/模式标志)。计数、key、TTL、原子性全由引擎负责。
type guard struct {
	engine   ratelimit.Service
	provider LoginPolicyProvider
}

func newGuard(engine ratelimit.Service, provider LoginPolicyProvider) *guard {
	return &guard{engine: engine, provider: provider}
}

func (g *guard) policy() LoginPolicy { return g.provider.LoginPolicy() }

// policyLimits 把 LoginPolicy 适配成 ratelimit.LimitsProvider:引擎按桶名问阈值时,现算自当前策略
// (策略可运行时可调;适配器无状态,读的是 provider 的当前值)。窗口对 IP/账号失败桶复用 IPWindowSeconds。
type policyLimits struct{ p LoginPolicyProvider }

func (pl policyLimits) Limits(bucket string) ratelimit.Limits {
	pol := pl.p.LoginPolicy()
	win := time.Duration(pol.IPWindowSeconds) * time.Second
	switch bucket {
	case "login_ip_fail":
		return ratelimit.Limits{Window: win, Limit: pol.IPFailLimit, LockSeconds: pol.IPLockSeconds}
	case "login_ip_hour":
		return ratelimit.Limits{Window: time.Hour, Limit: pol.IPPerHour}
	case "login_acct_fail":
		// AcctLockSeconds=0 → LockSeconds=0 → 引擎只计数不上锁(公网 count-only,防 DoS 锁他人)。
		return ratelimit.Limits{Window: win, Limit: pol.AcctFailLimit, LockSeconds: pol.AcctLockSeconds}
	}
	return ratelimit.Limits{}
}

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
	if g == nil || g.engine == nil || ip == "" { // fail-open
		return Decision{}
	}
	p := g.policy()
	dims := map[string]string{"ip": ip}
	if p.IPFailLimit > 0 {
		if locked, ra := g.engine.Locked(ctx, bucketIPFail, dims); locked {
			return Decision{Blocked: true, Reason: "ip_locked", RetryAfter: ra}
		}
	}
	hour := g.engine.Peek(ctx, bucketIPHour, dims)
	if p.IPPerHour > 0 && hour.Count >= int64(p.IPPerHour) {
		return Decision{Blocked: true, Reason: "ip_hour_cap", RetryAfter: hour.RetryAfter}
	}
	return Decision{
		IPFailRemaining: remaining(int64(p.IPFailLimit), g.engine.PeekFail(ctx, bucketIPFail, dims).Count),
		IPHourRemaining: remaining(int64(p.IPPerHour), hour.Count),
	}
}

func (g *guard) PrecheckAccount(ctx context.Context, account string) Decision {
	if g == nil || g.engine == nil || account == "" { // fail-open
		return Decision{}
	}
	p := g.policy()
	if p.AcctFailLimit <= 0 { // 账号维度关闭
		return Decision{}
	}
	dims := map[string]string{"account": account}
	if p.AcctLockSeconds > 0 { // 仅硬锁模式才有锁可查
		if locked, ra := g.engine.Locked(ctx, bucketAcctFail, dims); locked {
			return Decision{Blocked: true, Reason: "acct_locked", RetryAfter: ra}
		}
	}
	return Decision{AcctFailRemaining: remaining(int64(p.AcctFailLimit), g.engine.PeekFail(ctx, bucketAcctFail, dims).Count)}
}

func (g *guard) RecordFailure(ctx context.Context, ip, account string) {
	if g == nil || g.engine == nil { // fail-open
		return
	}
	p := g.policy()
	if ip != "" && p.IPFailLimit > 0 {
		// 超阈值且 IPLockSeconds>0 时,引擎自动上 IP 锁。
		g.engine.Fail(ctx, bucketIPFail, map[string]string{"ip": ip})
	}
	if account != "" && p.AcctFailLimit > 0 {
		// 账号维度:始终计数(供提示/监控);AcctLockSeconds=0 时引擎只计数不锁(count-only)。
		g.engine.Fail(ctx, bucketAcctFail, map[string]string{"account": account})
	}
}

// RecordPending 密码已验对但登录**未完成**(如待 2FA):只清**账号**失败计数(该账号密码已被证明对,
// 其爆破计数停),但**保留 IP 失败计数**、也**不计入每小时成功数**(登录尚未真正完成)。
//
// 为什么不清 IP:IP 计数是跨账号聚合的,某账号密码对**不能**证明同 IP 对其它账号的猜测是善意的。
// 若在此清 IP,持有任一有效密码者即可反复触发 pending 把 IP 失败计数归零(且 pending 不计每小时成功、
// 不受 IPPerHour 约束 = 一个**不计量的重置原语**),从而绕过 IP 锁去喷射爆破其它账号——尤其在账号维度
// 只计数不硬锁(AcctLockSeconds=0)的公网形态下,IP 锁是唯一拦截,清 IP 等于拆掉它。
func (g *guard) RecordPending(ctx context.Context, ip, account string) {
	if g == nil || g.engine == nil || account == "" { // fail-open;IP 不参与,故不看 ip
		return
	}
	g.engine.ClearFail(ctx, bucketAcctFail, map[string]string{"account": account})
}

func (g *guard) RecordSuccess(ctx context.Context, ip, account string) {
	if g == nil || g.engine == nil {
		return
	}
	if ip != "" {
		g.engine.ClearFail(ctx, bucketIPFail, map[string]string{"ip": ip})
	}
	if account != "" {
		g.engine.ClearFail(ctx, bucketAcctFail, map[string]string{"account": account})
	}
	if ip != "" && g.policy().IPPerHour > 0 {
		g.engine.Hit(ctx, bucketIPHour, map[string]string{"ip": ip}) // 计一次每小时成功
	}
}

// Unlock 运维强制解锁:清 IP 与账号的失败计数 + 锁(Unlock 连锁一起清,区别于 RecordSuccess 只清失败计数)。
// 传空串跳过该维度。每小时成功计数(IPPerHour)不动——那是滑动配额、不是"锁",不该被解锁抹掉。
func (g *guard) Unlock(ctx context.Context, ip, account string) {
	if g == nil || g.engine == nil {
		return
	}
	if ip != "" {
		g.engine.Unlock(ctx, bucketIPFail, map[string]string{"ip": ip})
	}
	if account != "" {
		g.engine.Unlock(ctx, bucketAcctFail, map[string]string{"account": account})
	}
}
