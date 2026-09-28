package microservice_token

import (
	"fmt"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/config"
	"github.com/shyandsy/aurora/contracts"
	auroraFeature "github.com/shyandsy/aurora/feature"
	"github.com/shyandsy/aurora/types"
	"github.com/shyandsy/aurora/modules/user/datalayer"
	"github.com/shyandsy/aurora/modules/user/model/dto"
)

// MicroserviceTokenService 微服务 token 服务接口（用于签发微服务调用所需的 token）
type MicroserviceTokenService interface {
	IssueToken(ctx *contracts.RequestContext, req dto.IssueMicroserviceTokenReq) (*dto.IssueMicroserviceTokenResp, bizerr.BizError)
	GetMicroserviceTokenFeatureTokens(ctx *contracts.RequestContext, req types.PagingReq) (*types.PagingResponse, bizerr.BizError)
	GetMicroserviceTokenFeatureToken(ctx *contracts.RequestContext, id int64) (*dto.MicroserviceTokenFeatureToken, bizerr.BizError)
	EnableToken(ctx *contracts.RequestContext, id int64) bizerr.BizError
	DisableToken(ctx *contracts.RequestContext, id int64) bizerr.BizError
}

// microserviceTokenService 微服务 token 服务实现
type microserviceTokenService struct {
	JWT          auroraFeature.JWTService   `inject:""`
	RedisService auroraFeature.RedisService `inject:""`
	Config       *config.JWTConfig
	FeatureDL    datalayer.MicroserviceTokenFeatureDatalayer      `inject:""`
	TokenDL      datalayer.MicroserviceTokenFeatureTokenDatalayer `inject:""`
}

// NewMicroserviceTokenService 创建微服务 token 服务
func NewMicroserviceTokenService(app contracts.App) MicroserviceTokenService {
	cfg := &config.JWTConfig{}
	if err := config.ResolveConfig(cfg); err != nil {
		panic(fmt.Errorf("failed to load JWT config: %w", err))
	}
	s := &microserviceTokenService{Config: cfg}
	if err := app.Resolve(s); err != nil {
		panic(fmt.Errorf("failed to resolve MicroserviceTokenService: %w", err))
	}
	return s
}
