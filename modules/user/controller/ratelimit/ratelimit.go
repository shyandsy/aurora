// Package ratelimit 是 user 服务「被锁登录管理」的 HTTP 控制器:列出被锁 IP/账号、解锁。
package ratelimit

import (
	"strings"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"

	"github.com/shyandsy/aurora/modules/user/model/dto"
	serviceRateLimit "github.com/shyandsy/aurora/modules/user/service/ratelimit"
)

// ListLocked GET /rate-limit/locked —— 当前被锁的 IP/账号列表(带最近邮箱、原因、解除剩余秒数)。
func ListLocked(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
	var svc serviceRateLimit.LockAdminService
	if err := c.App.Find(&svc); err != nil {
		return nil, bizerr.ErrInternalServerError(err)
	}
	return svc.ListLocked(c)
}

// Unlock POST /rate-limit/locked/unlock —— 按 scope+key 强制解锁,立即恢复。
func Unlock(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
	var req dto.UnlockReq
	if err := c.ShouldBindJSON(&req); err != nil {
		return nil, bizerr.NewValidationError(c.T("error.bad_request"), nil)
	}
	req.Scope = strings.TrimSpace(req.Scope)
	req.Key = strings.TrimSpace(req.Key)
	if req.Key == "" {
		return nil, bizerr.NewValidationError(c.T("error.bad_request"), nil)
	}

	var svc serviceRateLimit.LockAdminService
	if err := c.App.Find(&svc); err != nil {
		return nil, bizerr.ErrInternalServerError(err)
	}
	if bizErr := svc.Unlock(c, req.Scope, req.Key); bizErr != nil {
		return nil, bizErr
	}
	return map[string]interface{}{"success": true}, nil
}
