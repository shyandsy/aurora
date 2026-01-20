package dto

import (
	"time"

	commonModel "github.com/shyandsy/aurora/sample/common/model"
)

// Customer 客户信息（简化版）
type Customer struct {
	ID       int64     `json:"id"`
	Email    string    `json:"email"`
	Status   int       `json:"status"`
	Created  time.Time `json:"created"`
	Modified time.Time `json:"modified"`
}

// GetCustomersReq 获取客户列表请求（支持筛选）
type GetCustomersReq struct {
	commonModel.PagingReq        // 内嵌分页请求
	Email                 string `form:"email"`  // 邮箱模糊匹配
	Status                *int   `form:"status"` // 状态筛选
}

// CreateCustomerReq 创建客户请求
type CreateCustomerReq struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// UpdateCustomerReq 更新客户请求
type UpdateCustomerReq struct {
	Password *string `json:"password,omitempty"`
	Status   *int    `json:"status,omitempty"`
}
