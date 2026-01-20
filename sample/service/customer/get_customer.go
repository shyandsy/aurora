package customer

import (
	"fmt"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/sample/model/dto"
)

func (s *customerService) GetCustomer(ctx *contracts.RequestContext, id int64) (*dto.Customer, bizerr.BizError) {
	customer, err := s.DL.GetByID(ctx.Context, id)
	if err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}
	if customer == nil {
		msg := ctx.T("customer.not_found")
		return nil, bizerr.NewValidationError(msg, nil)
	}

	return customer.ToDto(), nil
}
