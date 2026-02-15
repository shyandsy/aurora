package customer

import (
	"fmt"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	commonModel "github.com/shyandsy/aurora/sample/full_showcase/common/model"
	"github.com/shyandsy/aurora/sample/full_showcase/datalayer"
	"github.com/shyandsy/aurora/sample/full_showcase/model/dto"
)

// CustomerService is the customer service interface.
type CustomerService interface {
	GetCustomers(ctx *contracts.RequestContext, req dto.GetCustomersReq) (*commonModel.PagingResponse, bizerr.BizError)
	GetCustomer(ctx *contracts.RequestContext, id int64) (*dto.Customer, bizerr.BizError)
	CreateCustomer(ctx *contracts.RequestContext, req dto.CreateCustomerReq) (*dto.Customer, bizerr.BizError)
	UpdateCustomer(ctx *contracts.RequestContext, id int64, req dto.UpdateCustomerReq) (*dto.Customer, bizerr.BizError)
	DeleteCustomer(ctx *contracts.RequestContext, id int64) bizerr.BizError
}

// customerService is the customer service implementation.
type customerService struct {
	DL datalayer.CustomerDatalayer `inject:""`
}

// NewCustomerService creates the customer service.
func NewCustomerService(app contracts.App) CustomerService {
	s := &customerService{}
	if err := app.Resolve(s); err != nil {
		panic(fmt.Errorf("failed to resolve CustomerService: %w", err))
	}
	return s
}
