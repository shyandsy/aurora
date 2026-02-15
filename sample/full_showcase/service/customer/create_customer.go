package customer

import (
	"fmt"
	"net/mail"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	commonModel "github.com/shyandsy/aurora/sample/full_showcase/common/model"
	"github.com/shyandsy/aurora/sample/full_showcase/model/dto"
	"github.com/shyandsy/aurora/sample/full_showcase/model/entity"
	"golang.org/x/crypto/bcrypt"
)

func (s *customerService) CreateCustomer(ctx *contracts.RequestContext, req dto.CreateCustomerReq) (*dto.Customer, bizerr.BizError) {
	// Validate email format
	if _, err := mail.ParseAddress(req.Email); err != nil {
		msg := ctx.T("user.invalid_email")
		return nil, bizerr.NewValidationError(msg, map[string]string{
			"email": msg,
		})
	}

	// Validate password length (min 6)
	if len(req.Password) < 6 {
		msg := ctx.T("user.password_invalid")
		return nil, bizerr.NewValidationError(msg, map[string]string{
			"password": msg,
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

	// Create customer (simplified)
	status := commonModel.GetDefaultStatus()
	customer := &entity.Customer{
		Email:    req.Email,
		Password: string(hashedPassword),
		Status:   status,
	}

	if err := s.DL.Create(ctx.Context, customer); err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}

	return customer.ToDto(), nil
}
