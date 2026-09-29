package user

import (
	"testing"

	"github.com/shyandsy/aurora/feature/loginguard"
)

// TestResolveLoginPolicyProvider_DefaultWhenNil 留空 → 内置 StaticPolicy 硬锁默认(现行行为,零回归)。
func TestResolveLoginPolicyProvider_DefaultWhenNil(t *testing.T) {
	got := resolveLoginPolicyProvider(Config{}).LoginPolicy()
	if got.AcctLockSeconds != defaultAcctLockSeconds {
		t.Fatalf("留空应回落内置硬锁 AcctLockSeconds=%d, got %d", defaultAcctLockSeconds, got.AcctLockSeconds)
	}
	// 其余字段 = DefaultPolicy(只把账号锁时长拨成硬锁)。
	def := loginguard.DefaultPolicy()
	if got.IPFailLimit != def.IPFailLimit || got.IPWindowSeconds != def.IPWindowSeconds ||
		got.IPLockSeconds != def.IPLockSeconds || got.IPPerHour != def.IPPerHour || got.AcctFailLimit != def.AcctFailLimit {
		t.Fatalf("留空应基于 DefaultPolicy(仅改 AcctLockSeconds),got %+v want base %+v", got, def)
	}
}

// TestResolveLoginPolicyProvider_UsesInjected 传值 → 原样用宿主注入的 provider(运行时可调的口子)。
func TestResolveLoginPolicyProvider_UsesInjected(t *testing.T) {
	custom := loginguard.LoginPolicy{
		IPFailLimit: 99, IPWindowSeconds: 300, IPLockSeconds: 900,
		IPPerHour: 60, AcctFailLimit: 10, AcctLockSeconds: 0, // 0 = 只计数,和默认硬锁 900 明显不同
	}
	got := resolveLoginPolicyProvider(Config{LoginPolicyProvider: loginguard.StaticPolicy(custom)}).LoginPolicy()
	if got.IPFailLimit != 99 || got.AcctLockSeconds != 0 {
		t.Fatalf("应原样用注入的 provider,got %+v", got)
	}
}
