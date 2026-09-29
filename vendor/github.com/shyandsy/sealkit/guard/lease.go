package guard

import (
	"context"
	"crypto/ed25519"
	"math/rand/v2"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// This file is the "light tier" (轻档) online-lease + graceful-degradation SDK from
// doc/protection-model.md §③. Two touchpoints: Start at boot, Middleware on the
// router. The crypto/renew/trusted-time/degradation stay hidden here.
//
// Model (protection-model §③): the node holds a signed lease with an embedded
// expiry (offline backstop), and periodically does a NONCE-CHALLENGE renew to fetch
// authoritative time + authorization. Trusted time (the signed `now`, advanced by a
// MONOTONIC clock between calls) defeats local clock rollback (`date -s`). Revoke /
// expiry-vs-trusted-time / can't-renew-past-grace → graceful degradation (random
// latency, not a hard kill).
//
// The "heavy tier" (lease-as-key woven into the request path, integrity guard web,
// OLLVM) is deliberately NOT a library call — build-time hardening for crown jewels.

// Renewer performs one renewal round-trip: given a fresh nonce, it returns the
// control plane's SIGNED verdict blob (raw bytes). It is transport-only — guard
// verifies the signature and nonce (see verifyVerdict). Return an error if the
// control plane is unreachable / refused. Use HTTPRenewer for the default HTTP impl.
type Renewer func(ctx context.Context, nonce string) (signedVerdict []byte, err error)

// Config drives Start. Zero values pick safe defaults.
type Config struct {
	// LicensePath / LicenseBytes / PubKey / Fingerprint / DevMode mirror Opts for the
	// initial Unlock. LicenseBytes (in-memory) takes precedence over LicensePath.
	// PubKey is ALSO the key used to verify renew verdicts (the project public key).
	LicensePath  string
	LicenseBytes []byte
	PubKey       ed25519.PublicKey
	Fingerprint  []byte
	DevMode      bool

	// Renew fetches fresh authoritative time + authorization. If nil, guard runs
	// pure-offline: it relies only on the sealed policy's embedded Expiry (defeatable
	// by clock rollback — fine for low-value assets, not crown jewels).
	Renew Renewer

	// RenewEvery is how often to attempt renewal (default 1h). Jittered ±20%.
	RenewEvery time.Duration
	// GraceWindow is how long we tolerate failed renewal before degrading (default
	// 72h). Big on purpose: a network blip must not degrade a paying customer.
	GraceWindow time.Duration

	// Degrade tunes the random-latency injector (see Middleware). Zero => defaults.
	Degrade DegradeConfig

	// Now overrides the clock (tests). When nil, time.Now is used. NOTE: real
	// time.Now carries a monotonic reading, which is what makes rollback-resistance
	// work between renews; test clocks without monotonic still function correctly.
	Now func() time.Time
}

// DegradeConfig controls the random-latency degradation. Defaults are conservative:
// low probability, moderate latency — "looks like a network/load problem", not a crash.
type DegradeConfig struct {
	Probability float64       // chance to inject latency per request while degraded (default 0.02)
	Min         time.Duration // min injected latency (default 3s)
	Max         time.Duration // max injected latency (default 10s)
}

// Guard is the live handle returned by Start. Middleware reads its state per request
// with a single atomic load — no crypto, no network on the hot path.
type Guard struct {
	degraded atomic.Bool
	deg      DegradeConfig
	cancel   context.CancelFunc
	now      func() time.Time

	mu     sync.RWMutex
	policy Policy
	asset  []byte
	// authorizedUntil is the trusted-time deadline (unix seconds); 0 = none.
	authorizedUntil int64
	// trusted-time anchor: at anchorMono (a g.now() reading carrying the monotonic
	// clock), the authoritative time was anchorTrusted. trustedNow advances it by the
	// monotonic elapsed since, so a wall-clock rollback cannot move it backwards.
	anchorTrusted int64
	anchorMono    time.Time
	// lastRenewOK is the g.now() of the last successful renew (for the grace window).
	lastRenewOK time.Time
}

// Start does the initial Unlock, seeds the offline expiry backstop, then launches a
// background loop that renews (if a Renewer is set) and flips degradation when the
// lease is revoked / expired (trusted time) / un-renewable past the grace window.
// Returns the Guard and the unlocked asset (nil if the sealed blob carried none).
func Start(cfg Config) (*Guard, []byte, error) {
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	asset, pol, err := Unlock(Opts{
		LicensePath:  cfg.LicensePath,
		LicenseBytes: cfg.LicenseBytes,
		PubKey:       cfg.PubKey,
		Fingerprint:  cfg.Fingerprint,
		Now:          now,
		DevMode:      cfg.DevMode,
	})
	if err != nil {
		return nil, nil, err
	}

	g := &Guard{deg: withDegradeDefaults(cfg.Degrade), now: now}
	g.mu.Lock()
	g.policy, g.asset = pol, asset
	g.authorizedUntil = pol.Expiry // embedded expiry = offline backstop
	g.lastRenewOK = now()
	g.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	g.cancel = cancel
	go g.loop(ctx, cfg)
	return g, asset, nil
}

// Stop halts the background loop.
func (g *Guard) Stop() {
	if g.cancel != nil {
		g.cancel()
	}
}

// Degraded reports whether the guard is currently in degradation mode.
func (g *Guard) Degraded() bool { return g.degraded.Load() }

// Policy returns the most recently accepted policy.
func (g *Guard) Policy() Policy {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.policy
}

// trustedNow returns the best current estimate of authoritative time. Before the
// first successful renew it falls back to the wall clock; after, it advances the last
// signed `now` by the monotonic elapsed — immune to `date -s`.
func (g *Guard) trustedNow() int64 {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.anchorMono.IsZero() {
		return g.now().Unix()
	}
	return g.anchorTrusted + int64(g.now().Sub(g.anchorMono).Seconds())
}

func (g *Guard) trustedExpired() bool {
	g.mu.RLock()
	au := g.authorizedUntil
	g.mu.RUnlock()
	return au != 0 && g.trustedNow() > au
}

// loop is the background heartbeat. Each tick: renew (if configured), then evaluate
// whether to degrade. Server authority (protection-model §③): the control plane's
// signed verdict is the source of truth; local tampering surfaces at the next renew.
func (g *Guard) loop(ctx context.Context, cfg Config) {
	every := orDur(cfg.RenewEvery, time.Hour)
	grace := orDur(cfg.GraceWindow, 72*time.Hour)

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(jitter(every)):
		}

		if cfg.Renew != nil {
			if g.tryRenew(ctx, cfg) {
				continue // fresh, authorized → healthy
			}
		}
		// Renew absent/failed/revoked → degrade if expired (trusted) or past grace.
		if g.trustedExpired() {
			g.degraded.Store(true)
			continue
		}
		g.mu.RLock()
		stale := cfg.Renew != nil && g.now().Sub(g.lastRenewOK) > grace
		g.mu.RUnlock()
		if stale {
			g.degraded.Store(true)
		}
	}
}

