package feature

import (
	"testing"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"

	"github.com/shyandsy/aurora/config"
)

func testJWT() *jwtFeature {
	return &jwtFeature{
		Config: &config.JWTConfig{
			Secret:            "test-secret",
			Issuer:            "aurora-test",
			ExpireTime:        30 * time.Minute,
			RefreshExpireTime: 24 * time.Hour,
		},
	}
}

// 同一用户在同一秒内签发的 token 必须各不相同。
//
// 为什么这条值得单独钉住:除时间戳外,claims 里其余字段(issuer/subject/user_id/email/features)
// 对同一用户都是固定的,而 NumericDate 只精确到**秒**。在加入 jti 之前,同一秒内连签两个 token
// 会得到**字节完全相同**的结果 —— 它们就是同一个凭据。真实后果:
//   - Logout 按 token 字符串拉黑:登出一台设备,同秒登录的另一台一起掉线;
//   - 上层按 token 哈希做的会话管理(如登录设备数限制)完全无法区分两台设备。
//
// 这个测试在加 jti 之前必然失败。
func TestGenerateToken_SameSecondTokensAreUnique(t *testing.T) {
	f := testJWT()

	const n = 20
	seenAccess := make(map[string]bool, n)
	seenRefresh := make(map[string]bool, n)

	// 连续快速签发:必然落在同一秒内,正是从前会碰撞的场景。
	for i := 0; i < n; i++ {
		resp, err := f.GenerateToken(42, "user@example.com", []string{})
		if err != nil {
			t.Fatalf("GenerateToken 失败: %v", err)
		}
		if seenAccess[resp.AccessToken] {
			t.Fatalf("第 %d 次签发的 access token 与之前某个完全相同 —— "+
				"同一秒登录的两台设备会共用一个凭据,登出/踢下线时会互相牵连", i+1)
		}
		if seenRefresh[resp.RefreshToken] {
			t.Fatalf("第 %d 次签发的 refresh token 与之前某个完全相同", i+1)
		}
		seenAccess[resp.AccessToken] = true
		seenRefresh[resp.RefreshToken] = true
	}
}

// access token 与 refresh token 之间也不能相同(它们 claims 只差过期时间,同样有碰撞风险)。
func TestGenerateToken_AccessAndRefreshDiffer(t *testing.T) {
	f := testJWT()
	resp, err := f.GenerateToken(42, "user@example.com", []string{})
	if err != nil {
		t.Fatalf("GenerateToken 失败: %v", err)
	}
	if resp.AccessToken == resp.RefreshToken {
		t.Fatal("access 与 refresh token 相同 —— 拿 access 就能当 refresh 用")
	}
}

// 签出来的 token 必须带 jti,且两次不同。
func TestGenerateToken_HasUniqueJTI(t *testing.T) {
	f := testJWT()

	first, err := f.GenerateToken(1, "a@example.com", nil)
	if err != nil {
		t.Fatalf("GenerateToken 失败: %v", err)
	}
	second, err := f.GenerateToken(1, "a@example.com", nil)
	if err != nil {
		t.Fatalf("GenerateToken 失败: %v", err)
	}

	c1 := mustParse(t, f, first.AccessToken)
	c2 := mustParse(t, f, second.AccessToken)

	if c1.ID == "" {
		t.Fatal("access token 没有 jti")
	}
	if c1.ID == c2.ID {
		t.Fatalf("两次签发的 jti 相同(%s)—— jti 没起到区分作用", c1.ID)
	}
}

// 加了 jti 不能破坏原有校验:token 照常验得过,业务字段照常读得出。
func TestGenerateToken_StillValidatesAndCarriesClaims(t *testing.T) {
	f := testJWT()

	resp, err := f.GenerateToken(7, "who@example.com", []string{"vip"})
	if err != nil {
		t.Fatalf("GenerateToken 失败: %v", err)
	}

	claims := mustParse(t, f, resp.AccessToken)
	if claims.UserID != 7 {
		t.Errorf("UserID = %d, 期望 7", claims.UserID)
	}
	if claims.Email != "who@example.com" {
		t.Errorf("Email = %q, 期望 who@example.com", claims.Email)
	}
	if len(claims.Features) != 1 || claims.Features[0] != "vip" {
		t.Errorf("Features = %v, 期望 [vip]", claims.Features)
	}
	if claims.Issuer != "aurora-test" {
		t.Errorf("Issuer = %q, 期望 aurora-test", claims.Issuer)
	}
}

// 老 token(没有 jti)必须继续验得过 —— 加 jti 只是"多写一个字段",不是强制要求它存在。
// 否则升级的那一刻,所有已签发的 token 会集体失效,把全部在线用户踢下线。
func TestParse_LegacyTokenWithoutJTI_StillValid(t *testing.T) {
	f := testJWT()

	// 手工造一个升级前格式的 token:没有 ID(jti)字段
	now := time.Now()
	legacy := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "aurora-test",
			Subject:   "9",
		},
		UserID: 9,
		Email:  "legacy@example.com",
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, legacy).SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatalf("构造老 token 失败: %v", err)
	}

	claims := mustParse(t, f, signed)
	if claims.UserID != 9 {
		t.Errorf("老 token 解出的 UserID = %d, 期望 9", claims.UserID)
	}
	if claims.ID != "" {
		t.Errorf("老 token 不该有 jti,却解出 %q", claims.ID)
	}
}

func mustParse(t *testing.T, f *jwtFeature, token string) *Claims {
	t.Helper()
	claims, err := f.parseClaims(token)
	if err != nil {
		t.Fatalf("解析 token 失败: %v", err)
	}
	return claims
}
