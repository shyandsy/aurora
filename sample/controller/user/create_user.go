package user

import (
	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/sample/model/dto"
	serviceUser "github.com/shyandsy/aurora/sample/service/user"
)

// CreateUser 创建新用户
func CreateUser(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
	var req dto.CreateUserReq
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
	user, bizErr := userService.CreateUser(c, req)
	if bizErr != nil {
		return nil, bizErr
	}

	return user, nil
}
