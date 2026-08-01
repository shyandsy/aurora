package config

import (
	"reflect"
	"testing"
)

func baseServerConfig() *ServerConfig {
	return &ServerConfig{Host: "0.0.0.0", Port: 8080, Name: "svc", RunLevel: RunLevelLocal}
}

func TestResolvedTrustedProxies(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"unset uses private default", nil, DefaultTrustedProxies},
		{"empty uses private default", []string{}, DefaultTrustedProxies},
		{"none sentinel trusts nobody", []string{"none"}, []string{}},
		{"NONE is case-insensitive", []string{" NONE "}, []string{}},
		{"explicit cidrs pass through", []string{"192.168.1.0/24", "10.1.2.3"}, []string{"192.168.1.0/24", "10.1.2.3"}},
		{"trust-all restores gin legacy", []string{"0.0.0.0/0", "::/0"}, []string{"0.0.0.0/0", "::/0"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := &ServerConfig{TrustedProxies: tc.in}
			if got := s.ResolvedTrustedProxies(); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ResolvedTrustedProxies() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestValidateTrustedProxies(t *testing.T) {
	// Valid: unset (default), sentinel, and correct IP/CIDR lists all pass.
	for _, in := range [][]string{nil, {"none"}, {"10.0.0.0/8"}, {"1.2.3.4"}, {"0.0.0.0/0", "::/0"}} {
		c := baseServerConfig()
		c.TrustedProxies = in
		if err := c.Validate(); err != nil {
			t.Fatalf("Validate() unexpected error for %v: %v", in, err)
		}
	}
	// Invalid CIDR/IP → error.
	for _, in := range [][]string{{"not-an-ip"}, {"10.0.0.0/8", "999.0.0.1"}, {"10.0.0.0/33"}} {
		c := baseServerConfig()
		c.TrustedProxies = in
		if err := c.Validate(); err == nil {
			t.Fatalf("Validate() expected error for %v, got nil", in)
		}
	}
}
