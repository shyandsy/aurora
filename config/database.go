package config

import "fmt"

type DatabaseConfig struct {
	Driver       string `env:"DB_DRIVER"`
	DSN          string `env:"DB_DSN"`
	MaxIdleConns int    `env:"DB_MAX_IDLE_CONNS"`
	MaxOpenConns int    `env:"DB_MAX_OPEN_CONNS"`
}

func (s *DatabaseConfig) Key() string {
	return "database"
}

func (s *DatabaseConfig) Validate() error {
	if s == nil {
		return NewConfigError("database config is required")
	}
	if s.Driver == "" {
		return NewConfigError("DB_DRIVER is required")
	}

	if s.DSN == "" {
		return NewConfigError("DB_DSN is required")
	}

	if s.MaxIdleConns <= 0 {
		return NewConfigError("DB_MAX_IDLE_CONNS must be greater than 0")
	}

	if s.MaxOpenConns <= 0 {
		return NewConfigError("DB_MAX_OPEN_CONNS must be greater than 0")
	}

	if s.MaxIdleConns > s.MaxOpenConns {
		return NewConfigError("DB_MAX_IDLE_CONNS must be less than or equal to DB_MAX_OPEN_CONNS")
	}

	supportedDrivers := []string{"mysql", "sqlite"}
	found := false
	for _, driver := range supportedDrivers {
		if s.Driver == driver {
			found = true
		}
	}
	if !found {
		return NewConfigError(fmt.Sprintf("unsupported database driver: %s, supported drivers: %v", s.Driver, supportedDrivers))
	}
	return nil
}
