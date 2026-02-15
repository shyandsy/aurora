package role_feature

import (
	"strconv"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	serviceRoleFeature "github.com/shyandsy/aurora/sample/full_showcase/service/role_feature"
)

// GetRoleFeature gets role-feature by ID.
func GetRoleFeature(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
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

	var roleFeatureService serviceRoleFeature.RoleFeatureService
	if err := c.App.Find(&roleFeatureService); err != nil {
		return nil, bizerr.ErrInternalServerError(err)
	}

	roleFeature, bizErr := roleFeatureService.GetRoleFeature(c, id)
	if bizErr != nil {
		return nil, bizErr
	}

	return roleFeature, nil
}
