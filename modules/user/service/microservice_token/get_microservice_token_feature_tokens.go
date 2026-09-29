package microservice_token

import (
	"fmt"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/types"
	"github.com/shyandsy/aurora/modules/user/model/dto"
)

// GetMicroserviceTokenFeatureTokens 分页获取微服务 token 记录
func (s *microserviceTokenService) GetMicroserviceTokenFeatureTokens(ctx *contracts.RequestContext, req types.PagingReq) (*types.PagingResponse, bizerr.BizError) {
	// 设置默认值
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}

	offset := int((req.Page - 1) * req.PageSize)
	pageSize := int(req.PageSize)
	tokens, total, err := s.TokenDL.GetAll(ctx.Context, offset, pageSize)
	if err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("failed to get microservice token feature tokens: %w", err))
	}

	items := make([]interface{}, 0, len(tokens))
	for _, t := range tokens {
		items = append(items, dto.MicroserviceTokenFeatureToken{
			ID:                     t.ID,
			MicroserviceFeatureIDs: []int64(t.MicroserviceFeatureIDs),
			Description:            t.Description,
			FeatureList:            []string(t.FeatureList),
			Token:                  t.Token,
			Issuer:                 t.Issuer,
			ExpiresAt:              t.ExpiresAt.Format("2006-01-02 15:04:05"),
			ExpiresIn:              t.ExpiresIn,
			Status:                 t.Status,
			Created:                t.Created.Format("2006-01-02 15:04:05"),
			Modified:               t.Modified.Format("2006-01-02 15:04:05"),
		})
	}

	totalPages := int32((total + int64(pageSize) - 1) / int64(pageSize))
	return &types.PagingResponse{
		Page:       req.Page,
		PageSize:   req.PageSize,
		Total:      total,
		TotalPages: totalPages,
		HasNext:    req.Page < totalPages,
		HasPrev:    req.Page > 1,
		Items:      items,
	}, nil
}
