package config

import (
	"testing"
	"time"
)

func TestJWTConfig_RefreshExpireOrDefault(t *testing.T) {
	tests := []struct {
		name       string
		expire     time.Duration
		refresh    time.Duration
		wantResult time.Duration
	}{
		{
			name:       "defaults to ExpireTime * 2 when unset",
			expire:     15 * time.Minute,
			refresh:    0,
			wantResult: 30 * time.Minute,
		},
		{
			name:       "uses configured value when set",
			expire:     15 * time.Minute,
			refresh:    24 * time.Hour,
			wantResult: 24 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &JWTConfig{ExpireTime: tt.expire, RefreshExpireTime: tt.refresh}
			if got := cfg.RefreshExpireOrDefault(); got != tt.wantResult {
				t.Errorf("RefreshExpireOrDefault() = %v, want %v", got, tt.wantResult)
			}
		})
	}
}

func TestJWTConfig_Validate_RefreshExpireTime(t *testing.T) {
	base := func() *JWTConfig {
		return &JWTConfig{
			Secret:     "a-non-default-secret",
			ExpireTime: 15 * time.Minute,
			Issuer:     "myapp",
		}
	}

	t.Run("unset refresh is valid", func(t *testing.T) {
		if err := base().Validate(); err != nil {
			t.Errorf("expected valid config, got error: %v", err)
		}
	})

	t.Run("positive refresh is valid", func(t *testing.T) {
		cfg := base()
		cfg.RefreshExpireTime = time.Hour
		if err := cfg.Validate(); err != nil {
			t.Errorf("expected valid config, got error: %v", err)
		}
	})

	t.Run("negative refresh is rejected", func(t *testing.T) {
		cfg := base()
		cfg.RefreshExpireTime = -time.Hour
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for negative JWT_REFRESH_EXPIRE_TIME, got nil")
		}
	})
}
