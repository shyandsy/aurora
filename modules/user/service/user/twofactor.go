package user

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// ==================== pending token(2FA 登录挑战 token）====================
//
// 密码验过、但用户开了 2FA 时,第一步不发正式 JWT,发这个短时挑战 token。
// 用 AES-256-GCM(encryption 包)加密载荷:只有服务端能造/能读、篡改即失败,
// **且不是 JWT** —— 普通鉴权中间件(校验 JWT)不会把它当 access token 放行。第二步凭它 + 验证码换正式 JWT。

const (
	pendingPurpose = "2fa-login"
	pendingTTL     = 5 * time.Minute
)

var errBadPendingToken = errors.New("invalid or expired pending token")

type pendingClaims struct {
	UserID  int64  `json:"uid"`
	Purpose string `json:"pp"`
	Exp     int64  `json:"exp"`
}

// makePendingToken 生成短时单用途挑战 token。
func makePendingToken(userID int64) (string, error) {
	b, err := json.Marshal(pendingClaims{
		UserID:  userID,
		Purpose: pendingPurpose,
		Exp:     time.Now().Add(pendingTTL).Unix(),
	})
	if err != nil {
		return "", err
	}
	return encryptCredential(string(b))
}

// parsePendingToken 校验并解出 userID;非本用途/过期/篡改一律拒。
func parsePendingToken(token string) (int64, error) {
	raw, err := decryptCredential(strings.TrimSpace(token))
	if err != nil || raw == "" {
		return 0, errBadPendingToken
	}
	var c pendingClaims
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		return 0, errBadPendingToken
	}
	if c.Purpose != pendingPurpose || time.Now().Unix() > c.Exp {
		return 0, errBadPendingToken
	}
	return c.UserID, nil
}

// ==================== recovery codes（一次性备用码）====================
//
// 绑定时生成 N 个,**明文只在生成时一次性返回给用户**;落库存 sha256 哈希(不存明文)。
// 手机丢了可用备用码登录;用过即作废。

const recoveryCount = 10

type recoveryEntry struct {
	Hash string `json:"h"`
	Used bool   `json:"u"`
}

// generateRecoveryCodes 返回明文码(一次性给用户)+ 哈希 JSON(落库)。
func generateRecoveryCodes() (plain []string, stored string, err error) {
	entries := make([]recoveryEntry, 0, recoveryCount)
	plain = make([]string, 0, recoveryCount)
	for i := 0; i < recoveryCount; i++ {
		buf := make([]byte, 5) // 10 位 hex
		if _, err = rand.Read(buf); err != nil {
			return nil, "", err
		}
		code := hex.EncodeToString(buf)
		plain = append(plain, code)
		entries = append(entries, recoveryEntry{Hash: hashRecovery(code)})
	}
	b, err := json.Marshal(entries)
	if err != nil {
		return nil, "", err
	}
	return plain, string(b), nil
}

// consumeRecoveryCode 若 code 命中未用过的备用码 → 标记用过,返回新的哈希 JSON + true;否则原样 + false。
func consumeRecoveryCode(stored, code string) (newStored string, ok bool) {
	var entries []recoveryEntry
	if stored == "" || json.Unmarshal([]byte(stored), &entries) != nil {
		return stored, false
	}
	h := hashRecovery(code)
	for i := range entries {
		if !entries[i].Used && entries[i].Hash == h {
			entries[i].Used = true
			b, _ := json.Marshal(entries)
			return string(b), true
		}
	}
	return stored, false
}

// hashRecovery 归一(去空白/连字符、小写)后 sha256,保证生成与校验一致。
func hashRecovery(code string) string {
	norm := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(code, "-", "")))
	sum := sha256.Sum256([]byte(norm))
	return hex.EncodeToString(sum[:])
}
