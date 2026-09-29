// Package encryption 提供跨服务共享的服务级凭据加解密助手(AES-256-GCM)。
//
// 用途:把落库的敏感凭据(如 TOTP 密钥、第三方服务 token、私钥、SSH 凭据等)加密存储,解密时还原。
//
// 本包是**纯算法助手**:加解密密钥由调用方作为参数传入,**本包不读取任何环境变量或配置**——
// 密钥从何而来(env / 密钥管理服务)由消费服务自行决定并注入。这样同一份代码可被任意服务复用,
// 彼此互不依赖;互相加解密的多个服务必须使用**同一把密钥**,否则一方加密的密文另一方解不开。
package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
)

// ErrKeyMissing 传入的原始密钥串为空。
//
// 绝不回落到内置占位密钥:占位密钥人人可知,落库密文形同明文,
// 一旦持有 root 的人拿到 DB 即可解出全部服务级凭据。因此空密钥直接报错,
// 由消费服务在启动期 fail-fast(密钥应来自配置,不入库、不硬编码)。
var ErrKeyMissing = errors.New("credential encryption key is empty")

// DeriveKey 从任意形态的原始密钥串派生出 32 字节 AES-256 密钥:
//   - raw 为空 → ErrKeyMissing(绝不回落占位密钥);
//   - raw 是合法 base64 且解码后恰为 32 字节 → 直接使用;
//   - 否则对原始串做 sha256 派生 32 字节(兼容任意口令形态的 key,稳定可用)。
//
// 消费服务读取自己的密钥 env(如 USER_CREDENTIAL_ENCRYPTION_KEY),经此派生后传给
// EncryptSecret / DecryptSecret。
func DeriveKey(raw string) ([]byte, error) {
	if raw == "" {
		return nil, ErrKeyMissing
	}
	if b, err := base64.StdEncoding.DecodeString(raw); err == nil && len(b) == 32 {
		return b, nil
	}
	sum := sha256.Sum256([]byte(raw))
	return sum[:], nil
}

// EncryptSecret 用给定 32 字节密钥做 AES-256-GCM 加密,返回 base64 密文;空明文原样返回空串。
func EncryptSecret(key []byte, plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptSecret 用给定 32 字节密钥解密 EncryptSecret 的产物;空串原样返回空串。
func DecryptSecret(key []byte, ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(data) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}
	nonce, enc := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, enc, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
