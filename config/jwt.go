package config

import "time"

// defaultRefreshExpireMultiplier is the fallback multiplier applied to
// ExpireTime when RefreshExpireTime is not explicitly configured.
const defaultRefreshExpireMultiplier = 2

// JWTConfig 是 JWT 引擎的**输入形状**(引擎需要哪些值),不带任何 env tag ——
// 框架不规定密钥等从哪个 env 读、也不定 env 名。由项目自行填充:单域项目可定义一个
// 自己的带 tag 结构体用 config.ResolveConfig 从 env(如 JWT_SECRET)读出再映射进来;
// 多域项目(同一进程校验后台域/客户域两把密钥)按自己的前缀(ADMIN_/CUSTOMER_…)读、
// 组出各自的 JWTConfig。这样"用哪把密钥、env 叫什么"完全归项目,框架零 env 名。
type JWTConfig struct {
	Secret     string
	ExpireTime time.Duration
	Issuer     string
	// RefreshExpireTime is the refresh token lifetime. Optional: when unset
	// (zero) it defaults to ExpireTime * defaultRefreshExpireMultiplier.
	RefreshExpireTime time.Duration
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

	// 错误消息用字段名(Secret/ExpireTime/…)而非具体 env 名 —— env 名由项目决定,
	// 框架不知道也不该假设它叫 JWT_SECRET 还是 ADMIN_JWT_SECRET。
	if s.Secret == "" {
		return NewConfigError("JWT config: Secret is required")
	}

	if s.Secret == "your-super-secret-jwt-key-here-change-in-production" {
		return NewConfigError("JWT config: Secret must be changed from its default value in production")
	}

	if s.ExpireTime <= 0 {
		return NewConfigError("JWT config: ExpireTime must be positive")
	}

	if s.RefreshExpireTime < 0 {
		return NewConfigError("JWT config: RefreshExpireTime must not be negative")
	}

	if s.Issuer == "" {
		return NewConfigError("JWT config: Issuer is required")
	}

	return nil
}
