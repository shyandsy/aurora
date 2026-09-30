package controlgate

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	crand "crypto/rand"
	"crypto/sha256"
	"errors"
)

// sealKeyLabel 派生持久化加密密钥的域分隔标签。prd 构建 garble -literals 会把它一并混掉。
const sealKeyLabel = "controlgate/state-seal/v1"

// sealKey 从项目公钥派生落盘用的对称密钥(AES-256):sha256(label ‖ pub)。
// 复用已有的 pubkey、不引入新秘密,且 key 不是可 grep 的字面量。
//
// ⚠️ 这是**混淆,不是保密**:甲方是 root,逆向二进制能得到 pub + label 从而重算出 key、或直接
// dump 内存拿明文。目的只是补上"磁盘上留个明文授权令牌"这个天窗,让 casual 翻盘/`base64 -d` 看到乱码——
// 与 garble / DCE 日志 / 中性 metrics 同一层减速带,不假装是保险柜。
func sealKey(pub ed25519.PublicKey) []byte {
	h := sha256.Sum256(append([]byte(sealKeyLabel), pub...))
	return h[:]
}

// sealEncrypt AES-256-GCM 加密,输出 nonce ‖ 密文‖tag。
func sealEncrypt(key, plain []byte) ([]byte, error) {
	gcm, err := newSealGCM(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := crand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plain, nil), nil
}

// sealDecrypt 解密 sealEncrypt 的输出;被改动 / 换 key / 非本格式 → 返回 error(调用方当无有效持久化)。
func sealDecrypt(key, enc []byte) ([]byte, error) {
	gcm, err := newSealGCM(key)
	if err != nil {
		return nil, err
	}
	ns := gcm.NonceSize()
	if len(enc) < ns {
		return nil, errors.New("seal: 密文过短")
	}
	return gcm.Open(nil, enc[:ns], enc[ns:], nil)
}

func newSealGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
