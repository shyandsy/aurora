package tokenguard

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"

	auroraFeature "github.com/shyandsy/aurora/feature"
)

// fakeRedis 是内存版 Redis(实现 tokenguard.Redis 的 Get/Set/Delete)。
// errPrefixes 命中的 key 让读写返回错误,用于验证 fail-close / 写失败上报。
type fakeRedis struct {
	data        map[string]string
	errPrefixes []string
}

func newFakeRedis() *fakeRedis { return &fakeRedis{data: map[string]string{}} }

func (f *fakeRedis) failFor(key string) bool {
	for _, p := range f.errPrefixes {
		if strings.HasPrefix(key, p) {
			return true
		}
	}
	return false
}

func (f *fakeRedis) Get(ctx context.Context, key string) (string, error) {
	if f.failFor(key) {
		return "", errors.New("backend down")
	}
	return f.data[key], nil
}

func (f *fakeRedis) Set(ctx context.Context, key string, value any, _ time.Duration) error {
	if f.failFor(key) {
		return errors.New("backend down")
	}
	f.data[key] = value.(string)
	return nil
}

func (f *fakeRedis) Delete(ctx context.Context, keys ...string) (int64, error) {
	var n int64
	for _, k := range keys {
		if f.failFor(k) {
			return 0, errors.New("backend down")
		}
		if _, ok := f.data[k]; ok {
			delete(f.data, k)
			n++
		}
	}
	return n, nil
}

const (
	jti = "jti-1"
	ip  = "1.2.3.4"
)

func newGuard(r *fakeRedis) Guard { return NewGuardWithRedis(r, 26*time.Hour) }

func claimsFor(features []string, exp time.Time) *auroraFeature.Claims {
	c := &auroraFeature.Claims{Features: features}
	c.ID = jti
	if !exp.IsZero() {
		c.ExpiresAt = jwt.NewNumericDate(exp)
	}
	return c
}

func webClaims() *auroraFeature.Claims {
	return claimsFor(SessionTags("web", true), time.Now().Add(time.Hour))
}
func appClaims() *auroraFeature.Claims {
	return claimsFor(SessionTags("app", false), time.Now().Add(time.Hour))
}

func mustBind(t *testing.T, g Guard) {
	t.Helper()
	if err := g.BindLoginIP(context.Background(), jti, ip, time.Hour); err != nil {
		t.Fatalf("BindLoginIP 意外失败: %v", err)
	}
}

func TestNewGuardWithRedisRejectsBadArgs(t *testing.T) {
	t.Run("denyTTL<=0 panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatalf("denyTTL<=0 应 panic")
			}
		}()
		_ = NewGuardWithRedis(newFakeRedis(), 0)
	})
	t.Run("nil redis panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatalf("nil redis 应 panic")
			}
		}()
		_ = NewGuardWithRedis(nil, time.Hour)
	})
}

func TestSessionTagsAndScope(t *testing.T) {
	web := SessionTags("web", true)
	if ScopeOf(web) != "web" {
		t.Fatalf("ScopeOf web")
	}
	app := SessionTags("app", false) // bindIP=false → 带 sess:noip
	if ScopeOf(app) != "app" {
		t.Fatalf("ScopeOf app")
	}
	pub := PublicFeatures(append([]string{"user.get"}, app...))
	for _, f := range pub {
		if strings.HasPrefix(f, "scope:") || f == "sess:noip" {
			t.Fatalf("PublicFeatures 未过滤内部标记: %v", pub)
		}
	}
	if len(pub) != 1 || pub[0] != "user.get" {
		t.Fatalf("PublicFeatures 应只剩业务 feature,got %v", pub)
	}
}

// TestBindLoginIPReportsInvalidArgs 建立不了保护必须报 error,绝不静默 no-op。
func TestBindLoginIPReportsInvalidArgs(t *testing.T) {
	ctx := context.Background()
	g := newGuard(newFakeRedis())
	if err := g.BindLoginIP(ctx, jti, "", time.Hour); err == nil {
		t.Fatalf("空 IP 应报错")
	}
	if err := g.BindLoginIP(ctx, jti, ip, 0); err == nil {
		t.Fatalf("ttl<=0 应报错")
	}
	r := newFakeRedis()
	r.errPrefixes = []string{"tokenip:"}
	if err := NewGuardWithRedis(r, time.Hour).BindLoginIP(ctx, jti, ip, time.Hour); err == nil {
		t.Fatalf("写失败应上报")
	}
}

