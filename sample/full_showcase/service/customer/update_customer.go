package customer

import (
	"fmt"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	commonModel "github.com/shyandsy/aurora/sample/full_showcase/common/model"
	"github.com/shyandsy/aurora/sample/full_showcase/model/dto"
	"golang.org/x/crypto/bcrypt"
)

func (s *customerService) UpdateCustomer(ctx *contracts.RequestContext, id int64, req dto.UpdateCustomerReq) (*dto.Customer, bizerr.BizError) {
	customer, err := s.DL.GetByID(ctx.Context, id)
	if err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}
	if customer == nil {
		msg := ctx.T("customer.not_found")
		return nil, bizerr.NewValidationError(msg, nil)
	}

	// Update password
	if req.Password != nil {
		// Validate password length (min 6)
		if len(*req.Password) < 6 {
			msg := ctx.T("user.password_invalid")
			return nil, bizerr.NewValidationError(msg, map[string]string{
				"password": msg,
			})
		}

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
		}
		customer.Password = string(hashedPassword)
	}

	// Update status
	if req.Status != nil {
		if !commonModel.IsValidStatus(*req.Status) {
			msg := ctx.T("customer.invalid_status")
			return nil, bizerr.NewValidationError(msg, map[string]string{
				"status": msg,
			})
		}
		customer.Status = *req.Status
	}

	if err := s.DL.Update(ctx.Context, customer); err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}

	return customer.ToDto(), nil
}
