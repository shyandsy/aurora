package role

import (
	"fmt"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/sample/full_showcase/model/dto"
)

func (s *roleService) GetRoles(ctx *contracts.RequestContext) ([]dto.Role, bizerr.BizError) {
	roles, err := s.DL.GetAll(ctx.Context)
	if err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}

	items := make([]dto.Role, 0, len(roles))
	for _, role := range roles {
		items = append(items, *role.ToDto())
	}

	return items, nil
}
