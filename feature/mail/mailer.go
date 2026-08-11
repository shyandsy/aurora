// Package mail is a provider-agnostic email sending abstraction.
//
// Apps depend on Mailer and pick an implementation at construction time. The dimensions:
//
//   - Content: Message carries Text and/or HTML (both -> multipart/alternative). See mailer.go.
//   - Auth (SMTP): pluggable via SMTPAuth — BasicAuth now, XOAuth2/etc. later. See auth.go.
//   - Provider: each provider is a Mailer from its own constructor — NewSMTP built in (smtp.go);
//     Aliyun DirectMail / AWS SES / SendGrid / … are just more Mailer implementations (e.g. in
//     mail/aliyun, mail/ses subpackages) so their SDK deps never bloat the core.
//
// The consumer only ever sees Mailer.Send(ctx, Message), so swapping provider/auth/credentials
// never touches call sites.
package mail

import (
	"context"
	"errors"
)

// Message is a provider-agnostic email.
type Message struct {
	From    string // optional; empty falls back to the provider's default From
	To      []string
	Cc      []string
	Bcc     []string
	Subject string
	Text    string // plain-text body
	HTML    string // optional HTML body
}

func (m Message) validate() error {
	if len(m.To) == 0 {
		return errors.New("mail: no recipients (To is empty)")
	}
	if m.Text == "" && m.HTML == "" {
		return errors.New("mail: empty body (set Text and/or HTML)")
	}
	return nil
}

// Mailer sends email. Apps depend on this; swap implementations (provider/auth) freely.
type Mailer interface {
	Send(ctx context.Context, msg Message) error
}
