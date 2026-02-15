package customer

import (
	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/sample/full_showcase/model/dto"
	serviceCustomer "github.com/shyandsy/aurora/sample/full_showcase/service/customer"
)

// GetCustomers gets customer list (paged, filterable).
func GetCustomers(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
	var req dto.GetCustomersReq
	if err := c.ShouldBindQuery(&req); err != nil {
		msg := c.T("error.bad_request")
		return nil, bizerr.NewValidationError(msg, nil)
	}

	// Set defaults
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}

	// Get CustomerService from DI container
	var customerService serviceCustomer.CustomerService
	if err := c.App.Find(&customerService); err != nil {
		return nil, bizerr.ErrInternalServerError(err)
	}

	// Call service layer
	resp, bizErr := customerService.GetCustomers(c, req)
	if bizErr != nil {
		return nil, bizErr
	}

	return resp, nil
}
