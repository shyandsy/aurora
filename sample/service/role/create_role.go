package role

import (
	"fmt"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/sample/model/dto"
	"github.com/shyandsy/aurora/sample/model/entity"
)

func (s *roleService) CreateRole(ctx *contracts.RequestContext, req dto.CreateRoleReq) (*dto.Role, bizerr.BizError) {
	// Check if role name already exists
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

	role := &entity.Role{
		Name: req.Name,
	}

	if err := s.DL.Create(ctx.Context, role); err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}

	return role.ToDto(), nil
}
