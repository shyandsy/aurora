package microservice_token_feature

import (
	"fmt"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
)

// DeleteMicroserviceTokenFeature 删除微服务 token 功能配置
func (s *microserviceTokenFeatureService) DeleteMicroserviceTokenFeature(ctx *contracts.RequestContext, id int64) bizerr.BizError {
	if err := s.DL.Delete(ctx.Context, id); err != nil {
		return bizerr.ErrInternalServerError(fmt.Errorf("failed to delete microservice token feature: %w", err))
	}
	return nil
}
