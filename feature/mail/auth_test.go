package mail

import (
	"context"
	"errors"
	"net/smtp"
	"testing"
)

// fakeDialer implements SMTPDialer — same way an external custom SMTPAuth would be tested,
// proving the auth axis is usable/verifiable without the concrete SMTP library.
type fakeDialer struct {
	basicUser, basicPass string
	mechUser             string
	mech                 smtp.Auth
}

func (f *fakeDialer) SetBasic(u, p string)                { f.basicUser, f.basicPass = u, p }
func (f *fakeDialer) SetMechanism(u string, m smtp.Auth) { f.mechUser, f.mech = u, m }

func TestBasicAuth_SetsUserPass(t *testing.T) {
	var d fakeDialer
	if err := BasicAuth("user@x", "pass").Apply(context.Background(), &d); err != nil {
		t.Fatal(err)
	}
	if d.basicUser != "user@x" || d.basicPass != "pass" {
		t.Errorf("BasicAuth: %q / %q", d.basicUser, d.basicPass)
	}
	if d.mech != nil {
		t.Error("BasicAuth should not set a custom mechanism")
	}
}

func TestNoAuth_SetsNothing(t *testing.T) {
	var d fakeDialer
	if err := NoAuth().Apply(context.Background(), &d); err != nil {
		t.Fatal(err)
	}
	if d.basicUser != "" || d.basicPass != "" || d.mech != nil {
		t.Error("NoAuth should leave the dialer untouched")
	}
}

func TestXOAuth2_SetsMechanismAndSASLResponse(t *testing.T) {
	var d fakeDialer
	err := XOAuth2("user@x", func(context.Context) (string, error) { return "TOK", nil }).
		Apply(context.Background(), &d)
	if err != nil {
		t.Fatal(err)
	}
	if d.mechUser != "user@x" || d.mech == nil {
		t.Fatalf("XOAuth2 did not set the mechanism: user=%q mech=%v", d.mechUser, d.mech)
	}

	proto, resp, err := d.mech.Start(nil)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if proto != "XOAUTH2" {
		t.Errorf("proto = %q, want XOAUTH2", proto)
	}
	if want := "user=user@x\x01auth=Bearer TOK\x01\x01"; string(resp) != want {
		t.Errorf("resp = %q, want %q", resp, want)
	}
	// Server error challenge (more=true) -> empty non-nil response.
	if next, err := d.mech.Next([]byte("base64err"), true); err != nil || next == nil {
		t.Errorf("Next(more=true) = %v, %v; want empty non-nil, nil", next, err)
	}
	// Normal completion (more=false) -> nil, nil.
	if next, err := d.mech.Next(nil, false); err != nil || next != nil {
		t.Errorf("Next(more=false) = %v, %v; want nil, nil", next, err)
	}
}

func TestXOAuth2_NilTokenSourceErrors(t *testing.T) {
	var d fakeDialer
	if err := XOAuth2("u", nil).Apply(context.Background(), &d); err == nil {
		t.Error("nil token source should error")
	}
}

func TestXOAuth2_TokenErrorPropagates(t *testing.T) {
	var d fakeDialer
	err := XOAuth2("u", func(context.Context) (string, error) { return "", errors.New("boom") }).
		Apply(context.Background(), &d)
	if err == nil {
		t.Error("token source error should propagate from Apply")
	}
	if d.mech != nil {
		t.Error("mechanism must not be set when the token fetch fails")
	}
}
