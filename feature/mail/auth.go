package mail

import (
	"context"
	"fmt"
	"net/smtp"
)

// SMTPDialer is the sink an SMTPAuth configures. It is implemented internally over the SMTP
// transport, so auth strategies — including your own — depend only on this interface and the
// standard net/smtp, never on the concrete SMTP library. This is how the auth axis stays open
// for extension while the transport detail stays encapsulated.
type SMTPDialer interface {
	// SetBasic sets username + password; the transport auto-selects PLAIN or LOGIN.
	SetBasic(username, password string)
	// SetMechanism sets a custom SASL mechanism (e.g. XOAUTH2).
	SetMechanism(username string, mech smtp.Auth)
}

// Compile-time assertions: the built-in strategies satisfy SMTPAuth, and the XOAUTH2 mechanism
// satisfies net/smtp.Auth.
var (
	_ SMTPAuth  = basicAuth{}
	_ SMTPAuth  = noAuth{}
	_ SMTPAuth  = xoauth2{}
	_ smtp.Auth = xoauth2Mech{}
)

// SMTPAuth configures authentication for one SMTP send. Implement it to add a mechanism;
// built-ins: BasicAuth, NoAuth, XOAuth2. It's passed to NewSMTP via WithAuth.
//
// Apply receives the send context so token-based strategies can fetch fresh, cancellable
// credentials, and returns an error to abort the send (e.g. token fetch failed).
type SMTPAuth interface {
	Apply(ctx context.Context, d SMTPDialer) error
}

// ---- username + password (PLAIN/LOGIN) ----

type basicAuth struct{ username, password string }

func (a basicAuth) Apply(_ context.Context, d SMTPDialer) error {
	d.SetBasic(a.username, a.password)
	return nil
}

// BasicAuth authenticates with username + password (SMTP PLAIN/LOGIN) — the common case
// (Gmail app password, Outlook, most providers).
func BasicAuth(username, password string) SMTPAuth { return basicAuth{username, password} }

// ---- no auth ----

type noAuth struct{}

func (noAuth) Apply(context.Context, SMTPDialer) error { return nil }

// NoAuth sends without authentication (e.g. an internal relay / open MTA).
func NoAuth() SMTPAuth { return noAuth{} }

// ---- OAuth2 bearer token (SASL XOAUTH2) ----

// TokenSource returns a fresh OAuth2 access token. Called once per send (with the send context)
// so short-lived, rotating tokens stay valid.
type TokenSource func(ctx context.Context) (string, error)

type xoauth2 struct {
	username string
	token    TokenSource
}

func (a xoauth2) Apply(ctx context.Context, d SMTPDialer) error {
	if a.token == nil {
		return fmt.Errorf("mail: XOAuth2 token source is nil")
	}
	tok, err := a.token(ctx)
	if err != nil {
		return fmt.Errorf("mail: fetch OAuth2 token: %w", err)
	}
	d.SetMechanism(a.username, xoauth2Mech{username: a.username, token: tok})
	return nil
}

// XOAuth2 authenticates via the SASL XOAUTH2 mechanism (OAuth2 bearer token) — the modern auth
// for Gmail / Outlook / Office 365.
func XOAuth2(username string, token TokenSource) SMTPAuth {
	return xoauth2{username: username, token: token}
}

// xoauth2Mech implements net/smtp.Auth for XOAUTH2 with an already-fetched token.
type xoauth2Mech struct{ username, token string }

func (m xoauth2Mech) Start(*smtp.ServerInfo) (string, []byte, error) {
	// SASL XOAUTH2 initial client response: "user=<user>^Aauth=Bearer <token>^A^A".
	return "XOAUTH2", []byte("user=" + m.username + "\x01auth=Bearer " + m.token + "\x01\x01"), nil
}

func (m xoauth2Mech) Next(_ []byte, more bool) ([]byte, error) {
	// On auth failure the server sends a base64 error challenge with more=true; XOAUTH2 requires
	// an empty client response to finish the exchange (surfacing the server's error).
	if more {
		return []byte{}, nil
	}
	return nil, nil
}
