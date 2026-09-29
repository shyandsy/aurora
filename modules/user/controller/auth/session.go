package auth

import (
	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	serviceUser "github.com/shyandsy/aurora/modules/user/service/user"
	"github.com/gin-gonic/gin"
)

func sessionService(c *contracts.RequestContext) (serviceUser.UserService, bizerr.BizError) {
	var svc serviceUser.UserService
	if err := c.App.Find(&svc); err != nil {
		return nil, bizerr.ErrInternalServerError(err)
	}
	return svc, nil
}

// ListSessions GET /auth/sessions —— 当前用户的活跃登录会话/设备清单(标出当前这台)。
func ListSessions(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
	svc, be := sessionService(c)
	if be != nil {
		return nil, be
	}
	return svc.ListSessions(c)
}

// RevokeSession POST /auth/sessions/:sid/revoke —— 撤销指定会话(仅限本人名下),即时失效。
func RevokeSession(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
	svc, be := sessionService(c)
	if be != nil {
		return nil, be
	}
	if be := svc.RevokeSession(c, c.Param("sid")); be != nil {
		return nil, be
	}
	return gin.H{"message": c.T("auth.session_revoked")}, nil
}

// RevokeOtherSessions POST /auth/sessions/revoke-others —— 撤销除当前这台外的所有会话("登出其他所有设备")。
func RevokeOtherSessions(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
	svc, be := sessionService(c)
	if be != nil {
		return nil, be
	}
	if be := svc.RevokeOtherSessions(c); be != nil {
		return nil, be
	}
	return gin.H{"message": c.T("auth.session_revoked")}, nil
}
