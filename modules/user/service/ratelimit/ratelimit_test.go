package ratelimit

import "testing"

// TestIndexKeys_DeriveFromNamespace 锁住本次行为修复:被锁列表索引 key **由 namespace 拼出**,
// 不再写死 "user"。否则 namespace≠user(引擎走 SERVICE_NAME)时,索引会和真正的锁落在不同前缀下、
// 后台列表读空/解锁失效(静默坏)。同时钉死 key 形状,防误改。
func TestIndexKeys_DeriveFromNamespace(t *testing.T) {
	cases := []struct {
		ns          string
		wantIP      string
		wantAccount string
	}{
		{"user", "rate_limit:user:index", "rate_limit:user:index:account"},
		{"acme", "rate_limit:acme:index", "rate_limit:acme:index:account"},
		{"feihang-prd", "rate_limit:feihang-prd:index", "rate_limit:feihang-prd:index:account"},
	}
	for _, c := range cases {
		if got := indexKeyIP(c.ns); got != c.wantIP {
			t.Errorf("indexKeyIP(%q) = %q, want %q", c.ns, got, c.wantIP)
		}
		if got := indexKeyAccount(c.ns); got != c.wantAccount {
			t.Errorf("indexKeyAccount(%q) = %q, want %q", c.ns, got, c.wantAccount)
		}
	}
}
