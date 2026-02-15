package feature

import (
	"fmt"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/sample/full_showcase/datalayer"
	"github.com/shyandsy/aurora/sample/full_showcase/model/dto"
)

// FeatureService is the feature service interface.
type FeatureService interface {
	GetFeatures(ctx *contracts.RequestContext) ([]dto.Feature, bizerr.BizError)
	GetFeature(ctx *contracts.RequestContext, id int64) (*dto.Feature, bizerr.BizError)
}

// featureService is the feature service implementation.
type featureService struct {
	DL datalayer.FeatureDatalayer `inject:""`
}

// NewFeatureService creates the feature service.
func NewFeatureService(app contracts.App) FeatureService {
	s := &featureService{}
	if err := app.Resolve(s); err != nil {
		panic(fmt.Errorf("failed to resolve FeatureService: %w", err))
	}
	return s
}
