package microservice_token

import (
	"fmt"
	"time"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	auroraFeature "github.com/shyandsy/aurora/feature"
	"github.com/shyandsy/aurora/types"
)

// DisableToken 禁用微服务 token（加入黑名单）
func (s *microserviceTokenService) DisableToken(ctx *contracts.RequestContext, id int64) bizerr.BizError {
	// 获取 token 记录
	token, err := s.TokenDL.GetByID(ctx.Context, id)
	if err != nil {
		return bizerr.ErrInternalServerError(fmt.Errorf("failed to get token: %w", err))
	}

	// 检查当前状态
	if token.Status == types.EnabledDisabledStatusDisabled {
		// 已经是禁用状态，直接返回
		return nil
	}

	// 更新数据库状态
	if err := s.TokenDL.UpdateStatus(ctx.Context, id, types.EnabledDisabledStatusDisabled); err != nil {
		return bizerr.ErrInternalServerError(fmt.Errorf("failed to update token status: %w", err))
	}

	// 将 token 加入黑名单
	// 使用数据库中的过期时间计算 TTL
	ttl := time.Until(token.ExpiresAt)
	if ttl <= 0 {
		// Token 已过期，不需要加入黑名单
		return nil
	}

	// 将 token 加入黑名单，TTL 设置为 token 的剩余过期时间。
	// key 必须复用 aurora 导出的前缀常量(与 ValidateToken 查的同一 key),**禁止手写字面量** ——
	// 否则 aurora 一改前缀,这里写旧 key、校验查新 key → 禁用静默失效(安全洞)。
	blacklistKey := fmt.Sprintf("%s:%s", auroraFeature.RedisKeyBlackAccessTokenPrefix, token.Token)
	if err := s.RedisService.Set(ctx.Context, blacklistKey, "1", ttl); err != nil {
		// 记录错误但不影响禁用操作
		// 因为即使 Redis 操作失败，数据库状态已经更新，下次验证时可以通过数据库状态判断
	}

	return nil
}
