package user

import (
	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	serviceUser "github.com/shyandsy/aurora/sample/service/user"
	commonModel "github.com/shyandsy/aurora/sample/common/model"
)

// GetUsers 获取用户列表（分页）
func GetUsers(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
	var req commonModel.PagingReq
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
