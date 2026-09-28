package encryption

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"
)

func TestDeriveKey(t *testing.T) {
	// 空 → ErrKeyMissing(绝不回落占位密钥)
	if _, err := DeriveKey(""); err != ErrKeyMissing {
		t.Fatalf("空密钥应返回 ErrKeyMissing,got %v", err)
	}

	// 合法 base64 且解码恰 32 字节 → 直接用
	raw32 := make([]byte, 32)
	for i := range raw32 {
		raw32[i] = byte(i)
	}
	b64 := base64.StdEncoding.EncodeToString(raw32)
	k, err := DeriveKey(b64)
	if err != nil || len(k) != 32 || string(k) != string(raw32) {
		t.Fatalf("base64-32 应原样用作 key,got len=%d err=%v", len(k), err)
	}

	// 任意口令串 → sha256 派生 32 字节(确定性)
	k1, _ := DeriveKey("some-passphrase")
	want := sha256.Sum256([]byte("some-passphrase"))
	if len(k1) != 32 || string(k1) != string(want[:]) {
		t.Fatal("任意口令应 sha256 派生 32 字节")
	}
	// 确定性:同口令两次相同
	k2, _ := DeriveKey("some-passphrase")
	if string(k1) != string(k2) {
		t.Fatal("同口令派生应确定")
	}

	// base64 合法但解码非 32 字节 → 走 sha256(不当 32 字节 key 用)
	short := base64.StdEncoding.EncodeToString([]byte("short"))
	ks, _ := DeriveKey(short)
	if len(ks) != 32 {
		t.Fatal("非 32 字节 base64 应 sha256 派生 32 字节")
	}
}

func keyFor(t *testing.T, raw string) []byte {
	t.Helper()
	k, err := DeriveKey(raw)
	if err != nil {
		t.Fatalf("DeriveKey: %v", err)
	}
	return k
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	key := keyFor(t, "test-key")
	for _, pt := range []string{"a", "TOTP-secret-ABCDEF", "含中文与符号!@#", strings.Repeat("x", 4096)} {
		ct, err := EncryptSecret(key, pt)
		if err != nil {
			t.Fatalf("encrypt %q: %v", pt, err)
		}
		if ct == pt {
			t.Fatalf("密文不应等于明文:%q", pt)
		}
		got, err := DecryptSecret(key, ct)
		if err != nil || got != pt {
			t.Fatalf("往返失败:want %q got %q err %v", pt, got, err)
		}
	}
}

func TestEmpty(t *testing.T) {
	key := keyFor(t, "test-key")
	if ct, err := EncryptSecret(key, ""); ct != "" || err != nil {
		t.Fatalf("空明文应返回空串,got %q %v", ct, err)
	}
	if pt, err := DecryptSecret(key, ""); pt != "" || err != nil {
		t.Fatalf("空密文应返回空串,got %q %v", pt, err)
	}
}

func TestDecrypt_Tampered(t *testing.T) {
	key := keyFor(t, "test-key")
	ct, _ := EncryptSecret(key, "secret")
	raw, _ := base64.StdEncoding.DecodeString(ct)
	raw[len(raw)-1] ^= 0xFF // 翻转最后一字节(密文体,GCM 认证应失败)
	tampered := base64.StdEncoding.EncodeToString(raw)
	if _, err := DecryptSecret(key, tampered); err == nil {
		t.Fatal("篡改密文应解密失败(GCM 认证)")
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	ct, _ := EncryptSecret(keyFor(t, "key-A"), "secret")
	if _, err := DecryptSecret(keyFor(t, "key-B"), ct); err == nil {
		t.Fatal("错密钥应解密失败")
	}
}

func TestDecrypt_TooShort(t *testing.T) {
	key := keyFor(t, "test-key")
	short := base64.StdEncoding.EncodeToString([]byte("x")) // 短于 nonce
	if _, err := DecryptSecret(key, short); err == nil {
		t.Fatal("过短密文应报错")
	}
	if _, err := DecryptSecret(key, "not-base64!!!"); err == nil {
		t.Fatal("非 base64 应报错")
	}
}

func TestEncrypt_NonceUnique(t *testing.T) {
	key := keyFor(t, "test-key")
	a, _ := EncryptSecret(key, "same")
	b, _ := EncryptSecret(key, "same")
	if a == b {
		t.Fatal("同明文两次加密应因随机 nonce 而不同")
	}
}
