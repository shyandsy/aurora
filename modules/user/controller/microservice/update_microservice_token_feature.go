package microservice

import (
	"strconv"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/modules/user/model/dto"
	serviceMicroserviceTokenFeature "github.com/shyandsy/aurora/modules/user/service/microservice_token_feature"
)

// UpdateMicroserviceTokenFeature 更新微服务 token 功能配置
func UpdateMicroserviceTokenFeature(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
	idStr := c.Param("id")
	if idStr == "" {
		msg := c.T("error.bad_request")
		return nil, bizerr.NewValidationError(msg, nil)
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		msg := c.T("error.bad_request")
		return nil, bizerr.NewValidationError(msg, nil)
	}

	var req dto.UpdateMicroserviceTokenFeatureReq
	if err := c.ShouldBindJSON(&req); err != nil {
		msg := c.T("error.bad_request")
		return nil, bizerr.NewValidationError(msg, nil)
	}

	var featureService serviceMicroserviceTokenFeature.MicroserviceTokenFeatureService
	if err := c.App.Find(&featureService); err != nil {
		return nil, bizerr.ErrInternalServerError(err)
	}

	resp, bizErr := featureService.UpdateMicroserviceTokenFeature(c, id, req)
	if bizErr != nil {
		return nil, bizErr
	}

	return resp, nil
}
