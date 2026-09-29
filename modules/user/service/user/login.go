package user

import (
	"net/mail"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/middleware"
	"github.com/shyandsy/aurora/types"
	"github.com/shyandsy/aurora/modules/user/model/dto"
	"golang.org/x/crypto/bcrypt"
)

func (s *userService) Login(ctx *contracts.RequestContext, req dto.LoginReq) (*dto.LoginResp, bizerr.BizError) {
	if req.Email == "" {
		msg := ctx.T("user.email_required")
		return nil, bizerr.NewValidationError(msg, map[string]string{
			"email": msg,
		})
	}
	if _, err := mail.ParseAddress(req.Email); err != nil {
		msg := ctx.T("user.invalid_email")
		return nil, bizerr.NewValidationError(msg, map[string]string{
			"email": msg,
		})
	}

	if req.Password == "" {
		msg := ctx.T("user.password_required")
		return nil, bizerr.NewValidationError(msg, map[string]string{
			"password": msg,
		})
	}

	// 登录前暴力破解预检(IP 硬锁/每小时上限 + 账号硬锁)。放在验密码之前——账号一旦被硬锁,
	// 连 bcrypt 都不必跑。account 用**明文 email**(硬锁模式便于运维去 Redis 手动解锁)。
	if be := s.precheckLogin(ctx, req.Email); be != nil {
		return nil, be
	}

	// Get user by email (with Role preloaded)
	user, err := s.DL.GetByEmail(ctx.Context, req.Email)
	if err != nil {
		return nil, internalErr(ctx, "user", err)
	}
	// 统一鉴权失败:不存在 / 密码错 / 禁用 一律返回同一 auth.login_failed(不分字段),且每条路径都
	// 恰好过一次 bcrypt —— 防账号枚举(不同文案/字段泄漏邮箱是否存在、是否被禁用)与计时侧信道
	// (不存在的用户不跑 bcrypt 会明显更快)。
	if user == nil {
		_ = bcrypt.CompareHashAndPassword(dummyBcryptHash, []byte(req.Password)) // 抹平计时
		// 凭据失败(账号不存在)= 权威判定点:计失败,达阈值按策略锁 IP/账号。
		s.recordLoginFailure(ctx, req.Email)
		return nil, loginFailed(ctx)
	}
	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)) != nil {
		// 凭据失败(密码错)= 权威判定点。
		s.recordLoginFailure(ctx, req.Email)
		return nil, loginFailed(ctx)
	}
	// 密码已验对:账号停用不是「凭据失败」(不是爆破信号),故不 RecordFailure;
	// 但也未登录成功,不清计数、不计成功 —— 什么都不记。
	if user.Status != types.StatusEnable {
		return nil, loginFailed(ctx)
	}

	// 开了两步验证:密码验过后不直接发 JWT,发短时挑战 token,前端弹验证码框走 /auth/login/2fa。
	if user.TotpEnabled {
		pending, perr := makePendingToken(user.ID)
		if perr != nil {
			return nil, internalErr(ctx, "user", perr)
		}
		// 密码对但登录未完成(待 2FA):只清账号失败、保留 IP、不计成功。
		s.recordLoginPending(ctx, req.Email)
		return &dto.LoginResp{TwoFactorRequired: true, PendingToken: pending}, nil
	}

	// 强制两步验证但该账号尚未绑定:发**仅绑定 token**(带 ScopeEnroll2FA 哨兵,后端中间件硬拦在
	// 所有业务接口之外,只放行 2FA 绑定/登出),前端据 MustEnrollTwoFactor 强制引导绑定;
	// 绑定 confirm 成功后换正式 token 才能操作。绝不再发空权限但畅通的 token(否则等于绕过 2FA)。
	if s.requireTwoFactor() {
		tok, terr := s.JWT.GenerateToken(user.ID, user.Email, []string{middleware.ScopeEnroll2FA})
		if terr != nil {
			return nil, internalErr(ctx, "user", terr)
		}
		s.bindTokenIP(ctx, tok)
		// 密码对但登录未完成(强制绑定 2FA 中,仅拿到受限的绑定 token):只清账号失败、保留 IP、不计成功。
		s.recordLoginPending(ctx, req.Email)
		userDto := user.ToDto()
		userDto.Features = []string{}
		return &dto.LoginResp{
			AccessToken:         tok.AccessToken,
			TokenType:           "bearer",
			ExpiresInSecond:     tok.ExpiresIn,
			RefreshToken:        tok.RefreshToken,
			Features:            []string{},
			User:                userDto,
			MustEnrollTwoFactor: true,
		}, nil
	}

	// 登录真正完成(无 2FA,直接签发正式 token):清失败 + 计每小时成功数。
	s.recordLoginSuccess(ctx, req.Email)
	// 签发委托给唯一的 token 签发中枢(见 token_issuer.go):作用域/绑 IP/权限上限全部数据驱动,此处不判断客户端类型。
	return s.issueTokens(ctx, user)
}
