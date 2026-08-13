package mail

import (
	"context"
	"errors"
	"net/smtp"
	"time"

	gomail "gopkg.in/mail.v2"
)

// Compile-time assertions.
var (
	_ Mailer     = (*smtpMailer)(nil)
	_ SMTPDialer = (*dialerAdapter)(nil)
)

const defaultSMTPTimeout = 10 * time.Second

// Encryption selects how the SMTP transport is secured. Empty ("") == EncryptionAuto, which
// preserves gomail's historical behaviour (SSL-on-connect only when port==465, otherwise
// opportunistic STARTTLS that silently falls back to plaintext). Set an explicit value to force
// a mode regardless of port.
type Encryption string

const (
	// EncryptionAuto keeps gomail defaults: SSL iff port==465, else opportunistic STARTTLS with
	// plaintext fallback. This is the zero value, so existing callers are unaffected.
	EncryptionAuto Encryption = ""
	// EncryptionNone sends over plaintext and disables STARTTLS entirely (no encryption).
	EncryptionNone Encryption = "none"
	// EncryptionStartTLS mandates STARTTLS: connect in plaintext then upgrade; fail if the server
	// does not advertise STARTTLS.
	EncryptionStartTLS Encryption = "starttls"
	// EncryptionSSL forces implicit TLS on connect (SMTPS), regardless of port.
	EncryptionSSL Encryption = "ssl"
)

// SMTPOption configures the SMTP mailer (functional options).
type SMTPOption func(*smtpMailer)

// WithFrom sets the default From used when a Message leaves From empty.
func WithFrom(from string) SMTPOption { return func(m *smtpMailer) { m.from = from } }

// WithAuth sets the SMTP authentication strategy (BasicAuth / XOAuth2 / NoAuth / your own).
func WithAuth(a SMTPAuth) SMTPOption { return func(m *smtpMailer) { m.auth = a } }

// WithTimeout bounds the connect+send time (default 10s). Also caps the background dial when a
// send is cancelled via context.
func WithTimeout(d time.Duration) SMTPOption { return func(m *smtpMailer) { m.timeout = d } }

// WithEncryption forces the transport security mode. Omit (or pass EncryptionAuto) to keep the
// port-based auto behaviour.
func WithEncryption(e Encryption) SMTPOption { return func(m *smtpMailer) { m.encryption = e } }

type smtpMailer struct {
	host       string
	port       int
	from       string
	auth       SMTPAuth
	timeout    time.Duration
	encryption Encryption
}

// NewSMTP builds a Mailer that sends over SMTP (Gmail / Outlook / any host). host+port are
// required; From/auth/timeout are set via options.
func NewSMTP(host string, port int, opts ...SMTPOption) Mailer {
	m := &smtpMailer{host: host, port: port, timeout: defaultSMTPTimeout}
	for _, o := range opts {
		o(m)
	}
	return m
}

func (s *smtpMailer) Send(ctx context.Context, msg Message) error {
	if s.host == "" || s.port <= 0 {
		return errors.New("mail: SMTP host/port not configured")
	}
	if err := msg.validate(); err != nil {
		return err
	}
	m, err := s.buildMessage(msg)
	if err != nil {
		return err
	}

	d := gomail.NewDialer(s.host, s.port, "", "")
	d.Timeout = s.timeout
	// Transport security. EncryptionAuto leaves gomail's port-based defaults (SSL iff port==465,
	// else opportunistic STARTTLS). Explicit modes override regardless of port.
	switch s.encryption {
	case EncryptionSSL:
		d.SSL = true
	case EncryptionStartTLS:
		d.SSL = false
		d.StartTLSPolicy = gomail.MandatoryStartTLS
	case EncryptionNone:
		d.SSL = false
		d.StartTLSPolicy = gomail.NoStartTLS
	case EncryptionAuto:
		// keep gomail defaults
	}
	if s.auth != nil {
		if err := s.auth.Apply(ctx, (*dialerAdapter)(d)); err != nil {
			return err
		}
	}

	// gomail's DialAndSend is not context-aware; run it and let the caller be cancelled via ctx.
	// The background dial is bounded by d.Timeout, so the goroutine cannot outlive that.
	done := make(chan error, 1)
	go func() { done <- d.DialAndSend(m) }()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}

// dialerAdapter exposes the gomail dialer as an SMTPDialer to auth strategies, keeping the
// concrete SMTP library out of the public API.
type dialerAdapter gomail.Dialer

func (a *dialerAdapter) SetBasic(username, password string) {
	a.Username = username
	a.Password = password
}

func (a *dialerAdapter) SetMechanism(username string, mech smtp.Auth) {
	a.Username = username
	a.Auth = mech
}

// buildMessage maps a provider-agnostic Message onto a gomail message: From fallback (errors if
// unresolved), recipient headers, and content selection (text / html / both -> multipart).
// Split out so header/content logic is unit-testable without dialing an SMTP server.
func (s *smtpMailer) buildMessage(msg Message) (*gomail.Message, error) {
	from := msg.From
	if from == "" {
		from = s.from
	}
	if from == "" {
		return nil, errors.New("mail: no From address (set Message.From or WithFrom)")
	}

	m := gomail.NewMessage()
	m.SetHeader("From", from)
	m.SetHeader("To", msg.To...)
	if len(msg.Cc) > 0 {
		m.SetHeader("Cc", msg.Cc...)
	}
	if len(msg.Bcc) > 0 {
		m.SetHeader("Bcc", msg.Bcc...)
	}
	m.SetHeader("Subject", msg.Subject)

	// Both set -> multipart/alternative (plain-text fallback + HTML). Otherwise a single part.
	switch {
	case msg.Text != "" && msg.HTML != "":
		m.SetBody("text/plain", msg.Text)
		m.AddAlternative("text/html", msg.HTML)
	case msg.HTML != "":
		m.SetBody("text/html", msg.HTML)
	default:
		m.SetBody("text/plain", msg.Text)
	}

	return m, nil
}
