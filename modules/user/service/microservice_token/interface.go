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

// NewMicroserviceTokenService 创建微服务 token 服务。
//
// JWT 配置从 DI 解析(由宿主 AddFeature(feature.NewJWTFeature(cfg)) 注册进来),而不是自己
// 从 env 读 —— env 名归宿主(后台域可能叫 ADMIN_JWT_SECRET 等),本模块不假设。它需要原始
// Secret/Issuer 自行签「服务间调用」长效 token(见 issue_token.go),所以要的是整份 config。
func NewMicroserviceTokenService(app contracts.App) MicroserviceTokenService {
	var cfg *config.JWTConfig
	if err := app.Find(&cfg); err != nil || cfg == nil {
		panic(fmt.Errorf("microservice token service: JWT config not found in container — host must AddFeature(feature.NewJWTFeature(cfg)) first: %w", err))
	}
	s := &microserviceTokenService{Config: cfg}
	if err := app.Resolve(s); err != nil {
		panic(fmt.Errorf("failed to resolve MicroserviceTokenService: %w", err))
	}
	return s
}