// tryRenew does one nonce-challenge round-trip. Returns true if it produced a fresh,
// verified, non-revoked verdict (→ anchor trusted time, clear degradation).
func (g *Guard) tryRenew(ctx context.Context, cfg Config) bool {
	nonce, err := newNonce()
	if err != nil {
		return false
	}
	blob, err := cfg.Renew(ctx, nonce)
	if err != nil {
		return false
	}
	v, err := verifyVerdict(blob, cfg.PubKey, nonce) // sig + nonce (anti-replay)
	if err != nil {
		return false
	}
	if v.Revoked {
		g.degraded.Store(true)
		return false
	}
	g.mu.Lock()
	g.anchorTrusted = v.Now
	g.anchorMono = g.now()
	g.authorizedUntil = v.AuthorizedUntil
	g.lastRenewOK = g.now()
	g.mu.Unlock()
	g.degraded.Store(false)
	return true
}

// Middleware wraps an http.Handler. On the hot path it does a single atomic load;
// only when degraded does it probabilistically inject latency. Default-off,
// observable, remotely clearable — must never hard-fail a customer over a blip.
func (g *Guard) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if g.degraded.Load() && rand.Float64() < g.deg.Probability {
			d := g.deg.Min + time.Duration(rand.Int64N(int64(g.deg.Max-g.deg.Min)+1))
			select {
			case <-time.After(d):
			case <-r.Context().Done():
			}
		}
		next.ServeHTTP(w, r)
	})
}

func withDegradeDefaults(d DegradeConfig) DegradeConfig {
	if d.Probability == 0 {
		d.Probability = 0.02
	}
	if d.Min == 0 {
		d.Min = 3 * time.Second
	}
	if d.Max <= d.Min {
		d.Max = 10 * time.Second
	}
	return d
}

func orDur(v, def time.Duration) time.Duration {
	if v <= 0 {
		return def
	}
	return v
}

// jitter returns d ±20% so the renew callback isn't a clean periodic signal (harder
// to fingerprint/block; spreads load).
func jitter(d time.Duration) time.Duration {
	if d <= 0 {
		return d
	}
	delta := int64(d) / 5 // 20%
	return d + time.Duration(rand.Int64N(2*delta+1)-delta)
}
