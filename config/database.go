package config

import (
	"fmt"
	"time"
)

type DatabaseConfig struct {
	Driver       string `env:"DB_DRIVER"`
	DSN          string `env:"DB_DSN"`
	MaxIdleConns int    `env:"DB_MAX_IDLE_CONNS"`
	MaxOpenConns int    `env:"DB_MAX_OPEN_CONNS"`

	// ConnMaxLifetime 单条连接从建立起最长可复用时长。0(不设)= 永不因年龄回收
	// (database/sql 默认,保持旧行为,向后兼容)。设为正值时连接池定期回收连接,
	// 主要作用是避开「MySQL wait_timeout 服务端关掉空闲连接后、连接池仍握着已失效连接」
	// 导致的偶发 invalid connection。建议设为小于 DB 的 wait_timeout(如 1h)。
	ConnMaxLifetime time.Duration `env:"DB_CONN_MAX_LIFETIME,omitempty"`

	// ConnMaxIdleTime 空闲连接最长保留时长。0(不设)= 空闲连接不因空闲时长被关,仅受
	// MaxIdleConns 条数上限约束(保持旧行为)。设为正值时,闲置超过该时长的连接被关,
	// 低峰期把空闲池缩到 MaxIdleConns 以下,进一步降低对 DB 的常驻连接占用。
	ConnMaxIdleTime time.Duration `env:"DB_CONN_MAX_IDLE_TIME,omitempty"`
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

	// 二者可选(omitempty),但给了就不能是负值。0 = 不设,保持旧行为。
	if s.ConnMaxLifetime < 0 {
		return NewConfigError("DB_CONN_MAX_LIFETIME must not be negative")
	}
	if s.ConnMaxIdleTime < 0 {
		return NewConfigError("DB_CONN_MAX_IDLE_TIME must not be negative")
	}

	supportedDrivers := []string{"mysql", "sqlite", "postgres", "postgresql", "pgx"}
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
