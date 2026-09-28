package microservice

import (
	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/modules/user/model/dto"
	serviceMicroserviceToken "github.com/shyandsy/aurora/modules/user/service/microservice_token"
)

// IssueJWTToken 签发微服务 JWT token
func IssueJWTToken(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
	var req dto.IssueMicroserviceTokenReq
	if err := c.ShouldBindJSON(&req); err != nil {
		msg := c.T("error.bad_request")
		return nil, bizerr.NewValidationError(msg, nil)
	}

	// Get MicroserviceTokenService from DI container
	var tokenService serviceMicroserviceToken.MicroserviceTokenService
	if err := c.App.Find(&tokenService); err != nil {
		return nil, bizerr.ErrInternalServerError(err)
	}

	// Call service layer
	resp, bizErr := tokenService.IssueToken(c, req)
	if bizErr != nil {
		return nil, bizErr
	}

	return resp, nil
}
