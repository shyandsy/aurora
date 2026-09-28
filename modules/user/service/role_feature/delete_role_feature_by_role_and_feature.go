package role_feature

import (
	"fmt"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
)

func (s *roleFeatureService) DeleteRoleFeatureByRoleAndFeature(ctx *contracts.RequestContext, roleID, featureID int64) bizerr.BizError {
	if err := s.DL.DeleteByRoleIDAndFeatureID(ctx.Context, roleID, featureID); err != nil {
		return bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}

	// DB 成功后重发布受影响 role 的最新展开 feature。best-effort。
	s.republishRole(ctx.Context, roleID)

	return nil
}
