package microservice

import (
	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	serviceMicroserviceTokenFeature "github.com/shyandsy/aurora/modules/user/service/microservice_token_feature"
)

// GetMicroserviceTokenFeatures 获取所有微服务 token 功能配置
func GetMicroserviceTokenFeatures(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
	var featureService serviceMicroserviceTokenFeature.MicroserviceTokenFeatureService
	if err := c.App.Find(&featureService); err != nil {
		return nil, bizerr.ErrInternalServerError(err)
	}

	resp, bizErr := featureService.GetMicroserviceTokenFeatures(c)
	if bizErr != nil {
		return nil, bizErr
	}

	return resp, nil
}
