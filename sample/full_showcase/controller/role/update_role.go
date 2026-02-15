package role

import (
	"strconv"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/sample/full_showcase/model/dto"
	serviceRole "github.com/shyandsy/aurora/sample/full_showcase/service/role"
)

// UpdateRole updates a role.
func UpdateRole(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
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

	var req dto.UpdateRoleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		msg := c.T("error.bad_request")
		return nil, bizerr.NewValidationError(msg, nil)
	}

	var roleService serviceRole.RoleService
	if err := c.App.Find(&roleService); err != nil {
		return nil, bizerr.ErrInternalServerError(err)
	}

	role, bizErr := roleService.UpdateRole(c, id, req)
	if bizErr != nil {
		return nil, bizErr
	}

	return role, nil
}