func TestVerifySession(t *testing.T) {
	ctx := context.Background()

	t.Run("nil claims fail-close", func(t *testing.T) {
		if err := newGuard(newFakeRedis()).VerifySession(ctx, nil, ip); err != ErrBackendUnavailable {
			t.Fatalf("want ErrBackendUnavailable, got %v", err)
		}
	})

	t.Run("revoked jti rejected", func(t *testing.T) {
		r := newFakeRedis()
		g := newGuard(r)
		if err := g.Revoke(ctx, jti); err != nil {
			t.Fatalf("Revoke 意外失败: %v", err)
		}
		mustBind(t, g)
		if err := g.VerifySession(ctx, webClaims(), ip); err != ErrRevoked {
			t.Fatalf("want ErrRevoked, got %v", err)
		}
	})

	t.Run("web bound + matching IP passes", func(t *testing.T) {
		g := newGuard(newFakeRedis())
		mustBind(t, g)
		if err := g.VerifySession(ctx, webClaims(), ip); err != nil {
			t.Fatalf("want nil, got %v", err)
		}
	})

	t.Run("web IP mismatch rejected", func(t *testing.T) {
		g := newGuard(newFakeRedis())
		mustBind(t, g)
		if err := g.VerifySession(ctx, webClaims(), "9.9.9.9"); err != ErrIPMismatch {
			t.Fatalf("want ErrIPMismatch, got %v", err)
		}
	})

	t.Run("web not bound fail-close", func(t *testing.T) {
		if err := newGuard(newFakeRedis()).VerifySession(ctx, webClaims(), ip); err != ErrIPMismatch {
			t.Fatalf("want ErrIPMismatch, got %v", err)
		}
	})

	t.Run("app (sess:noip) skips IP even if unbound", func(t *testing.T) {
		if err := newGuard(newFakeRedis()).VerifySession(ctx, appClaims(), "any"); err != nil {
			t.Fatalf("want nil (IP skipped), got %v", err)
		}
	})

	t.Run("proxied skips IP (revocation only)", func(t *testing.T) {
		if err := newGuard(newFakeRedis()).VerifySessionProxied(ctx, webClaims()); err != nil {
			t.Fatalf("want nil, got %v", err)
		}
	})

	t.Run("deny backend error fail-close", func(t *testing.T) {
		r := newFakeRedis()
		r.errPrefixes = []string{"tokendeny:"}
		if err := newGuard(r).VerifySession(ctx, webClaims(), ip); err != ErrBackendUnavailable {
			t.Fatalf("want ErrBackendUnavailable, got %v", err)
		}
	})

	t.Run("ip backend error fail-close", func(t *testing.T) {
		r := newFakeRedis()
		r.errPrefixes = []string{"tokenip:"}
		if err := newGuard(r).VerifySession(ctx, webClaims(), ip); err != ErrBackendUnavailable {
			t.Fatalf("want ErrBackendUnavailable, got %v", err)
		}
	})
}

// TestRevokeClaims 吊销两半都做:删 IP 绑定 + 入黑名单。
func TestRevokeClaims(t *testing.T) {
	ctx := context.Background()
	r := newFakeRedis()
	g := newGuard(r)
	mustBind(t, g)

	if err := g.RevokeClaims(ctx, webClaims()); err != nil {
		t.Fatalf("RevokeClaims 意外失败: %v", err)
	}
	if err := g.VerifySession(ctx, webClaims(), ip); err != ErrRevoked {
		t.Fatalf("want ErrRevoked after Revoke, got %v", err)
	}
}

// TestRevokeReportsFailure 撤销半失败(黑名单没写进去)必须上报,不能静默吞。
func TestRevokeReportsFailure(t *testing.T) {
	ctx := context.Background()
	r := newFakeRedis()
	r.errPrefixes = []string{"tokendeny:"}
	if err := newGuard(r).Revoke(ctx, jti); err == nil {
		t.Fatalf("黑名单写失败应上报")
	}
}

// TestRevokeClaimsExpired 已过期 token 不写黑名单(无必要),仍做 Unbind 保持幂等。
func TestRevokeClaimsExpired(t *testing.T) {
	ctx := context.Background()
	r := newFakeRedis()
	expired := claimsFor(SessionTags("app", false), time.Now().Add(-time.Hour))
	if err := newGuard(r).RevokeClaims(ctx, expired); err != nil {
		t.Fatalf("过期 token 撤销不该报错: %v", err)
	}
	if v := r.data["tokendeny:"+jti]; v != "" {
		t.Fatalf("已过期 token 不应写入黑名单")
	}
}

// TestRevokeNoopArgs 空 jti / nil claims = 无事可做(nil,非失败)。
func TestRevokeNoopArgs(t *testing.T) {
	ctx := context.Background()
	g := newGuard(newFakeRedis())
	if err := g.Revoke(ctx, ""); err != nil {
		t.Fatalf("空 jti 应为 no-op(nil),got %v", err)
	}
	if err := g.RevokeClaims(ctx, nil); err != nil {
		t.Fatalf("nil claims 应为 no-op(nil),got %v", err)
	}
}
