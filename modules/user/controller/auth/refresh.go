package auth

import (
	"errors"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/logger"
	"github.com/shyandsy/aurora/modules/user/model/dto"
	serviceUser "github.com/shyandsy/aurora/modules/user/service/user"
)

// Refresh 用 refreshToken 换新的 accessToken（无需已登录的 accessToken）
func Refresh(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
	var req dto.RefreshReq
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Errorf("Refresh: failed to bind JSON, error=%+v", err)
		msg := c.T("error.bad_request")
		return nil, bizerr.NewValidationError(msg, nil)
	}

	var userService serviceUser.UserService
	if err := c.App.Find(&userService); err != nil {
		logger.Errorf("Refresh: failed to find UserService, error=%+v", err)
		msg := c.T("error.internal_server")
		return nil, bizerr.ErrInternalServerError(errors.New(msg))
	}

	resp, bizErr := userService.RefreshToken(c, req)
	if bizErr != nil {
		return nil, bizErr
	}

	return resp, nil
}
