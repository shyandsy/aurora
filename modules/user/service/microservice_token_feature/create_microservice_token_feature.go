package microservice_token_feature

import (
	"fmt"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/types"
	"github.com/shyandsy/aurora/modules/user/model/dto"
	"github.com/shyandsy/aurora/modules/user/model/entity"
)

// CreateMicroserviceTokenFeature 创建微服务 token 功能配置
func (s *microserviceTokenFeatureService) CreateMicroserviceTokenFeature(ctx *contracts.RequestContext, req dto.CreateMicroserviceTokenFeatureReq) (*dto.MicroserviceTokenFeature, bizerr.BizError) {
	feature := &entity.MicroserviceTokenFeature{
		Name:        req.Name,
		Description: req.Description,
		FeatureList: types.JSONArray[string](req.FeatureList),
	}

	if err := s.DL.Create(ctx.Context, feature); err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("failed to create microservice token feature: %w", err))
	}

	// 重新查询以获取完整数据（包括 created, modified）
	created, err := s.DL.GetByID(ctx.Context, feature.ID)
	if err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("failed to get created microservice token feature: %w", err))
	}

	return &dto.MicroserviceTokenFeature{
		ID:          created.ID,
		Name:        created.Name,
		Description: created.Description,
		FeatureList: []string(created.FeatureList),
		Created:     created.Created.Format("2006-01-02 15:04:05"),
		Modified:    created.Modified.Format("2006-01-02 15:04:05"),
	}, nil
}
