package config

import "time"

type JWTConfig struct {
	Secret     string        `env:"JWT_SECRET"`
	ExpireTime time.Duration `env:"JWT_EXPIRE_TIME"`
	Issuer     string        `env:"JWT_ISSUER"`
}

func (s *JWTConfig) Key() string {
	return "jwt"
}

func (s *JWTConfig) Validate() error {
	if s == nil {
		return NewConfigError("JWT config is required")
	}

	if s.Secret == "" {
		return NewConfigError("JWT_SECRET is required")
	}

	if s.Secret == "your-super-secret-jwt-key-here-change-in-production" {
		return NewConfigError("JWT_SECRET must be changed from default value in production")
	}

	if s.ExpireTime <= 0 {
		return NewConfigError("JWT_EXPIRE_TIME must be positive")
	}

	if s.Issuer == "" {
		return NewConfigError("JWT_ISSUER is required")
	}

	return nil
}
