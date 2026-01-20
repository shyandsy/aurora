package role_feature

import (
	"fmt"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/sample/model/dto"
)

func (s *roleFeatureService) GetRoleFeatures(ctx *contracts.RequestContext, roleID int64) ([]dto.RoleFeature, bizerr.BizError) {
	roleFeatures, err := s.DL.GetByRoleID(ctx.Context, roleID)
	if err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}

	items := make([]dto.RoleFeature, 0, len(roleFeatures))
	for _, rf := range roleFeatures {
		items = append(items, *rf.ToDto())
	}

	return items, nil
}
