package customer

import (
	"fmt"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	commonModel "github.com/shyandsy/aurora/sample/common/model"
	"github.com/shyandsy/aurora/sample/datalayer"
	"github.com/shyandsy/aurora/sample/model/dto"
)

// CustomerService 客户服务接口
type CustomerService interface {
	GetCustomers(ctx *contracts.RequestContext, req dto.GetCustomersReq) (*commonModel.PagingResponse, bizerr.BizError)
	GetCustomer(ctx *contracts.RequestContext, id int64) (*dto.Customer, bizerr.BizError)
	CreateCustomer(ctx *contracts.RequestContext, req dto.CreateCustomerReq) (*dto.Customer, bizerr.BizError)
	UpdateCustomer(ctx *contracts.RequestContext, id int64, req dto.UpdateCustomerReq) (*dto.Customer, bizerr.BizError)
	DeleteCustomer(ctx *contracts.RequestContext, id int64) bizerr.BizError
}

// customerService 客户服务实现
type customerService struct {
	DL datalayer.CustomerDatalayer `inject:""`
}

// NewCustomerService 创建客户服务
func NewCustomerService(app contracts.App) CustomerService {
	s := &customerService{}
	if err := app.Resolve(s); err != nil {
		panic(fmt.Errorf("failed to resolve CustomerService: %w", err))
	}
	return s
}
