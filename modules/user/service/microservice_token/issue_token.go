package microservice_token

import (
	"fmt"
	"strings"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	auroraFeature "github.com/shyandsy/aurora/feature"
	"github.com/shyandsy/aurora/types"
	"github.com/shyandsy/aurora/modules/user/model/dto"
	"github.com/shyandsy/aurora/modules/user/model/entity"
)

// IssueToken 签发微服务 JWT token
func (s *microserviceTokenService) IssueToken(ctx *contracts.RequestContext, req dto.IssueMicroserviceTokenReq) (*dto.IssueMicroserviceTokenResp, bizerr.BizError) {
	// 验证请求
	if len(req.MicroserviceFeatureIDs) == 0 {
		return nil, bizerr.NewValidationError("microserviceFeatureIds are required", map[string]string{
			"microserviceFeatureIds": "microserviceFeatureIds are required",
		})
	}

	// 根据 ID 列表获取微服务配置
	features, err := s.FeatureDL.GetByIDs(ctx.Context, req.MicroserviceFeatureIDs)
	if err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("failed to get microservice features: %w", err))
	}

	// 验证是否所有 ID 都找到了
	if len(features) != len(req.MicroserviceFeatureIDs) {
		return nil, bizerr.NewValidationError("some microservice feature IDs not found", map[string]string{
			"microserviceFeatureIds": "some microservice feature IDs not found",
		})
	}

	// 合并 feature list（去重）
	featureMap := make(map[string]bool)
	featureList := make([]string, 0)
	descriptions := make([]string, 0)

	for _, feature := range features {
		descriptions = append(descriptions, feature.Description)
		for _, f := range feature.FeatureList {
			if !featureMap[f] {
				featureMap[f] = true
				featureList = append(featureList, f)
			}
		}
	}

	// 默认过期时间：1年
	defaultExpiry := 1 * 365 * 24 * time.Hour
	expiresIn := int64(defaultExpiry.Seconds())

	// 计算过期时间
	now := time.Now()
	expiresAt := now.Add(defaultExpiry)

	// 用 aurora 的 Claims 结构签发（**不要**再手搓 MapClaims）：与 aurora 令牌工厂同一套字段/json tag，
	// 关键是必须盖上 TokenType=access —— 否则 aurora ValidateToken 的 `token_type != "access"` 会把本
	// 令牌一律判 401（正是这类生产事故的根因：手搓 claim 漏了 token_type，服务间调用全挂）。
	// 说明：微服务令牌是「单个 access-only + 自定义 1 年长效」，而 aurora 目前只对外暴露 GenerateToken
	// （返回 access+refresh 一对、用配置里的短有效期），没有「自定义有效期签单个 access」的 API，故仍在此
	// 本地签名。真正的收口是给 aurora 增补一个 GenerateServiceToken(ttl) 工厂,见 doc/bug-analysis。
	claims := &auroraFeature.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    s.Config.Issuer,
			Subject:   "0", // 微服务 token，userID 为 0
		},
		UserID:    0,
		Email:     "",
		Features:  featureList,
		TokenType: auroraFeature.TokenTypeAccess,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	accessToken, err := token.SignedString([]byte(s.Config.Secret))
	if err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("failed to generate token: %w", err))
	}

	tokenResp := &auroraFeature.TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: "", // 微服务 token 不需要 refresh token
		ExpiresIn:    expiresIn,
	}

	// 保存 token 记录到数据库
	tokenRecord := &entity.MicroserviceTokenFeatureToken{
		MicroserviceFeatureIDs: types.JSONArray[int64](req.MicroserviceFeatureIDs),
		Description:            strings.Join(descriptions, "; "),
		FeatureList:            types.JSONArray[string](featureList),
		Token:                  tokenResp.AccessToken,
		Issuer:                 s.Config.Issuer,
		ExpiresAt:              expiresAt,
		ExpiresIn:              expiresIn,
		Status:                 types.GetDefaultEnabledDisabledStatus(),
	}

	if err := s.TokenDL.Create(ctx.Context, tokenRecord); err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("failed to save token record: %w", err))
	}

	return &dto.IssueMicroserviceTokenResp{
		Token:         tokenResp.AccessToken,
		Issuer:        s.Config.Issuer,
		ExpiresAt:     expiresAt.Unix(),
		ExpiresIn:     expiresIn,
		TokenRecordID: tokenRecord.ID,
	}, nil
}
