// Package envelope is sealkit's core crypto primitive: a signed, fingerprint-locked
// asset envelope ("绝招一" in the design docs).
//
//	Seal(asset, fingerprint, policy, privKey) -> license bytes
//	Open(license, fingerprint, pubKey)        -> asset bytes, policy
//
// Design (see doc/设计-可信授权SDK):
//   - The asset is encrypted with AES-256-CTR under a key derived from the target
//     machine's fingerprint (key = HKDF-SHA256(fingerprint, salt)). There is NO
//     "if fingerprint == X" branch anywhere — the fingerprint IS the key. A wrong
//     machine derives a wrong key and decrypts to garbage, so the caller's parse of
//     the plaintext fails naturally. Nothing to patch out.
//   - The whole body (salt, iv, ciphertext, policy) is signed with Ed25519. The
//     signature — not AES — provides integrity/authenticity. CTR is used precisely
//     because it has no auth tag: a wrong key yields garbage instead of a clean
//     "decrypt failed" that would itself be a patchable failure point.
//
// This package is zero-dependency (stdlib only): crypto/ed25519, crypto/aes,
// crypto/hmac, crypto/sha256.
package envelope

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
)

// Version is the wire-format version of a license blob.
const Version = 1

// hkdfInfo binds derived keys to this scheme/version.
const hkdfInfo = "sealkit/v1"

// Alg names the algorithm suite, recorded in the body for forward compat.
const Alg = "AES-256-CTR+Ed25519+HKDF-SHA256"

var (
	// ErrSignature means the Ed25519 signature did not verify (forged or tampered).
	ErrSignature = errors.New("sealkit: signature verification failed")
	// ErrFormat means the license blob is malformed / an unknown version.
	ErrFormat = errors.New("sealkit: malformed license")
)

// Policy travels inside the signed body. It is NOT encrypted (so guard can read
// expiry/revocation without the fingerprint) but it IS signed (so it can't be forged).
type Policy struct {
	Issued   int64    `json:"iss"`            // unix seconds when sealed
	Expiry   int64    `json:"exp,omitempty"`  // unix seconds; 0 = no offline expiry
	Features []string `json:"feat,omitempty"` // optional feature flags
	RevID    string   `json:"rev,omitempty"`  // revocation id (for online revoke)
}

// body is the signed portion of a license.
type body struct {
	V      int    `json:"v"`
	Alg    string `json:"alg"`
	Salt   []byte `json:"salt"` // HKDF salt (per-license random)
	IV     []byte `json:"iv"`   // AES-CTR IV
	CT     []byte `json:"ct"`   // AES-256-CTR ciphertext of the asset
	Policy Policy `json:"pol"`
}

// wire is what actually gets written to disk / sent down the wire.
// Neutral, JWT-ish: base64(bodyJSON).base64(sig). Signing the exact bodyJSON bytes
// avoids any JSON-canonicalization ambiguity.
type wire struct {
	Body []byte `json:"b"` // raw bodyJSON bytes
	Sig  []byte `json:"s"` // Ed25519 signature over Body
}

