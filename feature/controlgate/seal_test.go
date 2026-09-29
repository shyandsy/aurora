package controlgate

import (
	"bytes"
	"crypto/ed25519"
	crand "crypto/rand"
	"testing"
)

// TestSealRoundTripAndOpaque 加密落盘:能解回原文;磁盘密文不含明文字段;换 key 解不开。
func TestSealRoundTripAndOpaque(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(crand.Reader)
	key := sealKey(pub)

	plain := []byte(`{"b":"...","s":"...","authorizedUntil":123,"revoked":false}`)
	enc, err := sealEncrypt(key, plain)
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}

	// 磁盘密文不应出现明文里的敏感字段(混淆:casual base64/cat 看不出是授权令牌)。
	if bytes.Contains(enc, []byte("authorizedUntil")) {
		t.Error("密文里不应出现明文字段 authorizedUntil")
	}

	// 解回等于原文。
	got, err := sealDecrypt(key, enc)
	if err != nil {
		t.Fatalf("解密失败: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Error("解密结果与原文不一致")
	}

	// 换一把 key 解不开(GCM 认证失败)。
	pub2, _, _ := ed25519.GenerateKey(crand.Reader)
	if _, err := sealDecrypt(sealKey(pub2), enc); err == nil {
		t.Error("换 key 应解不开")
	}

	// sealKey 对同一 pub 确定性一致。
	if !bytes.Equal(key, sealKey(pub)) {
		t.Error("sealKey 对同一 pub 应确定性一致")
	}
}
