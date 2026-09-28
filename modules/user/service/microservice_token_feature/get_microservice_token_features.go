package microservice_token_feature

import (
	"fmt"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/modules/user/model/dto"
)

// GetMicroserviceTokenFeatures 获取所有微服务 token 功能配置
func (s *microserviceTokenFeatureService) GetMicroserviceTokenFeatures(ctx *contracts.RequestContext) ([]dto.MicroserviceTokenFeature, bizerr.BizError) {
	features, err := s.DL.GetAll(ctx.Context)
	if err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("failed to get microservice token features: %w", err))
	}

	result := make([]dto.MicroserviceTokenFeature, 0, len(features))
	for _, f := range features {
		result = append(result, dto.MicroserviceTokenFeature{
			ID:          f.ID,
			Name:        f.Name,
			Description: f.Description,
			FeatureList: []string(f.FeatureList),
			Created:     f.Created.Format("2006-01-02 15:04:05"),
			Modified:    f.Modified.Format("2006-01-02 15:04:05"),
		})
	}

	return result, nil
}
