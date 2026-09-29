package user

import (
	"time"

	"github.com/shyandsy/aurora/contracts"
	auroraFeature "github.com/shyandsy/aurora/feature"
	"github.com/shyandsy/aurora/logger"
)

// bindTokenIP 把新签发的 access + refresh token(按各自 jti)绑定到当前登录 IP。写入失败只记日志
// (尽力而为,refresh/中间件侧 fail-close 兜底,不阻断登录)。会话有效性内核收口在 aurora tokenguard.Guard。
func (s *userService) bindTokenIP(ctx *contracts.RequestContext, tokenResp *auroraFeature.TokenResponse) {
	if tokenResp == nil {
		return
	}
	ip := ctx.ClientIP()
	if ac, err := s.JWT.ValidateToken(tokenResp.AccessToken); err == nil {
		if berr := s.TokenGuard.BindLoginIP(ctx.Context, ac.ID, ip, claimsTTL(ac)); berr != nil {
			logger.Errorf("bindTokenIP: 绑定 access token IP 失败(best-effort): %v", berr)
		}
	}
	if rc, err := s.JWT.ValidateRefreshToken(tokenResp.RefreshToken); err == nil {
		if berr := s.TokenGuard.BindLoginIP(ctx.Context, rc.ID, ip, claimsTTL(rc)); berr != nil {
			logger.Errorf("bindTokenIP: 绑定 refresh token IP 失败(best-effort): %v", berr)
		}
	}
}

// claimsTTL 返回 claims 距过期的剩余时长(<=0 视为不绑定)。aurora Guard.BindLoginIP 收 time.Duration。
func claimsTTL(c *auroraFeature.Claims) time.Duration {
	if c == nil || c.ExpiresAt == nil {
		return 0
	}
	d := time.Until(c.ExpiresAt.Time)
	if d < 0 {
		return 0
	}
	return d
}
