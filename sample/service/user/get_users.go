package user

import (
	"fmt"
	"math"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/sample/model/dto"
	commonModel "github.com/shyandsy/aurora/sample/common/model"
)

func (s *userService) GetUsers(ctx *contracts.RequestContext, req commonModel.PagingReq) (*commonModel.PagingResponse, bizerr.BizError) {
	offset := (req.Page - 1) * req.PageSize
	limit := req.PageSize

	users, total, err := s.DL.GetAll(ctx.Context, int(offset), int(limit))
	if err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}

	// 转换为 DTO，并填充 features
	items := make([]dto.User, 0, len(users))
	for _, user := range users {
		userDto := user.ToDto()

		// 获取用户的 features
		if user.RoleID > 0 {
			features, err := s.FeatureDL.GetByRoleID(ctx.Context, user.RoleID)
			if err != nil {
				return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
			}
			// 转换为 feature name 列表
			featureNames := make([]string, 0, len(features))
			for _, feature := range features {
				featureNames = append(featureNames, feature.Name)
			}
			userDto.Features = featureNames
		}

		items = append(items, *userDto)
	}

	totalPages := int32(math.Ceil(float64(total) / float64(req.PageSize)))

	resp := &commonModel.PagingResponse{
		Page:       req.Page,
		PageSize:   req.PageSize,
		Total:      total,
		TotalPages: totalPages,
		HasNext:    req.Page < totalPages,
		HasPrev:    req.Page > 1,
		Items:      items,
	}

	return resp, nil
}
