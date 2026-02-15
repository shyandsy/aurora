package role

import (
	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	serviceRole "github.com/shyandsy/aurora/sample/full_showcase/service/role"
)

// GetRoles gets role list.
func GetRoles(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
	var roleService serviceRole.RoleService
	if err := c.App.Find(&roleService); err != nil {
		return nil, bizerr.ErrInternalServerError(err)
	}

	roles, bizErr := roleService.GetRoles(c)
	if bizErr != nil {
		return nil, bizErr
	}

	return roles, nil
}
