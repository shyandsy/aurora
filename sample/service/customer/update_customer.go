package customer

import (
	"fmt"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	commonModel "github.com/shyandsy/aurora/sample/common/model"
	"github.com/shyandsy/aurora/sample/model/dto"
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

	// 更新密码
	if req.Password != nil {
		// 简单验证密码长度（至少6位）
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

	// 更新状态
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
