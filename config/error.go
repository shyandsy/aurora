package config

type ConfigError struct {
	Message string
}

func NewConfigError(message string) *ConfigError {
	return &ConfigError{Message: message}
}

func (e *ConfigError) Error() string {
	return e.Message
}
