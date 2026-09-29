package user

import (
	"errors"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/logger"
	"github.com/shyandsy/aurora/modules/user/model/dto"
)

// RefreshToken 用登录时下发的 refreshToken 换一组新的 access + refresh token。
// accessToken 过期后前端用它续期,避免强制重新登录(见 web 端 auth.interceptor 的 401 自动续期)。
// 与 customer 侧同源(api/services/customer/service/customer/refresh.go)。
func (s *userService) RefreshToken(ctx *contracts.RequestContext, req dto.RefreshReq) (*dto.RefreshResp, bizerr.BizError) {
	if req.RefreshToken == "" {
		msg := ctx.T("auth.refresh_token_required")
		return nil, bizerr.NewValidationError(msg, map[string]string{
			"refreshToken": msg,
		})
	}

	// token-IP 绑定:强制校验旧 refresh token 是否来自登录 IP(fail-close:异地续期一律拒)。
	// refresh 与 access 都在 localStorage,泄漏常一起被偷;不卡 refresh 的话攻击者可在自己 IP 续期、
	// 把新 token 绑到自己 IP,绕过 access 侧校验。
	rc, verr := s.JWT.ValidateRefreshToken(req.RefreshToken)
	if verr != nil {
		return nil, bizerr.New(401, errors.New(ctx.T("auth.refresh_token_invalid")))
	}
	// 会话级校验(撤销 + IP 绑定)与鉴权中间件共用 tokenguard 同一套判定:/auth/refresh 是公开端点、
	// 不走中间件,故这里显式调。撤销堵旧 refresh 重放(App 无 IP 绑定靠它);IP 绑定按 token 自带 sess:noip
	// 标决定(web 校验、app 跳过)。任一失败 fail-close,统一回 refresh_token_invalid(不泄漏具体原因)。
	if verr := s.TokenGuard.VerifySession(ctx.Context, rc, ctx.ClientIP()); verr != nil {
		return nil, bizerr.New(401, errors.New(ctx.T("auth.refresh_token_invalid")))
	}
	// 会话已被撤销 → 拒绝续期(堵"撤销 vs 并发刷新"的竞态:即便某个旧绑定漏删,已撤销会话也续不出新 token)。
	// 找不到会话(旧 token 早于本功能)→ 放行,保持向后兼容。
	if sess, serr := s.SessionDL.GetByRefreshJTI(ctx.Context, rc.ID); serr == nil && sess != nil && sess.Revoked {
		return nil, bizerr.New(401, errors.New(ctx.T("auth.refresh_token_invalid")))
	}

	// RefreshToken 内部校验签名/过期/黑名单,并签发新的 access+refresh 对
	tokenResp, err := s.JWT.RefreshToken(req.RefreshToken)
	if err != nil {
		logger.Errorf("RefreshToken: failed to refresh token, error=%+v", err)
		msg := ctx.T("auth.refresh_token_invalid")
		return nil, bizerr.New(401, errors.New(msg))
	}

	// 绑定新 access+refresh 到当前 IP(与登录路径一致,一律绑)。token 自带 sess:noip 标记的作用域,其
	// IP 校验在 Guard.VerifySession 内部自动跳过,故对这类 token 建立 IP 绑定是无害的死数据(从不被校)。
	s.bindTokenIP(ctx, tokenResp)
	// 把会话滚动到新 jti(按旧 refresh jti 定位)+ 失效旧的一对(IP 绑定 + jti 黑名单)。best-effort。
	s.rotateSession(ctx, rc.ID, tokenResp)

	return &dto.RefreshResp{
		AccessToken:     tokenResp.AccessToken,
		TokenType:       "bearer",
		ExpiresInSecond: tokenResp.ExpiresIn,
		RefreshToken:    tokenResp.RefreshToken,
	}, nil
}
