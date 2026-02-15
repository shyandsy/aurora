package dto

import "time"

// Role is the role DTO.
type Role struct {
	ID       int64     `json:"id"`
	Name     string    `json:"name"`
	Created  time.Time `json:"created"`
	Modified time.Time `json:"modified"`
}

// CreateRoleReq is the create-role request.
type CreateRoleReq struct {
	Name string `json:"name" binding:"required"`
}

// UpdateRoleReq is the update-role request.
type UpdateRoleReq struct {
	Name string `json:"name" binding:"required"`
}
