package role_feature

import (
	"fmt"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/sample/model/dto"
)

func (s *roleFeatureService) GetRoleFeature(ctx *contracts.RequestContext, id int64) (*dto.RoleFeature, bizerr.BizError) {
	roleFeature, err := s.DL.GetByID(ctx.Context, id)
	if err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}
	if roleFeature == nil {
		msg := ctx.T("rolefeature.not_found")
		return nil, bizerr.NewValidationError(msg, nil)
	}

	return roleFeature.ToDto(), nil
}
