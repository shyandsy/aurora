package user

import (
	"fmt"
	"net/mail"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/sample/model/dto"
	commonModel "github.com/shyandsy/aurora/sample/common/model"
	"golang.org/x/crypto/bcrypt"
)

func (s *userService) UpdateUser(ctx *contracts.RequestContext, id int64, req dto.UpdateUserReq) (*dto.User, bizerr.BizError) {
	// 获取用户
	user, err := s.DL.GetByID(ctx.Context, id)
	if err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}
	if user == nil {
		msg := ctx.T("user.not_found")
		return nil, bizerr.NewValidationError(msg, nil)
	}

	hasUpdate := false

	// 更新角色（如果提供）
	if req.RoleID != nil {
		// 验证角色是否存在
		role, err := s.RoleDL.GetByID(ctx.Context, *req.RoleID)
		if err != nil {
			return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
		}
		if role == nil {
			msg := ctx.T("role.not_found")
			return nil, bizerr.NewValidationError(msg, map[string]string{
				"roleId": msg,
			})
		}
		user.RoleID = *req.RoleID
		hasUpdate = true
	}

	// 更新邮箱（如果提供）
	if req.Email != "" {
		// 验证邮箱格式
		if _, err := mail.ParseAddress(req.Email); err != nil {
			msg := ctx.T("user.invalid_email")
			return nil, bizerr.NewValidationError(msg, map[string]string{
				"email": msg,
			})
		}

		// 检查新邮箱是否已被其他用户使用
		existing, err := s.DL.GetByEmail(ctx.Context, req.Email)
		if err != nil {
			return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
		}
		if existing != nil && existing.ID != id {
			msg := ctx.T("user.email_exists")
			return nil, bizerr.NewValidationError(msg, map[string]string{
				"email": msg,
			})
		}

		user.Email = req.Email
		hasUpdate = true
	}

	// 更新状态（如果提供）
	if req.Status != nil {
		// 验证状态值
		if !commonModel.IsValidStatus(*req.Status) {
			msg := ctx.T("error.invalid_status")
			return nil, bizerr.NewValidationError(msg, map[string]string{
				"status": msg,
			})
		}
		user.Status = *req.Status
		hasUpdate = true
	}

	// 更新密码（如果提供）
	if req.Password != "" {
		// 验证密码长度
		if len(req.Password) < 6 || len(req.Password) > 30 {
			msg := ctx.T("user.password_invalid")
			return nil, bizerr.NewValidationError(msg, map[string]string{
				"password": msg,
			})
		}

		// 加密密码
		var err error
		hashedPasswordBytes, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
		}
		user.Password = string(hashedPasswordBytes)
		hasUpdate = true
	}

	// 只更新一次数据库
	if hasUpdate {
		// 统一调用 Update 方法，一次更新所有字段
		if err := s.DL.Update(ctx.Context, user); err != nil {
			return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
		}
	}

	return user.ToDto(), nil
}
