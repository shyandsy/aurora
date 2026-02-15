package role_feature

import (
	"fmt"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/sample/full_showcase/datalayer"
	"github.com/shyandsy/aurora/sample/full_showcase/model/dto"
)

// RoleFeatureService is the role-feature service interface.
type RoleFeatureService interface {
	GetRoleFeatures(ctx *contracts.RequestContext, roleID int64) ([]dto.RoleFeature, bizerr.BizError)
	GetRoleFeature(ctx *contracts.RequestContext, id int64) (*dto.RoleFeature, bizerr.BizError)
	CreateRoleFeature(ctx *contracts.RequestContext, req dto.CreateRoleFeatureReq) (*dto.RoleFeature, bizerr.BizError)
	DeleteRoleFeature(ctx *contracts.RequestContext, id int64) bizerr.BizError
}

// roleFeatureService is the role-feature service implementation.
type roleFeatureService struct {
	DL        datalayer.RoleFeatureDatalayer `inject:""`
	RoleDL    datalayer.RoleDatalayer        `inject:""`
	FeatureDL datalayer.FeatureDatalayer     `inject:""`
}

// NewRoleFeatureService creates the role-feature service.
func NewRoleFeatureService(app contracts.App) RoleFeatureService {
	s := &roleFeatureService{}
	if err := app.Resolve(s); err != nil {
		panic(fmt.Errorf("failed to resolve RoleFeatureService: %w", err))
	}
	return s
}
