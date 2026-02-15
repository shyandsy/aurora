package user

import (
	"strconv"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/sample/full_showcase/model/dto"
	serviceUser "github.com/shyandsy/aurora/sample/full_showcase/service/user"
)

// UpdateUser updates a user.
func UpdateUser(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
	// Get user ID from path parameter
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

	var req dto.UpdateUserReq
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
	user, bizErr := userService.UpdateUser(c, id, req)
	if bizErr != nil {
		return nil, bizErr
	}

	return user, nil
}
