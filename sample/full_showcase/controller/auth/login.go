package auth

import (
	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/sample/full_showcase/model/dto"
	serviceUser "github.com/shyandsy/aurora/sample/full_showcase/service/user"
)

// Login is admin login.
func Login(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
	var req dto.LoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		msg := c.T("error.bad_request")
		return nil, bizerr.NewValidationError(msg, nil)
	}

	// Get UserService from DI container
	var userService serviceUser.UserService
	if err := c.App.Find(&userService); err != nil {
		return nil, bizerr.ErrInternalServerError(err)
	}

	// Call service layer
	resp, bizErr := userService.Login(c, req)
	if bizErr != nil {
		return nil, bizErr
	}

	return resp, nil
}
