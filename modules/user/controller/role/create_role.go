package role

import (
	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/modules/user/model/dto"
	serviceRole "github.com/shyandsy/aurora/modules/user/service/role"
)

// CreateRole 创建新角色
func CreateRole(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
	var req dto.CreateRoleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		msg := c.T("error.bad_request")
		return nil, bizerr.NewValidationError(msg, nil)
	}

	var roleService serviceRole.RoleService
	if err := c.App.Find(&roleService); err != nil {
		return nil, bizerr.ErrInternalServerError(err)
	}

	role, bizErr := roleService.CreateRole(c, req)
	if bizErr != nil {
		return nil, bizErr
	}

	return role, nil
}
