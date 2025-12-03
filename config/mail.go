package config

type MailConfig struct {
	SMTPHost     string `env:"MAIL_SMTP_HOST"`
	SMTPPort     int    `env:"MAIL_SMTP_PORT"`
	SMTPUser     string `env:"MAIL_SMTP_USER"`
	SMTPPassword string `env:"MAIL_SMTP_PASSWORD"`
	FromEmail    string `env:"MAIL_FROM_EMAIL"`
	FromName     string `env:"MAIL_FROM_NAME,omitempty"`
}

func (m *MailConfig) Key() string {
	return "mail"
}

func (m *MailConfig) Validate() error {
	if m.SMTPHost == "" {
		return NewConfigError("MAIL_SMTP_HOST is required")
	}

	if m.SMTPPort <= 0 || m.SMTPPort > 65535 {
		return NewConfigError("MAIL_SMTP_PORT must be between 1 and 65535")
	}

	if m.SMTPUser == "" {
		return NewConfigError("MAIL_SMTP_USER is required")
	}

	if m.SMTPPassword == "" {
		return NewConfigError("MAIL_SMTP_PASSWORD is required")
	}

	if m.FromEmail == "" {
		return NewConfigError("MAIL_FROM_EMAIL is required")
	}

	return nil
}
