package model

// Status 通用状态枚举
// 用于 Customer、User 等实体的状态字段
const (
	// StatusDisable 禁用
	StatusDisable = 0
	// StatusEnable 启用
	StatusEnable = 1
)

// IsValidStatus 验证状态是否有效
func IsValidStatus(status int) bool {
	return status == StatusEnable || status == StatusDisable
}

// GetDefaultStatus 获取默认状态（启用）
func GetDefaultStatus() int {
	return StatusEnable
}
