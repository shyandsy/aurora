package dto

import "time"

// Role 角色信息
type Role struct {
	ID       int64     `json:"id"`
	Name     string    `json:"name"`
	Created  time.Time `json:"created"`
	Modified time.Time `json:"modified"`
}

// CreateRoleReq 创建角色请求
type CreateRoleReq struct {
	Name string `json:"name" binding:"required"`
}

// UpdateRoleReq 更新角色请求
type UpdateRoleReq struct {
	Name string `json:"name" binding:"required"`
}
