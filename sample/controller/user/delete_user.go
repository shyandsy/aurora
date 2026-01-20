package user

import (
	"strconv"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	serviceUser "github.com/shyandsy/aurora/sample/service/user"
)

// DeleteUser 删除用户
func DeleteUser(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
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
	bizErr := userService.DeleteUser(c, id)
	if bizErr != nil {
		return nil, bizErr
	}

	return map[string]string{"message": "User deleted successfully"}, nil
}
