package dto

import "time"

// RoleFeature 角色功能关联信息
type RoleFeature struct {
	ID        int64     `json:"id"`
	RoleID    int64     `json:"roleId"`
	FeatureID int64     `json:"featureId"`
	Created   time.Time `json:"created"`
	Modified  time.Time `json:"modified"`
}

// CreateRoleFeatureReq 创建角色功能关联请求
type CreateRoleFeatureReq struct {
	RoleID    int64 `json:"roleId" binding:"required"`
	FeatureID int64 `json:"featureId" binding:"required"`
}
