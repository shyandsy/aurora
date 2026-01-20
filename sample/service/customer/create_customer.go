package customer

import (
	"fmt"
	"net/mail"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	commonModel "github.com/shyandsy/aurora/sample/common/model"
	"github.com/shyandsy/aurora/sample/model/dto"
	"github.com/shyandsy/aurora/sample/model/entity"
	"golang.org/x/crypto/bcrypt"
)

func (s *customerService) CreateCustomer(ctx *contracts.RequestContext, req dto.CreateCustomerReq) (*dto.Customer, bizerr.BizError) {
	// 验证邮箱格式
	if _, err := mail.ParseAddress(req.Email); err != nil {
		msg := ctx.T("user.invalid_email")
		return nil, bizerr.NewValidationError(msg, map[string]string{
			"email": msg,
		})
	}

	// 验证密码长度（简单验证，至少6位）
	if len(req.Password) < 6 {
		msg := ctx.T("user.password_invalid")
		return nil, bizerr.NewValidationError(msg, map[string]string{
			"password": msg,
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

	// 创建客户（简化版，只保留基本字段）
	status := commonModel.GetDefaultStatus()
	customer := &entity.Customer{
		Email:    req.Email,
		Password: string(hashedPassword),
		Status:   status,
	}

	if err := s.DL.Create(ctx.Context, customer); err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}

	return customer.ToDto(), nil
}
