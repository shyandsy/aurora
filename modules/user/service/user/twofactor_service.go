package user

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/middleware"
	"github.com/shyandsy/aurora/modules/user/model/dto"
	"github.com/shyandsy/aurora/modules/user/model/entity"
	"golang.org/x/crypto/bcrypt"
)

// 验码限流:同一用户连续失败 twofaMaxAttempts 次锁 twofaLockWindow(防 6 位码暴力破解)。
const (
	twofaMaxAttempts = 5
	twofaLockWindow  = 15 * time.Minute
)

// SetupTotp 开始绑定(需已登录 + 重验密码):生成密钥、加密落库(未启用),返回二维码 URI + 密钥。
func (s *userService) SetupTotp(ctx *contracts.RequestContext, req dto.TotpSetupReq) (*dto.TotpSetupResp, bizerr.BizError) {
	user, bizErr := s.currentUser(ctx)
	if bizErr != nil {
		return nil, bizErr
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, bizerr.NewValidationError(ctx.T("auth.login_failed"), map[string]string{"password": ctx.T("auth.login_failed")})
	}
	if user.TotpEnabled {
		return nil, bizerr.NewValidationError(ctx.T("auth.totp_already_enabled"), nil)
	}

	secret, uri, err := generateTotpSecret(user.Email)
	if err != nil {
		return nil, internalErr(ctx, "user", err)
	}
	enc, err := encryptCredential(secret)
	if err != nil {
		return nil, internalErr(ctx, "user", err)
	}
	user.TotpSecret = enc
	user.TotpEnabled = false
	user.TotpRecoveryCodes = ""
	if err := s.DL.Update(ctx.Context, user); err != nil {
		return nil, internalErr(ctx, "user", err)
	}
	return &dto.TotpSetupResp{OtpauthURI: uri, Secret: secret}, nil
}

// ConfirmTotp 确认绑定:验一次 app 上的码,通过则启用 + 生成一次性备用码(仅此一次返回)。
func (s *userService) ConfirmTotp(ctx *contracts.RequestContext, req dto.TotpConfirmReq) (*dto.TotpConfirmResp, bizerr.BizError) {
	user, bizErr := s.currentUser(ctx)
	if bizErr != nil {
		return nil, bizErr
	}
	locked, lerr := s.twofaLocked(ctx, user.ID)
	if lerr != nil {
		// fail-close:锁计数后端(Redis)不可用时拒绝验码,不放行。
		return nil, bizerr.New(503, errors.New(ctx.T("auth.totp_backend_unavailable")))
	}
	if locked {
		return nil, bizerr.NewValidationError(ctx.T("auth.totp_locked"), nil)
	}
	secret, err := decryptCredential(user.TotpSecret)
	if err != nil || secret == "" {
		return nil, bizerr.NewValidationError(ctx.T("auth.totp_not_setup"), nil)
	}
	if !validateTotp(secret, req.Code) {
		s.twofaRecordFail(ctx, user.ID)
		return nil, bizerr.NewValidationError(ctx.T("auth.totp_invalid"), map[string]string{"code": ctx.T("auth.totp_invalid")})
	}

	plain, stored, err := generateRecoveryCodes()
	if err != nil {
		return nil, internalErr(ctx, "user", err)
	}
	user.TotpEnabled = true
	user.TotpRecoveryCodes = stored
	if err := s.DL.Update(ctx.Context, user); err != nil {
		return nil, internalErr(ctx, "user", err)
	}
	// 绑定成功即换正式 token:强制绑定场景下,绑定完直接进系统、无需重登(仅绑定 token → 正式 token)。
	login, be := s.issueTokens(ctx, user)
	if be != nil {
		return nil, be
	}
	return &dto.TotpConfirmResp{RecoveryCodes: plain, Login: login}, nil
}

// requireTwoFactor 是否强制两步验证。由 env USER_REQUIRE_TWO_FACTOR 决定:
// 未设 / 非 "false" 一律视为强制(默认开、从严);仅显式 "false" 关闭。
// 收成 env(不落 DB 设置表)是为了 user 服务**自包含**——整体复制到别的 aurora 项目改个
// import 前缀即可直接跑,无需连宿主项目的 schema。每个项目用自己的 env 定策略。
func (s *userService) requireTwoFactor() bool {
	return !strings.EqualFold(strings.TrimSpace(os.Getenv(userRequireTwoFactorEnv)), "false")
}

// userRequireTwoFactorEnv 强制两步验证开关的环境变量名。默认(未设)= 强制;仅 "false" 关闭。
const userRequireTwoFactorEnv = "USER_REQUIRE_TWO_FACTOR"

// DisableTotp 关闭两步验证:需密码 + 一个当前码,通过则清空密钥/启用位/备用码。
func (s *userService) DisableTotp(ctx *contracts.RequestContext, req dto.TotpDisableReq) bizerr.BizError {
	user, bizErr := s.currentUser(ctx)
	if bizErr != nil {
		return bizErr
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return bizerr.NewValidationError(ctx.T("auth.login_failed"), map[string]string{"password": ctx.T("auth.login_failed")})
	}
	if !user.TotpEnabled {
		return nil // 本就没开,幂等成功
	}
	secret, err := decryptCredential(user.TotpSecret)
	if err != nil || secret == "" || !validateTotp(secret, req.Code) {
		return bizerr.NewValidationError(ctx.T("auth.totp_invalid"), map[string]string{"code": ctx.T("auth.totp_invalid")})
	}
	user.TotpSecret = ""
	user.TotpEnabled = false
	user.TotpRecoveryCodes = ""
	if err := s.DL.Update(ctx.Context, user); err != nil {
		return internalErr(ctx, "user", err)
	}
	return nil
}

