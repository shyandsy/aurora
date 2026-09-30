package guard

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/shyandsy/sealkit/internal/envelope"
)

// ── Renew wire contract ──────────────────────────────────────────────────────
//
// The renew round-trip is a NONCE CHALLENGE that yields authoritative time +
// authorization, NOT a fresh sealed lease. See docs/protection-model.md §③.
//
//	guard → control:  RenewRequest{ nonce, projectUuid, deploymentId }
//	control → guard:  SignBlob( RenewVerdict{ nonce, now, authorizedUntil, revoked } )
//
// guard verifies: (1) Ed25519 signature with the embedded project public key,
// (2) the echoed nonce equals the one it just sent (anti-replay). The signed `now`
// is the trusted time that defeats local clock rollback.

// RenewRequest is what guard POSTs to the control plane.
type RenewRequest struct {
	Nonce        string `json:"nonce"`           // fresh per request (base64), anti-replay
	ProjectUUID  string `json:"projectUuid"`     // tenant / key selector
	DeploymentID string `json:"deploymentId"`    // which deployment under the project
	KeyID        string `json:"keyId,omitempty"` // optional: which signing key (rotation)
}

// RenewVerdict is the control plane's signed answer (the body inside SignBlob).
type RenewVerdict struct {
	Nonce           string `json:"nonce"`           // MUST echo the request nonce
	Now             int64  `json:"now"`             // authoritative unix seconds (trusted time)
	AuthorizedUntil int64  `json:"authorizedUntil"` // unix seconds; 0 = not authorized
	Revoked         bool   `json:"revoked"`         // true → degrade now
}

var (
	// ErrNonce means the verdict did not echo the nonce we sent (stale/replayed).
	ErrNonce = errors.New("sealkit: renew nonce mismatch (stale/replayed response)")
	// ErrRenewHTTP means the control plane was unreachable or returned non-200.
	ErrRenewHTTP = errors.New("sealkit: renew request failed")
)

// verifyVerdict checks the signature (embedded pubkey) and that the nonce matches
// the one we sent. Returns the parsed verdict.
func verifyVerdict(blob []byte, pub ed25519.PublicKey, sentNonce string) (RenewVerdict, error) {
	body, err := envelope.OpenBlob(blob, pub) // Ed25519 verify
	if err != nil {
		return RenewVerdict{}, err
	}
	var v RenewVerdict
	if err := json.Unmarshal(body, &v); err != nil {
		return RenewVerdict{}, err
	}
	if v.Nonce == "" || v.Nonce != sentNonce {
		return RenewVerdict{}, ErrNonce
	}
	return v, nil
}

// newNonce returns a fresh 16-byte base64 nonce.
func newNonce() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// HTTPRenewer is the default Renewer: it POSTs a RenewRequest to
// {controlURL}/projects/{projectUUID}/deployments/{deploymentID}/renew and returns
// the raw signed verdict blob. Transport only — guard verifies the signature/nonce.
//
// This is the "fill three params and you're integrated" path for any service.
func HTTPRenewer(controlURL, projectUUID, deploymentID string) Renewer {
	base := strings.TrimRight(controlURL, "/")
	url := fmt.Sprintf("%s/projects/%s/deployments/%s/renew", base, projectUUID, deploymentID)
	client := &http.Client{Timeout: 10 * time.Second}

	return func(ctx context.Context, nonce string) ([]byte, error) {
		reqBody, err := json.Marshal(RenewRequest{
			Nonce: nonce, ProjectUUID: projectUUID, DeploymentID: deploymentID,
		})
		if err != nil {
			return nil, err
		}
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
		if err != nil {
			return nil, err
		}
		httpReq.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(httpReq)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("%w: status %d", ErrRenewHTTP, resp.StatusCode)
		}
		return io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	}
}
