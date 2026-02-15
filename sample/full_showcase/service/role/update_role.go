package role

import (
	"fmt"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/sample/full_showcase/model/dto"
)

func (s *roleService) UpdateRole(ctx *contracts.RequestContext, id int64, req dto.UpdateRoleReq) (*dto.Role, bizerr.BizError) {
	role, err := s.DL.GetByID(ctx.Context, id)
	if err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}
	if role == nil {
		msg := ctx.T("role.not_found")
		return nil, bizerr.NewValidationError(msg, nil)
	}

	// Check if new name already exists
	if req.Name != role.Name {
		existing, err := s.DL.GetByName(ctx.Context, req.Name)
		if err != nil {
			return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
		}
		if existing != nil {
			msg := ctx.T("role.name_exists")
			return nil, bizerr.NewValidationError(msg, map[string]string{
				"name": msg,
			})
		}
	}

	role.Name = req.Name

	if err := s.DL.Update(ctx.Context, role); err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}

	return role.ToDto(), nil
}
