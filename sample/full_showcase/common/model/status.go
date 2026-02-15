package model

// Status is the common status enum for Customer, User, etc.
const (
	StatusDisable = 0
	StatusEnable  = 1
)

// IsValidStatus checks if status is valid.
func IsValidStatus(status int) bool {
	return status == StatusEnable || status == StatusDisable
}

// GetDefaultStatus returns the default status (enabled).
func GetDefaultStatus() int {
	return StatusEnable
}
