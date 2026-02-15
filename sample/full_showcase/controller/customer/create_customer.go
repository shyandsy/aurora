package customer

import (
	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/sample/full_showcase/model/dto"
	serviceCustomer "github.com/shyandsy/aurora/sample/full_showcase/service/customer"
)

// CreateCustomer creates a customer.
func CreateCustomer(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
	var req dto.CreateCustomerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		msg := c.T("error.bad_request")
		return nil, bizerr.NewValidationError(msg, nil)
	}

	// Get CustomerService from DI container
	var customerService serviceCustomer.CustomerService
	if err := c.App.Find(&customerService); err != nil {
		return nil, bizerr.ErrInternalServerError(err)
	}

	// Call service layer
	customer, bizErr := customerService.CreateCustomer(c, req)
	if bizErr != nil {
		return nil, bizErr
	}

	return customer, nil
}
