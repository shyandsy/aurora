package dto

import "time"

// User 管理员用户信息
type User struct {
	ID       int64     `json:"id"`
	Email    string    `json:"email"`
	RoleID   int64     `json:"roleId"`
	Status   int       `json:"status"`
	Created  time.Time `json:"created"`
	Modified time.Time `json:"modified"`

	// 关联对象
	Role     *Role    `json:"role"`
	Features []string `json:"features"`
}

// CreateUserReq 创建用户请求
type CreateUserReq struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
	RoleID   int64  `json:"roleId" binding:"required"`
}

// UpdateUserReq 更新用户请求
type UpdateUserReq struct {
	Email    string `json:"email" binding:"omitempty,email"`
	Password string `json:"password" binding:"omitempty"`
	RoleID   *int64 `json:"roleId"`
	Status   *int   `json:"status"`
}
