// Package fingerprint reads a stable machine identity for the lock-key layer.
//
// The design (doc §3) splits fingerprinting into two layers; this package is the
// KEY layer: only attributes that survive reboot / NIC swap / disk swap / kernel
// upgrade, so a legitimate machine never fails to decrypt. It intentionally does
// NOT use MAC addresses (host vs container read differently) — only files that
// both the control side (over SSH) and the container read identically.
//
// Honest boundary: on a machine you own root of, these values are readable and (on
// a VM host) spoofable. The fingerprint is the AES key, not a secret guard — what
// actually raises the cost is obfuscating "which attributes, combined how" (L2 in
// the design). See doc/安全方案-授权租约与优雅降级(总纲).md.
package fingerprint

import (
	"crypto/sha256"
	"errors"
	"os"
	"strings"
)

// sources is the priority fallback chain. First non-empty value from each path is
// mixed in; product_uuid + machine-id are the primary, stable pair.
var sources = [][]string{
	{"/host/sys/class/dmi/id/product_uuid", "/sys/class/dmi/id/product_uuid"},
	{"/host/etc/machine-id", "/etc/machine-id"},
	{"/host/sys/class/dmi/id/board_serial", "/sys/class/dmi/id/board_serial"},
}

// ErrNoIdentity means no stable identity file could be read. Callers MUST treat
// this as fatal at collect time rather than sealing against an empty fingerprint
// (which would let any/no machine decrypt). See doc §3 "可靠性坑".
var ErrNoIdentity = errors.New("sealkit: no stable machine identity found")

// Local computes sha256 over the concatenation of the first readable value in each
// source group. Returns ErrNoIdentity if nothing usable was found.
func Local() ([]byte, error) {
	h := sha256.New()
	found := false
	for _, group := range sources {
		for _, p := range group {
			if v := readTrim(p); v != "" {
				h.Write([]byte(v))
				h.Write([]byte{0}) // separator, avoid concatenation ambiguity
				found = true
				break
			}
		}
	}
	if !found {
		return nil, ErrNoIdentity
	}
	return h.Sum(nil), nil
}

func readTrim(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}
