// Package guard is the RUNTIME library embedded into a protected image (go:embed
// the public key, import guard). It has two entry points:
//
//	Unlock(...)  — cold-start: verify + fingerprint-decrypt a sealed asset (绝招一).
//	Start(...)   — the online-lease + graceful-degradation SDK that external services
//	               (e.g. homeserver) integrate with two touchpoints: Start + Middleware.
//
// Guard NEVER holds a signing private key — only the embedded public key. See
// doc/设计-可信授权SDK(sealkit).md and doc/安全方案-授权租约与优雅降级(总纲).md.
package guard

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/shyandsy/sealkit/internal/envelope"
	"github.com/shyandsy/sealkit/internal/fingerprint"
)

// Policy is re-exported so callers depend only on guard, not internal/.
type Policy = envelope.Policy

var (
	// ErrExpired is returned when the offline policy expiry has passed. Note: offline
	// expiry alone is defeatable by clock rollback — the authoritative anti-rollback
	// path is online renewal (see lease.go / the design 机制二).
	ErrExpired = errors.New("sealkit: license expired")
	// ErrUnlock is the generic PROD-mode failure (no detail leaked). DevMode returns
	// the underlying cause instead.
	ErrUnlock = errors.New("sealkit: unlock failed")
)

// Opts configures a one-shot Unlock.
type Opts struct {
	// LicensePath is the sealed blob on disk (neutral name, e.g. /etc/app/config.dat).
	LicensePath string
	// LicenseBytes, if set, is used instead of reading LicensePath (for testing / for
	// leases delivered in-memory).
	LicenseBytes []byte
	// PubKey is the control-side public key, typically //go:embed'd.
	PubKey ed25519.PublicKey
	// Fingerprint overrides the machine fingerprint (tests / custom collectors).
	// When nil, guard reads the local stable fingerprint.
	Fingerprint []byte
	// Now overrides the clock (tests). When nil, time.Now is used.
	Now func() time.Time
	// DevMode returns descriptive errors for integration debugging. PROD builds
	// should leave this false so failures are opaque.
	DevMode bool
}

// Unlock verifies the license signature and decrypts the asset with the local
// fingerprint. On a wrong machine the decrypt yields garbage (no error) — the
// CALLER must parse the returned asset and treat a parse failure as "wrong machine".
func Unlock(o Opts) ([]byte, Policy, error) {
	raw := o.LicenseBytes
	if raw == nil {
		b, err := os.ReadFile(o.LicensePath)
		if err != nil {
			return nil, Policy{}, wrap(o.DevMode, err)
		}
		raw = b
	}

	fp := o.Fingerprint
	if fp == nil {
		f, err := fingerprint.Local()
		if err != nil {
			return nil, Policy{}, wrap(o.DevMode, err)
		}
		fp = f
	}

	asset, pol, err := envelope.Open(raw, fp, o.PubKey)
	if err != nil {
		return nil, Policy{}, wrap(o.DevMode, err)
	}

	if pol.Expiry != 0 {
		now := time.Now
		if o.Now != nil {
			now = o.Now
		}
		if now().Unix() > pol.Expiry {
			return nil, pol, wrap(o.DevMode, ErrExpired)
		}
	}
	return asset, pol, nil
}

// DecodePub parses a base64 public key (e.g. a //go:embed'd pub.key), so a runtime
// service can depend on guard alone.
func DecodePub(b64 string) (ed25519.PublicKey, error) { return envelope.DecodePubKey(b64) }

func wrap(dev bool, err error) error {
	if dev {
		return fmt.Errorf("%w: %v", ErrUnlock, err)
	}
	return ErrUnlock
}
