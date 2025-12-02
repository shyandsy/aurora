package config

type RedisConfig struct {
	Addr     string `env:"REDIS_ADDR"`
	Password string `env:"REDIS_PASSWORD"`
	DB       int    `env:"REDIS_DB"`
}

func (s *RedisConfig) Key() string {
	return "redis"
}

func (s *RedisConfig) Validate() error {
	if s.Addr == "" {
		return NewConfigError("REDIS_ADDR is required")
	}

	if s.Password == "" {
		return NewConfigError("REDIS_PASSWORD is required")
	}

	if s.DB < 0 {
		return NewConfigError("REDIS_DB must be greater than 0")
	}

	return nil
}
