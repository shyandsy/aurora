package mail

import (
	"bytes"
	"context"
	"net"
	"strings"
	"testing"
	"time"

	gomail "gopkg.in/mail.v2"
)

// TestSend_ReturnsDialResult exercises the non-cancelled path: DialAndSend runs to completion and
// its result flows back through the done channel. A loopback listener that hangs up immediately
// makes the SMTP handshake fail deterministically, without needing a full SMTP server.
func TestSend_ReturnsDialResult(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("no loopback listener: %v", err)
	}
	defer ln.Close()
	go func() {
		if c, err := ln.Accept(); err == nil {
			_ = c.Close() // hang up before the SMTP greeting
		}
	}()

	port := ln.Addr().(*net.TCPAddr).Port
	m := NewSMTP("127.0.0.1", port, WithFrom("f@x.com"), WithTimeout(2*time.Second))
	if err := m.Send(context.Background(), Message{To: []string{"a@b.com"}, Subject: "s", Text: "b"}); err == nil {
		t.Error("send to a server that hangs up should return the dial error")
	}
}

func TestSMTPOptions_ApplyAndDefaults(t *testing.T) {
	m := NewSMTP("h", 25, WithFrom("f@x.com"), WithAuth(NoAuth()), WithTimeout(3*time.Second)).(*smtpMailer)
	if m.host != "h" || m.port != 25 || m.from != "f@x.com" || m.auth == nil || m.timeout != 3*time.Second {
		t.Errorf("options not applied: %+v", m)
	}
	// Default timeout when WithTimeout is omitted.
	if d := NewSMTP("h", 25).(*smtpMailer); d.timeout != defaultSMTPTimeout {
		t.Errorf("default timeout = %v, want %v", d.timeout, defaultSMTPTimeout)
	}
}

// TestWithEncryption proves the encryption option is applied and defaults to auto (empty),
// which keeps gomail's port-based behaviour for existing callers.
func TestWithEncryption(t *testing.T) {
	// Default: no option -> EncryptionAuto ("").
	if m := NewSMTP("h", 25).(*smtpMailer); m.encryption != EncryptionAuto {
		t.Errorf("default encryption = %q, want auto (empty)", m.encryption)
	}
	for _, enc := range []Encryption{EncryptionNone, EncryptionStartTLS, EncryptionSSL} {
		if m := NewSMTP("h", 25, WithEncryption(enc)).(*smtpMailer); m.encryption != enc {
			t.Errorf("WithEncryption(%q): got %q", enc, m.encryption)
		}
	}
}

// TestSend_AuthErrorAbortsBeforeDial proves WithAuth is wired into Send and that an auth failure
// aborts the send before any network I/O (here: XOAuth2 whose token fetch fails).
func TestSend_AuthErrorAbortsBeforeDial(t *testing.T) {
	failing := XOAuth2("u", func(context.Context) (string, error) {
		return "", context.DeadlineExceeded
	})
	m := NewSMTP("smtp.example.com", 587, WithFrom("f@x.com"), WithAuth(failing))
	err := m.Send(context.Background(), Message{To: []string{"a@b.com"}, Subject: "s", Text: "b"})
	if err == nil {
		t.Fatal("auth failure should abort the send")
	}
	if !strings.Contains(err.Error(), "OAuth2 token") {
		t.Errorf("want token error surfaced, got: %v", err)
	}
}

// TestDialerAdapter_MapsToGomailDialer covers the real adapter that bridges SMTPAuth to gomail.
func TestDialerAdapter_MapsToGomailDialer(t *testing.T) {
	d := gomail.NewDialer("h", 587, "", "")
	a := (*dialerAdapter)(d)

	a.SetBasic("user@x", "pass")
	if d.Username != "user@x" || d.Password != "pass" {
		t.Errorf("SetBasic: %q / %q", d.Username, d.Password)
	}

	a.SetMechanism("mech@x", xoauth2Mech{username: "mech@x", token: "t"})
	if d.Username != "mech@x" || d.Auth == nil {
		t.Errorf("SetMechanism: user=%q auth=%v", d.Username, d.Auth)
	}
}

