package role

import (
	"fmt"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/modules/user/model/dto"
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

	oldName := role.Name
	role.Name = req.Name

	if err := s.DL.Update(ctx.Context, role); err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}

	// DB 成功后同步读模型:改名则读模型键随角色名走,先删旧名键再发布新名键(避免旧名残留可解析)。best-effort。
	if oldName != role.Name {
		s.removeRoleReadModel(ctx.Context, oldName)
	}
	s.publishRoleReadModel(ctx.Context, role.ID, role.Name)

	return role.ToDto(), nil
}
