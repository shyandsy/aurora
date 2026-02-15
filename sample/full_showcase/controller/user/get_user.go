package user

import (
	"strconv"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	serviceUser "github.com/shyandsy/aurora/sample/full_showcase/service/user"
)

// GetUser gets user by ID.
func GetUser(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
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

	// Get UserService from DI container
	var userService serviceUser.UserService
	if err := c.App.Find(&userService); err != nil {
		return nil, bizerr.ErrInternalServerError(err)
	}

	// Call service layer
	user, bizErr := userService.GetUser(c, id)
	if bizErr != nil {
		return nil, bizErr
	}

	return user, nil
}
