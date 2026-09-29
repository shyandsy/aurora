package user

import (
	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/types"
	serviceUser "github.com/shyandsy/aurora/modules/user/service/user"
)

// GetUsers 获取用户列表（分页）
func GetUsers(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
	var req types.PagingReq
	if err := c.ShouldBindQuery(&req); err != nil {
		msg := c.T("error.bad_request")
		return nil, bizerr.NewValidationError(msg, nil)
	}

	// 设置默认值
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}

	// Get UserService from DI container
	var userService serviceUser.UserService
	if err := c.App.Find(&userService); err != nil {
		return nil, bizerr.ErrInternalServerError(err)
	}

	// Call service layer
	resp, bizErr := userService.GetUsers(c, req)
	if bizErr != nil {
		return nil, bizErr
	}

	return resp, nil
}
