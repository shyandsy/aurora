package user

import (
	"fmt"
	"net/mail"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/sample/full_showcase/model/dto"
	commonModel "github.com/shyandsy/aurora/sample/full_showcase/common/model"
	"golang.org/x/crypto/bcrypt"
)

func (s *userService) Login(ctx *contracts.RequestContext, req dto.LoginReq) (*dto.LoginResp, bizerr.BizError) {
	if req.Email == "" {
		msg := ctx.T("user.email_required")
		return nil, bizerr.NewValidationError(msg, map[string]string{
			"email": msg,
		})
	}
	if _, err := mail.ParseAddress(req.Email); err != nil {
		msg := ctx.T("user.invalid_email")
		return nil, bizerr.NewValidationError(msg, map[string]string{
			"email": msg,
		})
	}

	if req.Password == "" {
		msg := ctx.T("user.password_required")
		return nil, bizerr.NewValidationError(msg, map[string]string{
			"password": msg,
		})
	}

	// Get user by email (with Role preloaded)
	user, err := s.DL.GetByEmail(ctx.Context, req.Email)
	if err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}
	if user == nil {
		msg := ctx.T("auth.login_failed")
		return nil, bizerr.NewValidationError(msg, map[string]string{
			"email": msg,
		})
	}

	// Check user status
	if user.Status != commonModel.StatusEnable {
		msg := ctx.T("user.account_disabled")
		return nil, bizerr.NewValidationError(msg, map[string]string{
			"email": msg,
		})
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		msg := ctx.T("auth.login_failed")
		return nil, bizerr.NewValidationError(msg, map[string]string{
			"password": msg,
		})
	}

	// Get user features by role_id
	features, err := s.FeatureDL.GetByRoleID(ctx.Context, user.RoleID)
	if err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}

	// Convert features to string array
	featureNames := make([]string, 0, len(features))
	for _, feature := range features {
		featureNames = append(featureNames, feature.Name)
	}

	// Generate JWT token with features
	tokenResp, err := s.JWT.GenerateToken(user.ID, user.Email, featureNames)
	if err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}

	// Convert user entity to DTO with Role and Features
	userDto := user.ToDto()
	userDto.Features = featureNames

	// Build login response
	resp := &dto.LoginResp{
		AccessToken:     tokenResp.AccessToken,
		TokenType:       "bearer",
		ExpiresInSecond: tokenResp.ExpiresIn,
		RefreshToken:    tokenResp.RefreshToken,
		Features:        featureNames,
		User:            userDto,
	}

	return resp, nil
}
