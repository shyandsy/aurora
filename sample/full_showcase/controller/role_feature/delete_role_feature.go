package role_feature

import (
	"strconv"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	serviceRoleFeature "github.com/shyandsy/aurora/sample/full_showcase/service/role_feature"
)

// DeleteRoleFeature deletes a role-feature.
func DeleteRoleFeature(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
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

	bizErr := roleFeatureService.DeleteRoleFeature(c, id)
	if bizErr != nil {
		return nil, bizErr
	}

	return map[string]string{"message": "Role feature deleted successfully"}, nil
}
