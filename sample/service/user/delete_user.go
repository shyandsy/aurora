package user

import (
	"fmt"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/sample/common/middleware"
)

func (s *userService) DeleteUser(ctx *contracts.RequestContext, id int64) bizerr.BizError {
	// 获取当前用户ID
	currentUserID, exists := middleware.GetUserID(ctx.Context)
	if !exists {
		msg := ctx.T("auth.unauthorized")
		return bizerr.NewValidationError(msg, nil)
	}

	// 不允许删除自己
	if id == currentUserID {
		msg := ctx.T("user.cannot_delete_self")
		if msg == "user.cannot_delete_self" {
			// 如果翻译不存在，使用默认消息
			msg = "Cannot delete yourself"
		}
		return bizerr.NewValidationError(msg, nil)
	}

	// 检查用户是否存在
	user, err := s.DL.GetByID(ctx.Context, id)
	if err != nil {
		return bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}
	if user == nil {
		msg := ctx.T("user.not_found")
		return bizerr.NewValidationError(msg, nil)
	}

	// 删除用户
	if err := s.DL.Delete(ctx.Context, id); err != nil {
		return bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}

	return nil
}
