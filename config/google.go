package config

type GoogleConfig struct {
	ClientID     string `env:"GOOGLE_CLIENT_ID"`
	ClientSecret string `env:"GOOGLE_CLIENT_SECRET"`
	RedirectURL  string `env:"GOOGLE_REDIRECT_URL"`
}

func (s *GoogleConfig) Key() string {
	return "google"
}

func (s *GoogleConfig) Validate() error {
	if s.ClientID == "" {
		return NewConfigError("GOOGLE_CLIENT_ID is required")
	}

	if s.ClientSecret == "" {
		return NewConfigError("GOOGLE_CLIENT_SECRET is required")
	}

	if s.RedirectURL == "" {
		return NewConfigError("GOOGLE_REDIRECT_URL is required")
	}

	return nil
}
