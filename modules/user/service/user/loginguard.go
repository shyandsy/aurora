package user

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"

	serviceRateLimit "github.com/shyandsy/aurora/modules/user/service/ratelimit"
)

// 给客户端的「还剩几次」预警头(与原 LoginRateLimitMiddleware 同名,保持客户端契约):
// 客户端可在快锁前提示"还剩 N 次",不必盲目重试。仅提示、不影响放行。
const (
	headerLoginFailRemaining     = "X-Login-Fail-Remaining"         // 距 IP 被锁还能失败几次
	headerLoginHourlyRemaining   = "X-Login-Hourly-Remaining"       // 本小时还能成功登录几次
	headerLoginAcctFailRemaining = "X-Login-Account-Fail-Remaining" // 按账号距硬锁还剩几次失败
)

// loginguard.go —— 登录暴力破解防护(aurora feature/loginguard,防护三件套之「登录前」)的**接入点**。
//
// 语义分工(权威判定点在 service 层,不靠 HTTP 码猜):
//   - precheckLogin  —— 验密码**之前**:先按 IP(硬锁/每小时上限)、再按账号(硬锁模式)预检,命中即 429;
//   - LoginGuard.RecordFailure —— 凭据失败(账号不存在 / 密码错 / TOTP 错)时调用;
//   - LoginGuard.RecordPending —— 密码已验对但登录未完成(待 2FA / 强制绑定 2FA)时调用:清失败、不计成功;
//   - LoginGuard.RecordSuccess —— 真正签发正式 token(登录完成)时调用:清失败 + 计每小时成功数。
//
// 本服务(内部用户中心)用**硬锁**模式,account 传**明文 email**(便于运维去 Redis 手动解锁)。
// 运行时 fail-open 由 guard 内部处理(Redis 抖动时预检放行、记录 no-op),此处只管在正确的点调用。

// precheckLogin 登录前预检:IP 维度(锁 / 每小时成功上限)+ 账号维度(硬锁)。命中返回 429。
// account 为明文 email;为空时只走 IP 维度。顺带把「还剩几次」预警头写上(即便被拦也给,供客户端提示)。
func (s *userService) precheckLogin(ctx *contracts.RequestContext, account string) bizerr.BizError {
	dIP := s.LoginGuard.PrecheckIP(ctx.Context, ctx.ClientIP())
	ctx.Header(headerLoginFailRemaining, strconv.Itoa(dIP.IPFailRemaining))
	ctx.Header(headerLoginHourlyRemaining, strconv.Itoa(dIP.IPHourRemaining))
	if dIP.Blocked {
		return loginRateLimited(ctx)
	}
	if account != "" {
		dAcct := s.LoginGuard.PrecheckAccount(ctx.Context, account)
		ctx.Header(headerLoginAcctFailRemaining, strconv.Itoa(dAcct.AcctFailRemaining))
		if dAcct.Blocked {
			return loginRateLimited(ctx)
		}
	}
	return nil
}

// loginRateLimited 被暴力破解防护拦下的统一出口(HTTP 429,机器可读的「稍后再试」)。
func loginRateLimited(ctx *contracts.RequestContext) bizerr.BizError {
	return bizerr.New(http.StatusTooManyRequests, errors.New(ctx.T("auth.login_rate_limited")))
}

// ---- 记录侧薄封装 ----
// 收口「IP 从哪取、account 用明文还是哈希」:调用点只给业务语义(哪个账号、哪种结局),
// 不重复 ctx.Context / ctx.ClientIP()。将来若改 IP 来源或对 account 做哈希,只改这三处。

// recordLoginFailure 记一次凭据失败(账号不存在 / 密码错 / TOTP 错)。
// account 传明文 email;**传空串 = 只计 IP 维度**(用于 2FA 失败:账号维度的 2FA 锁由 twofaRecordFail 独占,避免双计)。
func (s *userService) recordLoginFailure(ctx *contracts.RequestContext, account string) {
	ip := ctx.ClientIP()
	s.LoginGuard.RecordFailure(ctx.Context, ip, account)
	// 记进 user 专属索引,供后台「被锁列表」枚举(loginguard 引擎无 SCAN)。best-effort。
	serviceRateLimit.RecordFailureIndex(ctx.Context, s.Redis, ip, account)
}

// recordLoginPending 记一次「密码已对但登录未完成」(待 2FA / 强制绑定 2FA):
// 只清**账号**失败计数(该账号密码已证明对)、**保留 IP** 失败计数(IP 跨账号聚合,防被当重置原语绕过)、不计成功。
func (s *userService) recordLoginPending(ctx *contracts.RequestContext, account string) {
	s.LoginGuard.RecordPending(ctx.Context, ctx.ClientIP(), account)
}

// recordLoginSuccess 记一次登录真正完成(直签 / 2FA 通过):清失败 + 计每小时成功数。
func (s *userService) recordLoginSuccess(ctx *contracts.RequestContext, account string) {
	s.LoginGuard.RecordSuccess(ctx.Context, ctx.ClientIP(), account)
}
