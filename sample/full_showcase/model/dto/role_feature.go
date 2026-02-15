package dto

import "time"

// RoleFeature is the role-feature association DTO.
type RoleFeature struct {
	ID        int64     `json:"id"`
	RoleID    int64     `json:"roleId"`
	FeatureID int64     `json:"featureId"`
	Created   time.Time `json:"created"`
	Modified  time.Time `json:"modified"`
}

// CreateRoleFeatureReq is the create-role-feature request.
type CreateRoleFeatureReq struct {
	RoleID    int64 `json:"roleId" binding:"required"`
	FeatureID int64 `json:"featureId" binding:"required"`
}