// LoginTwoFactor 登录第二步:验 pendingToken + TOTP 码(或备用码),通过发正式 token。
func (s *userService) LoginTwoFactor(ctx *contracts.RequestContext, req dto.Login2FAReq) (*dto.LoginResp, bizerr.BizError) {
	// 登录第二步同属登录爆破面(6 位 TOTP 码可被暴力枚举):先按 IP 预检(硬锁/每小时上限)。
	if be := s.precheckLogin(ctx, ""); be != nil {
		return nil, be
	}
	userID, err := parsePendingToken(req.PendingToken)
	if err != nil {
		return nil, bizerr.NewValidationError(ctx.T("auth.totp_pending_invalid"), nil)
	}
	locked, lerr := s.twofaLocked(ctx, userID)
	if lerr != nil {
		// fail-close:锁计数后端(Redis)不可用时拒绝验码,不放行。
		return nil, bizerr.New(503, errors.New(ctx.T("auth.totp_backend_unavailable")))
	}
	if locked {
		return nil, bizerr.NewValidationError(ctx.T("auth.totp_locked"), nil)
	}
	user, derr := s.DL.GetByID(ctx.Context, userID)
	if derr != nil {
		return nil, internalErr(ctx, "user", derr)
	}
	if user == nil || !user.TotpEnabled {
		return nil, bizerr.NewValidationError(ctx.T("auth.totp_pending_invalid"), nil)
	}

	// 账号维度硬锁预检(此步才拿到 email;硬锁模式下被锁则拒)。account 用明文 email。
	if d := s.LoginGuard.PrecheckAccount(ctx.Context, user.Email); d.Blocked {
		return nil, loginRateLimited(ctx)
	}

	secret, derr := decryptCredential(user.TotpSecret)
	if derr != nil {
		return nil, internalErr(ctx, "user", derr)
	}
	// 先试 TOTP 码;不过再试备用码(命中即作废,落库)。
	if validateTotp(secret, req.Code) {
		// 登录真正完成:清失败 + 计每小时成功数。
		s.recordLoginSuccess(ctx, user.Email)
		return s.issueTokens(ctx, user)
	}
	if newStored, ok := consumeRecoveryCode(user.TotpRecoveryCodes, req.Code); ok {
		user.TotpRecoveryCodes = newStored
		if uerr := s.DL.Update(ctx.Context, user); uerr != nil {
			return nil, internalErr(ctx, "user", uerr)
		}
		// 登录真正完成(用备用码):清失败 + 计每小时成功数。
		s.recordLoginSuccess(ctx, user.Email)
		return s.issueTokens(ctx, user)
	}

	s.twofaRecordFail(ctx, userID)
	// TOTP/备用码错误:**账号维度**的 2FA 锁由上面的 twofaRecordFail 独占(fail-close,专防 6 位码枚举);
	// loginguard 这里只计 **IP** 维度(account 传空),挡「一个 IP 狂刷多账号的 2FA」——twofaLocked 是 per-用户、
	// 管不了跨账号的 IP 喷。故不重复计账号,避免与 twofaRecordFail 双锁同一账号。
	s.recordLoginFailure(ctx, "")
	return nil, bizerr.NewValidationError(ctx.T("auth.totp_invalid"), map[string]string{"code": ctx.T("auth.totp_invalid")})
}

// ---- 辅助 ----

// currentUser 从 JWT 上下文取当前用户并加载。
func (s *userService) currentUser(ctx *contracts.RequestContext) (*entity.User, bizerr.BizError) {
	userID, ok := middleware.GetUserID(ctx.Context)
	if !ok {
		return nil, bizerr.ErrUnauthorized()
	}
	user, err := s.DL.GetByID(ctx.Context, userID)
	if err != nil {
		return nil, internalErr(ctx, "user", err)
	}
	if user == nil {
		return nil, bizerr.ErrUnauthorized()
	}
	return user, nil
}

// twofaKey 验码失败计数 redis key。
func twofaKey(userID int64) string { return fmt.Sprintf("user:2fa:fail:%d", userID) }

// twofaLocked 当前是否已锁(失败次数达上限)。
//
// fail-close:Redis 不可用(未配置或连接出错)时返回 error,调用方**拒绝**继续验码。
// 不 fail-open——2FA 的暴力破解锁全靠这个计数,Redis 一挂就放行等于把唯一的 TOTP 限速闸拆了;
// 宁可 Redis 抖动时暂时登不进(可用性),也不留爆破窗口(安全性)。
// 注意区分「key 不存在」(Get 返回 "",nil = 尚无失败记录 → 未锁,放行)与「Redis 出错」
// (err != nil = 后端不可用 → 拒绝),否则新用户零失败也会被误锁。
func (s *userService) twofaLocked(ctx *contracts.RequestContext, userID int64) (bool, error) {
	if s.Redis == nil {
		return false, errors.New("2fa lock backend unavailable")
	}
	v, err := s.Redis.Get(ctx.Context, twofaKey(userID))
	if err != nil {
		return false, err
	}
	if v == "" {
		return false, nil
	}
	n, _ := strconv.Atoi(v)
	return n >= twofaMaxAttempts, nil
}

// twofaRecordFail 记一次失败;首次失败时设过期窗口。
func (s *userService) twofaRecordFail(ctx *contracts.RequestContext, userID int64) {
	if s.Redis == nil {
		return
	}
	key := twofaKey(userID)
	n, err := s.Redis.Incr(ctx.Context, key)
	if err == nil && n == 1 {
		_ = s.Redis.Expire(ctx.Context, key, twofaLockWindow)
	}
}
