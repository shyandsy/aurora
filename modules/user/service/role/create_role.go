package role

import (
	"fmt"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/modules/user/model/dto"
	"github.com/shyandsy/aurora/modules/user/model/entity"
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

	// DB 成功后写穿读模型(新 role 尚无 feature,展开即空集,建键以区别于键缺失)。best-effort。
	s.publishRoleReadModel(ctx.Context, role.ID, role.Name)

	return role.ToDto(), nil
}
