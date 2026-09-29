package microservice_token_feature

import (
	"fmt"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/types"
	"github.com/shyandsy/aurora/modules/user/model/dto"
	"github.com/shyandsy/aurora/modules/user/model/entity"
)

// UpdateMicroserviceTokenFeature 更新微服务 token 功能配置
func (s *microserviceTokenFeatureService) UpdateMicroserviceTokenFeature(ctx *contracts.RequestContext, id int64, req dto.UpdateMicroserviceTokenFeatureReq) (*dto.MicroserviceTokenFeature, bizerr.BizError) {
	// 检查是否存在
	_, err := s.DL.GetByID(ctx.Context, id)
	if err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("microservice token feature not found: %w", err))
	}

	// 构建更新对象
	update := &entity.MicroserviceTokenFeature{}
	if req.Name != nil {
		update.Name = *req.Name
	}
	if req.Description != nil {
		update.Description = *req.Description
	}
	if req.FeatureList != nil {
		update.FeatureList = types.JSONArray[string](req.FeatureList)
	}

	if err := s.DL.Update(ctx.Context, id, update); err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("failed to update microservice token feature: %w", err))
	}

	// 重新查询以获取更新后的数据
	updated, err := s.DL.GetByID(ctx.Context, id)
	if err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("failed to get updated microservice token feature: %w", err))
	}

	return &dto.MicroserviceTokenFeature{
		ID:          updated.ID,
		Name:        updated.Name,
		Description: updated.Description,
		FeatureList: []string(updated.FeatureList),
		Created:     updated.Created.Format("2006-01-02 15:04:05"),
		Modified:    updated.Modified.Format("2006-01-02 15:04:05"),
	}, nil
}