func TestNewSMTP_Guards(t *testing.T) {
	ctx := context.Background()
	valid := Message{To: []string{"a@b.com"}, Subject: "s", Text: "b"}

	if err := NewSMTP("", 465, WithFrom("f@x.com")).Send(ctx, valid); err == nil {
		t.Error("missing host should error")
	}
	if err := NewSMTP("smtp.example.com", 0, WithFrom("f@x.com")).Send(ctx, valid); err == nil {
		t.Error("missing port should error")
	}

	m := NewSMTP("smtp.example.com", 587, WithFrom("f@x.com"))
	if err := m.Send(ctx, Message{Subject: "s", Text: "b"}); err == nil {
		t.Error("no recipients should error")
	}
	if err := m.Send(ctx, Message{To: []string{"a@b.com"}, Subject: "s"}); err == nil {
		t.Error("empty body should error")
	}
}

func TestSend_ContextCancelledReturnsPromptly(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	// A routable-looking host so a real dial would hang for the timeout; ctx must short-circuit it.
	m := NewSMTP("10.255.255.1", 587, WithFrom("f@x.com"))
	err := m.Send(ctx, Message{To: []string{"a@b.com"}, Subject: "s", Text: "b"})
	if err != context.Canceled {
		t.Errorf("cancelled ctx should return context.Canceled, got %v", err)
	}
}

func render(t *testing.T, s *smtpMailer, msg Message) string {
	t.Helper()
	m, err := s.buildMessage(msg)
	if err != nil {
		t.Fatalf("buildMessage: %v", err)
	}
	var buf bytes.Buffer
	if _, err := m.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	return buf.String()
}

func TestBuildMessage_FromFallbackAndError(t *testing.T) {
	s := &smtpMailer{from: "default@x.com"}

	// From falls back to the configured default.
	m, err := s.buildMessage(Message{To: []string{"a@b.com"}, Subject: "s", Text: "b"})
	if err != nil {
		t.Fatal(err)
	}
	if got := m.GetHeader("From"); len(got) != 1 || got[0] != "default@x.com" {
		t.Errorf("From default not applied: %v", got)
	}

	// Explicit From overrides the default.
	m2, _ := s.buildMessage(Message{From: "explicit@x.com", To: []string{"a@b.com"}, Text: "b"})
	if got := m2.GetHeader("From"); len(got) != 1 || got[0] != "explicit@x.com" {
		t.Errorf("explicit From not used: %v", got)
	}

	// No From anywhere -> error (avoid sending a mail servers will reject).
	if _, err := (&smtpMailer{}).buildMessage(Message{To: []string{"a@b.com"}, Text: "b"}); err == nil {
		t.Error("missing From should error")
	}
}

func TestBuildMessage_HeadersAndContent(t *testing.T) {
	s := &smtpMailer{from: "f@x.com"}
	to := []string{"a@b.com"}

	m, _ := s.buildMessage(Message{
		To:      []string{"a@b.com", "a2@b.com"},
		Cc:      []string{"c@b.com"},
		Bcc:     []string{"bcc@b.com"},
		Subject: "Hi", Text: "b",
	})
	if got := m.GetHeader("To"); len(got) != 2 || got[0] != "a@b.com" || got[1] != "a2@b.com" {
		t.Errorf("To (multiple): %v", got)
	}
	if got := m.GetHeader("Cc"); len(got) != 1 || got[0] != "c@b.com" {
		t.Errorf("Cc: %v", got)
	}
	if got := m.GetHeader("Bcc"); len(got) != 1 || got[0] != "bcc@b.com" {
		t.Errorf("Bcc: %v", got)
	}

	textOnly := render(t, s, Message{To: to, Subject: "s", Text: "plain"})
	if !strings.Contains(textOnly, "text/plain") || strings.Contains(textOnly, "text/html") {
		t.Errorf("text-only should be text/plain only:\n%s", textOnly)
	}

	htmlOnly := render(t, s, Message{To: to, Subject: "s", HTML: "<b>x</b>"})
	if !strings.Contains(htmlOnly, "text/html") || strings.Contains(htmlOnly, "text/plain") {
		t.Errorf("html-only should be text/html only:\n%s", htmlOnly)
	}

	both := render(t, s, Message{To: to, Subject: "s", Text: "plain", HTML: "<b>x</b>"})
	if !strings.Contains(both, "multipart/alternative") ||
		!strings.Contains(both, "text/plain") || !strings.Contains(both, "text/html") {
		t.Errorf("both should be multipart/alternative with text+html:\n%s", both)
	}
}
