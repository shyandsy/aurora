package microservice_token_feature

import (
	"fmt"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/modules/user/datalayer"
	"github.com/shyandsy/aurora/modules/user/model/dto"
)

// MicroserviceTokenFeatureService 微服务 token 功能配置服务接口
type MicroserviceTokenFeatureService interface {
	GetMicroserviceTokenFeatures(ctx *contracts.RequestContext) ([]dto.MicroserviceTokenFeature, bizerr.BizError)
	GetMicroserviceTokenFeature(ctx *contracts.RequestContext, id int64) (*dto.MicroserviceTokenFeature, bizerr.BizError)
	CreateMicroserviceTokenFeature(ctx *contracts.RequestContext, req dto.CreateMicroserviceTokenFeatureReq) (*dto.MicroserviceTokenFeature, bizerr.BizError)
	UpdateMicroserviceTokenFeature(ctx *contracts.RequestContext, id int64, req dto.UpdateMicroserviceTokenFeatureReq) (*dto.MicroserviceTokenFeature, bizerr.BizError)
	DeleteMicroserviceTokenFeature(ctx *contracts.RequestContext, id int64) bizerr.BizError
}

// microserviceTokenFeatureService 微服务 token 功能配置服务实现
type microserviceTokenFeatureService struct {
	DL datalayer.MicroserviceTokenFeatureDatalayer `inject:""`
}

// NewMicroserviceTokenFeatureService 创建微服务 token 功能配置服务
func NewMicroserviceTokenFeatureService(app contracts.App) MicroserviceTokenFeatureService {
	s := &microserviceTokenFeatureService{}
	if err := app.Resolve(s); err != nil {
		panic(fmt.Errorf("failed to resolve MicroserviceTokenFeatureService: %w", err))
	}
	return s
}
