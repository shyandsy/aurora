package microservice

import (
	"strconv"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	serviceMicroserviceTokenFeature "github.com/shyandsy/aurora/modules/user/service/microservice_token_feature"
)

// DeleteMicroserviceTokenFeature 删除微服务 token 功能配置
func DeleteMicroserviceTokenFeature(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
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

	var featureService serviceMicroserviceTokenFeature.MicroserviceTokenFeatureService
	if err := c.App.Find(&featureService); err != nil {
		return nil, bizerr.ErrInternalServerError(err)
	}

	bizErr := featureService.DeleteMicroserviceTokenFeature(c, id)
	if bizErr != nil {
		return nil, bizErr
	}

	return map[string]string{"message": "success"}, nil
}
