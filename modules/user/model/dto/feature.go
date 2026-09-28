package dto

import "time"

// Feature 功能信息
type Feature struct {
	ID       int64     `json:"id"`
	Name     string    `json:"name"`
	Module   string    `json:"module"`
	Action   string    `json:"action"`
	Kind     string    `json:"kind"`
	Created  time.Time `json:"created"`
	Modified time.Time `json:"modified"`
}

// CreateFeatureReq 创建功能请求
type CreateFeatureReq struct {
	Name string `json:"name" binding:"required"`
}

// UpdateFeatureReq 更新功能请求
type UpdateFeatureReq struct {
	Name string `json:"name" binding:"required"`
}
