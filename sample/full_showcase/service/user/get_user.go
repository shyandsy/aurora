package user

import (
	"fmt"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/sample/full_showcase/model/dto"
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

	// Convert to DTO
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

	return userDto, nil
}
