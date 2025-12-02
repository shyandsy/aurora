package config

type CORSConfig struct {
	AllowedOrigins   []string `env:"CORS_ALLOWED_ORIGINS,omitempty"`
	AllowedMethods   []string `env:"CORS_ALLOWED_METHODS,omitempty"`
	AllowedHeaders   []string `env:"CORS_ALLOWED_HEADERS,omitempty"`
	AllowCredentials bool     `env:"CORS_ALLOWED_CREDENTIALS,omitempty"`
}

func (s *CORSConfig) Key() string {
	return "cors"
}

func (s *CORSConfig) Validate() error {
	if len(s.AllowedOrigins) == 0 {
		return NewConfigError("CORS_ALLOWED_ORIGINS is required")
	}

	if len(s.AllowedMethods) == 0 {
		return NewConfigError("CORS_ALLOWED_METHODS is required")
	}

	if len(s.AllowedHeaders) == 0 {
		return NewConfigError("CORS_ALLOWED_HEADERS is required")
	}

	return nil
}
