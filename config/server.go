package config

import (
	"fmt"
	"net"
	"time"
)

const (
	RunLevelLocal      = "local"
	RunLevelStage      = "stage"
	RunLevelProduction = "production"
)

var ValidRunLevels = map[string]bool{
	RunLevelLocal:      true,
	RunLevelStage:      true,
	RunLevelProduction: true,
}

func validRunLevelsString() string {
	return fmt.Sprintf("%s, %s, %s", RunLevelLocal, RunLevelStage, RunLevelProduction)
}

type ServerConfig struct {
	Host            string        `env:"HOST" envDefault:"0.0.0.0"`
	Port            int           `env:"PORT" envDefault:"8080"`
	ReadTimeout     time.Duration `env:"READ_TIMEOUT" envDefault:"30s"`
	WriteTimeout    time.Duration `env:"WRITE_TIMEOUT" envDefault:"30s"`
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" envDefault:"5s"`

	Name     string `env:"SERVICE_NAME" envDefault:"myapp"`
	Version  string `env:"SERVICE_VERSION" envDefault:"1.0.0"`
	RunLevel string `env:"RUN_LEVEL" envDefault:"local"`
}

func (s *ServerConfig) Key() string {
	return "server"
}

func (s *ServerConfig) Validate() error {
	ip := net.ParseIP(s.Host)
	if ip == nil {
		return NewConfigError("HOST should be a valid IP address")
	}

	if s.Port < 1 || s.Port > 65535 {
		return NewConfigError("PORT should be in [1, 65535]")
	}

	if s.Name == "" {
		return NewConfigError("SERVICE_NAME is required")
	}

	if !ValidRunLevels[s.RunLevel] {
		return NewConfigError(fmt.Sprintf("RUN_LEVEL must be one of: %s", validRunLevelsString()))
	}

	return nil
}

func (s *ServerConfig) IsProduction() bool {
	return s.RunLevel == RunLevelProduction
}

func (s *ServerConfig) GinMode() string {
	if s.IsProduction() {
		return "release"
	}
	return "debug"
}
