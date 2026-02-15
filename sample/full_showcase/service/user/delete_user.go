package user

import (
	"fmt"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/sample/full_showcase/common/middleware"
)

func (s *userService) DeleteUser(ctx *contracts.RequestContext, id int64) bizerr.BizError {
	// Get current user ID
	currentUserID, exists := middleware.GetUserID(ctx.Context)
	if !exists {
		msg := ctx.T("auth.unauthorized")
		return bizerr.NewValidationError(msg, nil)
	}

	// Cannot delete self
	if id == currentUserID {
		msg := ctx.T("user.cannot_delete_self")
		if msg == "user.cannot_delete_self" {
			// Fallback if translation missing
			msg = "Cannot delete yourself"
		}
		return bizerr.NewValidationError(msg, nil)
	}

	// Check user exists
	user, err := s.DL.GetByID(ctx.Context, id)
	if err != nil {
		return bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}
	if user == nil {
		msg := ctx.T("user.not_found")
		return bizerr.NewValidationError(msg, nil)
	}

	// Delete user
	if err := s.DL.Delete(ctx.Context, id); err != nil {
		return bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}

	return nil
}
