package role

import (
	"fmt"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/sample/full_showcase/model/dto"
)

func (s *roleService) GetRole(ctx *contracts.RequestContext, id int64) (*dto.Role, bizerr.BizError) {
	role, err := s.DL.GetByID(ctx.Context, id)
	if err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}
	if role == nil {
		msg := ctx.T("role.not_found")
		return nil, bizerr.NewValidationError(msg, nil)
	}

	return role.ToDto(), nil
}

