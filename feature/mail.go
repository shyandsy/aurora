package feature

import (
	"context"
	"fmt"
	"log"

	"gopkg.in/mail.v2"

	"github.com/shyandsy/aurora/config"
	"github.com/shyandsy/aurora/contracts"
)

// EmailService provides email sending functionality
type EmailService interface {
	// SendText sends a plain text email
	SendText(ctx context.Context, to []string, subject, body string) error

	// SendHTML sends an HTML email
	SendHTML(ctx context.Context, to []string, subject, htmlBody string) error

	// Send sends an email with both plain text and HTML content
	Send(ctx context.Context, to []string, subject, textBody, htmlBody string) error
}

type mailFeature struct {
	config *config.MailConfig
	dialer *mail.Dialer
}

func NewMailFeature() contracts.Features {
	cfg := &config.MailConfig{}
	if err := config.ResolveConfig(cfg); err != nil {
		log.Fatalf("Failed to load mail config: %v", err)
	}
	return &mailFeature{config: cfg}
}

func (f *mailFeature) Name() string {
	return "mail"
}

func (f *mailFeature) Setup(app contracts.App) error {
	if err := f.config.Validate(); err != nil {
		return fmt.Errorf("mail configuration validation failed: %w", err)
	}

	f.dialer = mail.NewDialer(f.config.SMTPHost, f.config.SMTPPort, f.config.SMTPUser, f.config.SMTPPassword)

	mailSvc := &mailService{
		config: f.config,
		dialer: f.dialer,
	}
	app.ProvideAs(mailSvc, (*EmailService)(nil))

	return nil
}

func (f *mailFeature) Close() error {
	return nil
}

type mailService struct {
	config *config.MailConfig
	dialer *mail.Dialer
}

func (s *mailService) SendText(ctx context.Context, to []string, subject, body string) error {
	return s.send(ctx, to, subject, body, "")
}

func (s *mailService) SendHTML(ctx context.Context, to []string, subject, htmlBody string) error {
	return s.send(ctx, to, subject, "", htmlBody)
}

func (s *mailService) Send(ctx context.Context, to []string, subject, textBody, htmlBody string) error {
	return s.send(ctx, to, subject, textBody, htmlBody)
}

func (s *mailService) send(ctx context.Context, to []string, subject, textBody, htmlBody string) error {
	m := mail.NewMessage()

	// Set From
	from := s.config.FromEmail
	if s.config.FromName != "" {
		from = fmt.Sprintf("%s <%s>", s.config.FromName, s.config.FromEmail)
	}
	m.SetHeader("From", from)

	// Set To
	m.SetHeader("To", to...)

	// Set Subject
	m.SetHeader("Subject", subject)

	// Set body
	if textBody != "" && htmlBody != "" {
		m.SetBody("text/plain", textBody)
		m.AddAlternative("text/html", htmlBody)
	} else if htmlBody != "" {
		m.SetBody("text/html", htmlBody)
	} else if textBody != "" {
		m.SetBody("text/plain", textBody)
	} else {
		return fmt.Errorf("either text body or HTML body must be provided")
	}

	// Send email
	return s.dialer.DialAndSend(m)
}
