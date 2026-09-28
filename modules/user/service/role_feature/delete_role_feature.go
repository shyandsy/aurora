package role_feature

import (
	"fmt"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
)

func (s *roleFeatureService) DeleteRoleFeature(ctx *contracts.RequestContext, id int64) bizerr.BizError {
	roleFeature, err := s.DL.GetByID(ctx.Context, id)
	if err != nil {
		return bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}
	if roleFeature == nil {
		msg := ctx.T("rolefeature.not_found")
		return bizerr.NewValidationError(msg, nil)
	}

	if err := s.DL.Delete(ctx.Context, id); err != nil {
		return bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}

	// DB 成功后重发布受影响 role 的最新展开 feature。best-effort。
	s.republishRole(ctx.Context, roleFeature.RoleID)

	return nil
}
