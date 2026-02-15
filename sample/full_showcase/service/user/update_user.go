package user

import (
	"fmt"
	"net/mail"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/sample/full_showcase/model/dto"
	commonModel "github.com/shyandsy/aurora/sample/full_showcase/common/model"
	"golang.org/x/crypto/bcrypt"
)

func (s *userService) UpdateUser(ctx *contracts.RequestContext, id int64, req dto.UpdateUserReq) (*dto.User, bizerr.BizError) {
	// Get user
	user, err := s.DL.GetByID(ctx.Context, id)
	if err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}
	if user == nil {
		msg := ctx.T("user.not_found")
		return nil, bizerr.NewValidationError(msg, nil)
	}

	hasUpdate := false

	// Update role if provided
	if req.RoleID != nil {
		// Check role exists
		role, err := s.RoleDL.GetByID(ctx.Context, *req.RoleID)
		if err != nil {
			return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
		}
		if role == nil {
			msg := ctx.T("role.not_found")
			return nil, bizerr.NewValidationError(msg, map[string]string{
				"roleId": msg,
			})
		}
		user.RoleID = *req.RoleID
		hasUpdate = true
	}

	// Update email if provided
	if req.Email != "" {
		// Validate email format
		if _, err := mail.ParseAddress(req.Email); err != nil {
			msg := ctx.T("user.invalid_email")
			return nil, bizerr.NewValidationError(msg, map[string]string{
				"email": msg,
			})
		}

		// Check email not used by another user
		existing, err := s.DL.GetByEmail(ctx.Context, req.Email)
		if err != nil {
			return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
		}
		if existing != nil && existing.ID != id {
			msg := ctx.T("user.email_exists")
			return nil, bizerr.NewValidationError(msg, map[string]string{
				"email": msg,
			})
		}

		user.Email = req.Email
		hasUpdate = true
	}

	// Update status if provided
	if req.Status != nil {
		// Validate status
		if !commonModel.IsValidStatus(*req.Status) {
			msg := ctx.T("error.invalid_status")
			return nil, bizerr.NewValidationError(msg, map[string]string{
				"status": msg,
			})
		}
		user.Status = *req.Status
		hasUpdate = true
	}

	// Update password if provided
	if req.Password != "" {
		// Validate password length
		if len(req.Password) < 6 || len(req.Password) > 30 {
			msg := ctx.T("user.password_invalid")
			return nil, bizerr.NewValidationError(msg, map[string]string{
				"password": msg,
			})
		}

		// Hash password
		var err error
		hashedPasswordBytes, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
		}
		user.Password = string(hashedPasswordBytes)
		hasUpdate = true
	}

	// Single DB update
	if hasUpdate {
		// Call Update once with all fields
		if err := s.DL.Update(ctx.Context, user); err != nil {
			return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
		}
	}

	return user.ToDto(), nil
}
