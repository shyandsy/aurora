package user

import (
	"fmt"
	"math"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/sample/full_showcase/model/dto"
	commonModel "github.com/shyandsy/aurora/sample/full_showcase/common/model"
)

func (s *userService) GetUsers(ctx *contracts.RequestContext, req commonModel.PagingReq) (*commonModel.PagingResponse, bizerr.BizError) {
	offset := (req.Page - 1) * req.PageSize
	limit := req.PageSize

	users, total, err := s.DL.GetAll(ctx.Context, int(offset), int(limit))
	if err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}

	// Convert to DTO and fill features
	items := make([]dto.User, 0, len(users))
	for _, user := range users {
		userDto := user.ToDto()

		// Get user features
		if user.RoleID > 0 {
			features, err := s.FeatureDL.GetByRoleID(ctx.Context, user.RoleID)
			if err != nil {
				return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
			}
			// Convert to feature name list
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
