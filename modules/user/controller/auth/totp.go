package auth

import (
	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/modules/user/model/dto"
	serviceUser "github.com/shyandsy/aurora/modules/user/service/user"
)

func userSvc(c *contracts.RequestContext) (serviceUser.UserService, bizerr.BizError) {
	var svc serviceUser.UserService
	if err := c.App.Find(&svc); err != nil {
		return nil, bizerr.ErrInternalServerError(err)
	}
	return svc, nil
}

// SetupTotp 开始绑定两步验证(需已登录 + 重验密码),返回二维码 URI + 密钥。
func SetupTotp(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
	var req dto.TotpSetupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		return nil, bizerr.NewValidationError(c.T("error.bad_request"), nil)
	}
	svc, be := userSvc(c)
	if be != nil {
		return nil, be
	}
	return svc.SetupTotp(c, req)
}

// ConfirmTotp 确认绑定(输 app 上的码),通过后一次性返回备用码。
func ConfirmTotp(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
	var req dto.TotpConfirmReq
	if err := c.ShouldBindJSON(&req); err != nil {
		return nil, bizerr.NewValidationError(c.T("error.bad_request"), nil)
	}
	svc, be := userSvc(c)
	if be != nil {
		return nil, be
	}
	return svc.ConfirmTotp(c, req)
}

// DisableTotp 关闭两步验证(需密码 + 当前码)。
func DisableTotp(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
	var req dto.TotpDisableReq
	if err := c.ShouldBindJSON(&req); err != nil {
		return nil, bizerr.NewValidationError(c.T("error.bad_request"), nil)
	}
	svc, be := userSvc(c)
	if be != nil {
		return nil, be
	}
	if be := svc.DisableTotp(c, req); be != nil {
		return nil, be
	}
	return map[string]bool{"success": true}, nil
}

// LoginTwoFactor 登录第二步:pendingToken + 验证码(或备用码)换正式 token。
func LoginTwoFactor(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
	var req dto.Login2FAReq
	if err := c.ShouldBindJSON(&req); err != nil {
		return nil, bizerr.NewValidationError(c.T("error.bad_request"), nil)
	}
	svc, be := userSvc(c)
	if be != nil {
		return nil, be
	}
	return svc.LoginTwoFactor(c, req)
}
