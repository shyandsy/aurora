package types

// EnabledDisabledStatus 通用的启用/禁用状态常量
// 用于需要 ENABLED/DISABLED 状态的实体（如 Schedule、MicroserviceToken 等）
const (
	// EnabledDisabledStatusEnabled 启用状态
	EnabledDisabledStatusEnabled = "ENABLED"
	// EnabledDisabledStatusDisabled 禁用状态
	EnabledDisabledStatusDisabled = "DISABLED"
)

// IsValidEnabledDisabledStatus 验证启用/禁用状态是否有效
func IsValidEnabledDisabledStatus(status string) bool {
	return status == EnabledDisabledStatusEnabled ||
		status == EnabledDisabledStatusDisabled
}

// GetDefaultEnabledDisabledStatus 获取默认状态（启用）
func GetDefaultEnabledDisabledStatus() string {
	return EnabledDisabledStatusEnabled
}
