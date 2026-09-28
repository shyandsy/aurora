package role

import (
	"fmt"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
)

func (s *roleService) DeleteRole(ctx *contracts.RequestContext, id int64) bizerr.BizError {
	role, err := s.DL.GetByID(ctx.Context, id)
	if err != nil {
		return bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}
	if role == nil {
		msg := ctx.T("role.not_found")
		return bizerr.NewValidationError(msg, nil)
	}

	if err := s.DL.Delete(ctx.Context, id); err != nil {
		return bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}

	// DB 删除成功后移除读模型键(role 已不存在 → 其 token 解析应 miss→fail-close)。best-effort。
	s.removeRoleReadModel(ctx.Context, role.Name)

	return nil
}
