package customer

import (
	"fmt"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
)

func (s *customerService) DeleteCustomer(ctx *contracts.RequestContext, id int64) bizerr.BizError {
	// Check customer exists
	customer, err := s.DL.GetByID(ctx.Context, id)
	if err != nil {
		return bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}
	if customer == nil {
		msg := ctx.T("customer.not_found")
		return bizerr.NewValidationError(msg, nil)
	}

	// Delete customer
	if err := s.DL.Delete(ctx.Context, id); err != nil {
		return bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}

	return nil
}
