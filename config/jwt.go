package config

import "time"

// defaultRefreshExpireMultiplier is the fallback multiplier applied to
// ExpireTime when RefreshExpireTime is not explicitly configured.
const defaultRefreshExpireMultiplier = 2

type JWTConfig struct {
	Secret     string        `env:"JWT_SECRET"`
	ExpireTime time.Duration `env:"JWT_EXPIRE_TIME"`
	Issuer     string        `env:"JWT_ISSUER"`
	// RefreshExpireTime is the refresh token lifetime. Optional: when unset
	// (zero) it defaults to ExpireTime * defaultRefreshExpireMultiplier.
	RefreshExpireTime time.Duration `env:"JWT_REFRESH_EXPIRE_TIME,omitempty"`
}

func (s *JWTConfig) Key() string {
	return "jwt"
}

// RefreshExpireOrDefault returns the configured refresh token lifetime, or the
// default (ExpireTime * defaultRefreshExpireMultiplier) when it is not set.
func (s *JWTConfig) RefreshExpireOrDefault() time.Duration {
	if s.RefreshExpireTime > 0 {
		return s.RefreshExpireTime
	}
	return s.ExpireTime * defaultRefreshExpireMultiplier
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

	if s.RefreshExpireTime < 0 {
		return NewConfigError("JWT_REFRESH_EXPIRE_TIME must not be negative")
	}

	if s.Issuer == "" {
		return NewConfigError("JWT_ISSUER is required")
	}

	return nil
}
