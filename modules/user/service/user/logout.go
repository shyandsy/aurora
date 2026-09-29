package user

import (
	"errors"
	"strings"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/logger"
	"github.com/shyandsy/aurora/modules/user/model/dto"
)

// Logout 登出:把当前 accessToken 和 refreshToken 都加入黑名单,使其立即失效。
// 路由受 JWT 中间件保护,accessToken 从 Authorization 头提取,refreshToken 从 body 提取。
// 与 customer 侧同源(见 api/services/customer/service/customer/logout.go):否则泄露的 token
// 在自然过期(JWT_EXPIRE_TIME)前仍可被他人使用。
func (s *userService) Logout(ctx *contracts.RequestContext, req dto.LogoutReq) bizerr.BizError {
	accessToken := extractBearerToken(ctx.GetHeader("Authorization"))

	// 先把对应会话标为撤销(须在拉黑前:拉黑后 refresh 进黑名单就解不出 jti 了)。best-effort。
	s.revokeSessionByRefreshToken(ctx, req.RefreshToken)

	if err := s.JWT.Logout(accessToken, req.RefreshToken); err != nil {
		logger.Errorf("Logout: failed to blacklist tokens, error=%+v", err)
		msg := ctx.T("error.internal_server")
		return bizerr.ErrInternalServerError(errors.New(msg))
	}

	return nil
}

// extractBearerToken 从 "Bearer <token>" 头里取出 token；格式不符返回空串。
func extractBearerToken(authHeader string) string {
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return ""
	}
	return parts[1]
}
