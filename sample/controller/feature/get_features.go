package feature

import (
	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	serviceFeature "github.com/shyandsy/aurora/sample/service/feature"
)

// GetFeatures 获取功能列表
func GetFeatures(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
	var featureService serviceFeature.FeatureService
	if err := c.App.Find(&featureService); err != nil {
		return nil, bizerr.ErrInternalServerError(err)
	}

	features, bizErr := featureService.GetFeatures(c)
	if bizErr != nil {
		return nil, bizErr
	}

	return features, nil
}
