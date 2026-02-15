package role_feature

import (
	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/sample/full_showcase/model/dto"
	serviceRoleFeature "github.com/shyandsy/aurora/sample/full_showcase/service/role_feature"
)

// CreateRoleFeature creates a role-feature.
func CreateRoleFeature(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
	var req dto.CreateRoleFeatureReq
	if err := c.ShouldBindJSON(&req); err != nil {
		msg := c.T("error.bad_request")
		return nil, bizerr.NewValidationError(msg, nil)
	}

	var roleFeatureService serviceRoleFeature.RoleFeatureService
	if err := c.App.Find(&roleFeatureService); err != nil {
		return nil, bizerr.ErrInternalServerError(err)
	}

	roleFeature, bizErr := roleFeatureService.CreateRoleFeature(c, req)
	if bizErr != nil {
		return nil, bizErr
	}

	return roleFeature, nil
}
