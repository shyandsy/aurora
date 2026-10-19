package dto

import (
	"time"

	"github.com/shyandsy/aurora/types"
)

// Feature 功能信息
type Feature struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Module string `json:"module"`
	Action string `json:"action"`
	Kind   string `json:"kind"`
	// LabelI18n 多语言显示名(可授权页目录用),JSON 对象 {语言:文案};无则省略。
	// 语言集/默认/回退不写死后端,由前端按 i18n 当前语言选取。SortOrder 页目录顺序。
	LabelI18n types.LocalizedText `json:"labelI18n,omitempty"`
	SortOrder int                 `json:"sortOrder"`
	Created   time.Time           `json:"created"`
	Modified  time.Time           `json:"modified"`
}

// CreateFeatureReq 创建功能请求
type CreateFeatureReq struct {
	Name string `json:"name" binding:"required"`
}

// UpdateFeatureReq 更新功能请求
type UpdateFeatureReq struct {
	Name string `json:"name" binding:"required"`
}
