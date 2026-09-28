package microservice

import (
	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/modules/user/model/dto"
	serviceMicroserviceTokenFeature "github.com/shyandsy/aurora/modules/user/service/microservice_token_feature"
)

// CreateMicroserviceTokenFeature 创建微服务 token 功能配置
func CreateMicroserviceTokenFeature(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
	var req dto.CreateMicroserviceTokenFeatureReq
	if err := c.ShouldBindJSON(&req); err != nil {
		msg := c.T("error.bad_request")
		return nil, bizerr.NewValidationError(msg, nil)
	}

	var featureService serviceMicroserviceTokenFeature.MicroserviceTokenFeatureService
	if err := c.App.Find(&featureService); err != nil {
		return nil, bizerr.ErrInternalServerError(err)
	}

	resp, bizErr := featureService.CreateMicroserviceTokenFeature(c, req)
	if bizErr != nil {
		return nil, bizErr
	}

	return resp, nil
}
