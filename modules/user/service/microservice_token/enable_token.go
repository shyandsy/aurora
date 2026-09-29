package microservice_token

import (
	"fmt"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	auroraFeature "github.com/shyandsy/aurora/feature"
	"github.com/shyandsy/aurora/types"
)

// EnableToken 启用微服务 token（从黑名单中移除）
func (s *microserviceTokenService) EnableToken(ctx *contracts.RequestContext, id int64) bizerr.BizError {
	// 获取 token 记录
	token, err := s.TokenDL.GetByID(ctx.Context, id)
	if err != nil {
		return bizerr.ErrInternalServerError(fmt.Errorf("failed to get token: %w", err))
	}

	// 检查当前状态
	if token.Status == types.EnabledDisabledStatusEnabled {
		// 已经是启用状态，直接返回
		return nil
	}

	// 更新数据库状态
	if err := s.TokenDL.UpdateStatus(ctx.Context, id, types.EnabledDisabledStatusEnabled); err != nil {
		return bizerr.ErrInternalServerError(fmt.Errorf("failed to update token status: %w", err))
	}

	// 从黑名单中移除 token。key 复用 aurora 导出的前缀常量(与 ValidateToken/DisableToken 同一 key),
	// **禁止手写字面量**,防前缀漂移导致启用/禁用与校验对不上。
	blacklistKey := fmt.Sprintf("%s:%s", auroraFeature.RedisKeyBlackAccessTokenPrefix, token.Token)
	if _, err := s.RedisService.Delete(ctx.Context, blacklistKey); err != nil {
		// 记录错误但不影响启用操作
		// 因为如果 token 不在黑名单中，Delete 操作也不会报错
		// 这里只是确保如果 token 在黑名单中，会被移除
	}

	return nil
}
