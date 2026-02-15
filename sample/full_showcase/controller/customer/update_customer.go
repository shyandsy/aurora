package customer

import (
	"strconv"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/sample/full_showcase/model/dto"
	serviceCustomer "github.com/shyandsy/aurora/sample/full_showcase/service/customer"
)

// UpdateCustomer updates a customer.
func UpdateCustomer(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
	// Get customer ID from path parameter
	idStr := c.Param("id")
	if idStr == "" {
		msg := c.T("error.bad_request")
		return nil, bizerr.NewValidationError(msg, nil)
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		msg := c.T("error.bad_request")
		return nil, bizerr.NewValidationError(msg, nil)
	}

	var req dto.UpdateCustomerReq
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
	customer, bizErr := customerService.UpdateCustomer(c, id, req)
	if bizErr != nil {
		return nil, bizErr
	}

	return customer, nil
}
