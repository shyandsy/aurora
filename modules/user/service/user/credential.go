package user

import (
	"os"

	"github.com/shyandsy/aurora/encryption"
)

// totpKeyEnv 凭据加密密钥的环境变量名(全词、不缩写)。
// TOTP 密钥、2FA 挑战 token 等敏感数据以 AES-256-GCM(common/secret)加解密,密钥由本 env 提供。
// 默认 "USER_GOOGLE_TOTP_AUTH_KEY";装配层(usercenter)可用 SetCredentialKeyEnv 覆盖,
// 使「每项目 env 名」成为单一事实来源(启动校验与此处运行时读同一份)。
var totpKeyEnv = "USER_GOOGLE_TOTP_AUTH_KEY"

// SetCredentialKeyEnv 设置凭据密钥的环境变量名(供模块装配层按 Config 注入)。
// 传空串则保持当前值不变(默认 "USER_GOOGLE_TOTP_AUTH_KEY")——避免误清空。
func SetCredentialKeyEnv(env string) {
	if env != "" {
		totpKeyEnv = env
	}
}

// ValidateCredentialKey 校验凭据密钥当前是否有效(非空且可派生)。
// 供装配层在启动期做「未配置密钥即拒绝启动」的强制校验;返回 nil 表示可用。
func ValidateCredentialKey() error {
	_, err := credentialKey()
	return err
}

// credentialKey 从 env 派生 32 字节 AES-256 密钥。空密钥返回 ErrKeyMissing(绝不回落占位密钥)。
func credentialKey() ([]byte, error) {
	return encryption.DeriveKey(os.Getenv(totpKeyEnv))
}

// encryptCredential 用凭据密钥加密明文(空明文原样返回空串)。
func encryptCredential(plaintext string) (string, error) {
	key, err := credentialKey()
	if err != nil {
		return "", err
	}
	return encryption.EncryptSecret(key, plaintext)
}

// decryptCredential 用凭据密钥解密密文(空串原样返回空串)。
func decryptCredential(ciphertext string) (string, error) {
	key, err := credentialKey()
	if err != nil {
		return "", err
	}
	return encryption.DecryptSecret(key, ciphertext)
}
