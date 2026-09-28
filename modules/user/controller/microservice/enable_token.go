package microservice

import (
	"strconv"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	serviceMicroserviceToken "github.com/shyandsy/aurora/modules/user/service/microservice_token"
)

// EnableToken 启用微服务 token
func EnableToken(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
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

	var tokenService serviceMicroserviceToken.MicroserviceTokenService
	if err := c.App.Find(&tokenService); err != nil {
		return nil, bizerr.ErrInternalServerError(err)
	}

	bizErr := tokenService.EnableToken(c, id)
	if bizErr != nil {
		return nil, bizErr
	}

	return map[string]string{"message": "Token enabled successfully"}, nil
}
