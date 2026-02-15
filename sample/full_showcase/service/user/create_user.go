package user

import (
	"fmt"
	"net/mail"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/sample/full_showcase/model/dto"
	"github.com/shyandsy/aurora/sample/full_showcase/model/entity"
	commonModel "github.com/shyandsy/aurora/sample/full_showcase/common/model"
	"golang.org/x/crypto/bcrypt"
)

func (s *userService) CreateUser(ctx *contracts.RequestContext, req dto.CreateUserReq) (*dto.User, bizerr.BizError) {
	// Validate email format
	if _, err := mail.ParseAddress(req.Email); err != nil {
		msg := ctx.T("user.invalid_email")
		return nil, bizerr.NewValidationError(msg, map[string]string{
			"email": msg,
		})
	}

	// Validate password length
	if len(req.Password) < 6 || len(req.Password) > 30 {
		msg := ctx.T("user.password_invalid")
		return nil, bizerr.NewValidationError(msg, map[string]string{
			"password": msg,
		})
	}

	// Check role exists
	role, err := s.RoleDL.GetByID(ctx.Context, req.RoleID)
	if err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}
	if role == nil {
		msg := ctx.T("role.not_found")
		return nil, bizerr.NewValidationError(msg, map[string]string{
			"roleId": msg,
		})
	}

	// Check email already exists
	existing, err := s.DL.GetByEmail(ctx.Context, req.Email)
	if err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}
	if existing != nil {
		msg := ctx.T("user.email_exists")
		return nil, bizerr.NewValidationError(msg, map[string]string{
			"email": msg,
		})
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}

	// Create user with default status (enabled)
	user := &entity.User{
		Email:    req.Email,
		Password: string(hashedPassword),
		RoleID:   req.RoleID,
		Status:   commonModel.GetDefaultStatus(),
	}

	if err := s.DL.Create(ctx.Context, user); err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}

	return user.ToDto(), nil
}
