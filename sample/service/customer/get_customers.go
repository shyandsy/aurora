package customer

import (
	"fmt"
	"math"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	commonModel "github.com/shyandsy/aurora/sample/common/model"
	"github.com/shyandsy/aurora/sample/model/dto"
)

func (s *customerService) GetCustomers(ctx *contracts.RequestContext, req dto.GetCustomersReq) (*commonModel.PagingResponse, bizerr.BizError) {
	// 设置默认值
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}

	// 获取客户列表（支持筛选）
	customers, total, err := s.DL.GetAll(ctx.Context, req)
	if err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}

	// 转换为 DTO
	items := make([]dto.Customer, 0, len(customers))
	for _, customer := range customers {
		items = append(items, *customer.ToDto())
	}

	// 计算总页数
	totalPages := int32(math.Ceil(float64(total) / float64(req.PageSize)))

	// 构建响应
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
