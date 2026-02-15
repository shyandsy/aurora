package role_feature

import (
	"fmt"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/sample/full_showcase/model/dto"
	"github.com/shyandsy/aurora/sample/full_showcase/model/entity"
)

func (s *roleFeatureService) CreateRoleFeature(ctx *contracts.RequestContext, req dto.CreateRoleFeatureReq) (*dto.RoleFeature, bizerr.BizError) {
	// Check if role exists
	role, err := s.RoleDL.GetByID(ctx.Context, req.RoleID)
	if err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}
	if role == nil {
		msg := ctx.T("role.not_found")
		return nil, bizerr.NewValidationError(msg, nil)
	}

	// Check if feature exists
	feature, err := s.FeatureDL.GetByID(ctx.Context, req.FeatureID)
	if err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}
	if feature == nil {
		msg := ctx.T("feature.not_found")
		return nil, bizerr.NewValidationError(msg, nil)
	}

	// Check if role-feature already exists
	existing, err := s.DL.GetByRoleIDAndFeatureID(ctx.Context, req.RoleID, req.FeatureID)
	if err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}
	if existing != nil {
		msg := ctx.T("rolefeature.exists")
		return nil, bizerr.NewValidationError(msg, nil)
	}

	roleFeature := &entity.RoleFeature{
		RoleID:    req.RoleID,
		FeatureID: req.FeatureID,
	}

	if err := s.DL.Create(ctx.Context, roleFeature); err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}

	return roleFeature.ToDto(), nil
}
