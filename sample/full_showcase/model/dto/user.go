package dto

import "time"

// User is the admin user DTO.
type User struct {
	ID       int64     `json:"id"`
	Email    string    `json:"email"`
	RoleID   int64     `json:"roleId"`
	Status   int       `json:"status"`
	Created  time.Time `json:"created"`
	Modified time.Time `json:"modified"`

	Role     *Role    `json:"role"`
	Features []string `json:"features"`
}

// CreateUserReq is the create-user request.
type CreateUserReq struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
	RoleID   int64  `json:"roleId" binding:"required"`
}

// UpdateUserReq is the update-user request.
type UpdateUserReq struct {
	Email    string `json:"email" binding:"omitempty,email"`
	Password string `json:"password" binding:"omitempty"`
	RoleID   *int64 `json:"roleId"`
	Status   *int   `json:"status"`
}
