package role_feature

import (
	"strconv"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	serviceRoleFeature "github.com/shyandsy/aurora/sample/full_showcase/service/role_feature"
)

// GetRoleFeatures gets role-features by role ID.
func GetRoleFeatures(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
	roleIDStr := c.Query("roleId")
	if roleIDStr == "" {
		msg := c.T("error.bad_request")
		return nil, bizerr.NewValidationError(msg, nil)
	}

	roleID, err := strconv.ParseInt(roleIDStr, 10, 64)
	if err != nil || roleID <= 0 {
		msg := c.T("error.bad_request")
		return nil, bizerr.NewValidationError(msg, nil)
	}

	var roleFeatureService serviceRoleFeature.RoleFeatureService
	if err := c.App.Find(&roleFeatureService); err != nil {
		return nil, bizerr.ErrInternalServerError(err)
	}

	roleFeatures, bizErr := roleFeatureService.GetRoleFeatures(c, roleID)
	if bizErr != nil {
		return nil, bizErr
	}

	return roleFeatures, nil
}
