package microservice_token_feature

import (
	"fmt"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/modules/user/model/dto"
)

// GetMicroserviceTokenFeature 根据 ID 获取微服务 token 功能配置
func (s *microserviceTokenFeatureService) GetMicroserviceTokenFeature(ctx *contracts.RequestContext, id int64) (*dto.MicroserviceTokenFeature, bizerr.BizError) {
	feature, err := s.DL.GetByID(ctx.Context, id)
	if err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("failed to get microservice token feature: %w", err))
	}

	return &dto.MicroserviceTokenFeature{
		ID:          feature.ID,
		Name:        feature.Name,
		Description: feature.Description,
		FeatureList: []string(feature.FeatureList),
		Created:     feature.Created.Format("2006-01-02 15:04:05"),
		Modified:    feature.Modified.Format("2006-01-02 15:04:05"),
	}, nil
}
