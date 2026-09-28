package microservice_token

import (
	"fmt"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/modules/user/model/dto"
)

// GetMicroserviceTokenFeatureToken 根据 ID 获取微服务 token 记录详情
func (s *microserviceTokenService) GetMicroserviceTokenFeatureToken(ctx *contracts.RequestContext, id int64) (*dto.MicroserviceTokenFeatureToken, bizerr.BizError) {
	token, err := s.TokenDL.GetByID(ctx.Context, id)
	if err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("failed to get microservice token feature token: %w", err))
	}

	return &dto.MicroserviceTokenFeatureToken{
		ID:                     token.ID,
		MicroserviceFeatureIDs: []int64(token.MicroserviceFeatureIDs),
		Description:            token.Description,
		FeatureList:            []string(token.FeatureList),
		Token:                  token.Token,
		Issuer:                 token.Issuer,
		ExpiresAt:              token.ExpiresAt.Format("2006-01-02 15:04:05"),
		ExpiresIn:              token.ExpiresIn,
		Status:                 token.Status,
		Created:                token.Created.Format("2006-01-02 15:04:05"),
		Modified:               token.Modified.Format("2006-01-02 15:04:05"),
	}, nil
}
