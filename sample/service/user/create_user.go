package user

import (
	"fmt"
	"net/mail"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/sample/model/dto"
	"github.com/shyandsy/aurora/sample/model/entity"
	commonModel "github.com/shyandsy/aurora/sample/common/model"
	"golang.org/x/crypto/bcrypt"
)

func (s *userService) CreateUser(ctx *contracts.RequestContext, req dto.CreateUserReq) (*dto.User, bizerr.BizError) {
	// 验证邮箱格式
	if _, err := mail.ParseAddress(req.Email); err != nil {
		msg := ctx.T("user.invalid_email")
		return nil, bizerr.NewValidationError(msg, map[string]string{
			"email": msg,
		})
	}

	// 验证密码长度
	if len(req.Password) < 6 || len(req.Password) > 30 {
		msg := ctx.T("user.password_invalid")
		return nil, bizerr.NewValidationError(msg, map[string]string{
			"password": msg,
		})
	}

	// 验证角色是否存在
	role, err := s.RoleDL.GetByID(ctx.Context, req.RoleID)
	if err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}
	if role == nil {
		msg := ctx.T("role.not_found")
		return nil, bizerr.NewValidationError(msg, map[string]string{
			"roleId": msg,
		})
	}

	// 检查邮箱是否已存在
	existing, err := s.DL.GetByEmail(ctx.Context, req.Email)
	if err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}
	if existing != nil {
		msg := ctx.T("user.email_exists")
		return nil, bizerr.NewValidationError(msg, map[string]string{
			"email": msg,
		})
	}

	// 加密密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}

	// 创建用户，使用默认状态（启用）
	user := &entity.User{
		Email:    req.Email,
		Password: string(hashedPassword),
		RoleID:   req.RoleID,
		Status:   commonModel.GetDefaultStatus(),
	}

	if err := s.DL.Create(ctx.Context, user); err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}

	return user.ToDto(), nil
}
