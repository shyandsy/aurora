package customer

import (
	"fmt"
	"math"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	commonModel "github.com/shyandsy/aurora/sample/full_showcase/common/model"
	"github.com/shyandsy/aurora/sample/full_showcase/model/dto"
)

func (s *customerService) GetCustomers(ctx *contracts.RequestContext, req dto.GetCustomersReq) (*commonModel.PagingResponse, bizerr.BizError) {
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

	// Get customer list (filterable)
	customers, total, err := s.DL.GetAll(ctx.Context, req)
	if err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}

	// Convert to DTO
	items := make([]dto.Customer, 0, len(customers))
	for _, customer := range customers {
		items = append(items, *customer.ToDto())
	}

	// Calc total pages
	totalPages := int32(math.Ceil(float64(total) / float64(req.PageSize)))

	// Build response
	response := &commonModel.PagingResponse{
		Page:       req.Page,
		PageSize:   req.PageSize,
		Total:      total,
		TotalPages: totalPages,
		HasNext:    req.Page < totalPages,
		HasPrev:    req.Page > 1,
		Items:      items,
	}

	return response, nil
}