// Seal encrypts asset under the target fingerprint and signs the result.
func Seal(asset, fingerprint []byte, pol Policy, priv ed25519.PrivateKey) ([]byte, error) {
	if len(fingerprint) == 0 {
		return nil, errors.New("sealkit: empty fingerprint")
	}
	if len(priv) != ed25519.PrivateKeySize {
		return nil, errors.New("sealkit: invalid signing key")
	}

	salt := make([]byte, 16)
	iv := make([]byte, aes.BlockSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	if _, err := rand.Read(iv); err != nil {
		return nil, err
	}

	key := hkdfSHA256(fingerprint, salt, []byte(hkdfInfo), 32)
	ct, err := ctrXOR(key, iv, asset)
	if err != nil {
		return nil, err
	}

	bj, err := json.Marshal(body{V: Version, Alg: Alg, Salt: salt, IV: iv, CT: ct, Policy: pol})
	if err != nil {
		return nil, err
	}
	sig := ed25519.Sign(priv, bj)

	return json.Marshal(wire{Body: bj, Sig: sig})
}

// Open verifies the signature and decrypts the asset using the local fingerprint.
//
// IMPORTANT: Open does NOT error on a wrong fingerprint. CTR has no auth tag, so a
// wrong machine gets garbage plaintext back with a nil error — by design ("错机器自然崩").
// The caller MUST parse the returned asset (e.g. json.Unmarshal); a wrong machine's
// garbage will fail that parse. Open only errors on a bad signature / malformed blob.
func Open(license, fingerprint []byte, pub ed25519.PublicKey) ([]byte, Policy, error) {
	if len(pub) != ed25519.PublicKeySize {
		return nil, Policy{}, errors.New("sealkit: invalid public key")
	}
	var w wire
	if err := json.Unmarshal(license, &w); err != nil || len(w.Body) == 0 || len(w.Sig) == 0 {
		return nil, Policy{}, ErrFormat
	}
	if !ed25519.Verify(pub, w.Body, w.Sig) {
		return nil, Policy{}, ErrSignature
	}
	var b body
	if err := json.Unmarshal(w.Body, &b); err != nil {
		return nil, Policy{}, ErrFormat
	}
	if b.V != Version {
		return nil, Policy{}, fmt.Errorf("%w: version %d", ErrFormat, b.V)
	}
	key := hkdfSHA256(fingerprint, b.Salt, []byte(hkdfInfo), 32)
	pt, err := ctrXOR(key, b.IV, b.CT)
	if err != nil {
		return nil, Policy{}, err
	}
	return pt, b.Policy, nil
}

// SignBlob wraps body with an Ed25519 signature (SIGN-ONLY, no encryption). Used
// for the renew verdict: the control plane signs an authorization statement that is
// authentic but not secret. Wire is the same {b,s} shape as a license.
func SignBlob(body []byte, priv ed25519.PrivateKey) ([]byte, error) {
	if len(priv) != ed25519.PrivateKeySize {
		return nil, errors.New("sealkit: invalid signing key")
	}
	return json.Marshal(wire{Body: body, Sig: ed25519.Sign(priv, body)})
}

// OpenBlob verifies a SignBlob and returns the raw body. Errors on bad signature /
// malformed blob. (Counterpart of SignBlob; no encryption involved.)
func OpenBlob(blob []byte, pub ed25519.PublicKey) ([]byte, error) {
	if len(pub) != ed25519.PublicKeySize {
		return nil, errors.New("sealkit: invalid public key")
	}
	var w wire
	if err := json.Unmarshal(blob, &w); err != nil || len(w.Body) == 0 || len(w.Sig) == 0 {
		return nil, ErrFormat
	}
	if !ed25519.Verify(pub, w.Body, w.Sig) {
		return nil, ErrSignature
	}
	return w.Body, nil
}

// ctrXOR runs AES-256-CTR over in (encrypt and decrypt are the same op).
func ctrXOR(key, iv, in []byte) ([]byte, error) {
	block, err := aes.NewCipher(key) // 32-byte key => AES-256
	if err != nil {
		return nil, err
	}
	if len(iv) != aes.BlockSize {
		return nil, errors.New("sealkit: bad IV length")
	}
	out := make([]byte, len(in))
	cipher.NewCTR(block, iv).XORKeyStream(out, in)
	return out, nil
}

// hkdfSHA256 is a minimal RFC-5869 HKDF (Extract+Expand) on SHA-256, stdlib-only.
func hkdfSHA256(ikm, salt, info []byte, length int) []byte {
	if len(salt) == 0 {
		salt = make([]byte, sha256.Size)
	}
	// Extract
	ext := hmac.New(sha256.New, salt)
	ext.Write(ikm)
	prk := ext.Sum(nil)
	// Expand
	var okm, t []byte
	for i := 1; len(okm) < length; i++ {
		exp := hmac.New(sha256.New, prk)
		exp.Write(t)
		exp.Write(info)
		exp.Write([]byte{byte(i)})
		t = exp.Sum(nil)
		okm = append(okm, t...)
	}
	return okm[:length]
}

// EncodePubKey / DecodePubKey / EncodePrivKey / DecodePrivKey are base64 helpers so
// callers (guardctl, go:embed'd pub keys) have one canonical text form.
func EncodePubKey(pub ed25519.PublicKey) string    { return base64.StdEncoding.EncodeToString(pub) }
func EncodePrivKey(priv ed25519.PrivateKey) string { return base64.StdEncoding.EncodeToString(priv) }

func DecodePubKey(s string) (ed25519.PublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	if len(raw) != ed25519.PublicKeySize {
		return nil, errors.New("sealkit: bad public key length")
	}
	return ed25519.PublicKey(raw), nil
}

func DecodePrivKey(s string) (ed25519.PrivateKey, error) {
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	if len(raw) != ed25519.PrivateKeySize {
		return nil, errors.New("sealkit: bad private key length")
	}
	return ed25519.PrivateKey(raw), nil
}
