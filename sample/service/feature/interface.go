package feature

import (
	"fmt"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/sample/datalayer"
	"github.com/shyandsy/aurora/sample/model/dto"
)

// FeatureService 功能服务接口
type FeatureService interface {
	GetFeatures(ctx *contracts.RequestContext) ([]dto.Feature, bizerr.BizError)
	GetFeature(ctx *contracts.RequestContext, id int64) (*dto.Feature, bizerr.BizError)
}

// featureService 功能服务实现
type featureService struct {
	DL datalayer.FeatureDatalayer `inject:""`
}

// NewFeatureService 创建功能服务
func NewFeatureService(app contracts.App) FeatureService {
	s := &featureService{}
	if err := app.Resolve(s); err != nil {
		panic(fmt.Errorf("failed to resolve FeatureService: %w", err))
	}
	return s
}
