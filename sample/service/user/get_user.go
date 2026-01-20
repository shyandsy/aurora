package user

import (
	"fmt"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/sample/model/dto"
)

func (s *userService) GetUser(ctx *contracts.RequestContext, id int64) (*dto.User, bizerr.BizError) {
	user, err := s.DL.GetByID(ctx.Context, id)
	if err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}
	if user == nil {
		msg := ctx.T("user.not_found")
		return nil, bizerr.NewValidationError(msg, nil)
	}

	// 转换为 DTO
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

	return userDto, nil
}
