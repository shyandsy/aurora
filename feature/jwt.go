package feature

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"

	"github.com/shyandsy/aurora/config"
	"github.com/shyandsy/aurora/contracts"
)

const (
	RedisKeyBlackAccessTokenPrefix  = "jwt:blacklist:accesstoken"
	RedisKeyBlackRefreshTokenPrefix = "jwt:blacklist:refreshtoken"
)

type JWTService interface {
	contracts.Features
	GenerateToken(userID int64, email string, features []string) (*TokenResponse, error)
	// GenerateTokenWithRoles 同 GenerateToken,但额外把 roles 写入 Claims.Roles(access + refresh 都带,
	// 且 RefreshToken 续期时保留)。roles 传 nil 等价于 GenerateToken。
	GenerateTokenWithRoles(userID int64, email string, features []string, roles []string) (*TokenResponse, error)
	ValidateToken(tokenString string) (*Claims, error)
	RefreshToken(tokenString string) (*TokenResponse, error)
	ExtractUserID(tokenString string) (int64, error)
	ValidateRefreshToken(tokenString string) (*Claims, error)
	Logout(accessToken, refreshToken string) error
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

// 令牌类型:区分 access / refresh。二者同密钥、同 Claims 结构,不加类型标记就无法区分,
// 会让 refresh 令牌被当 access 用去调业务接口——而"退出/踢设备"的吊销恰恰挂在 refresh 黑名单/RT 集上,
// 于是被绕过。加上类型标记后,ValidateToken 只认 access、ValidateRefreshToken 只认 refresh。
const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

type Claims struct {
	jwt.RegisteredClaims
	UserID   int64    `json:"user_id"`
	Email    string   `json:"email"`
	Features []string `json:"features"`
	// Roles 是签发时写入的角色名(可选)。用于「token 存 role、鉴权时再展开成 feature」的模式,
	// 避免把权限点摊平进 token 导致其随 API 数量膨胀。存名字(与 Features 一致、可读、改名由上层负责)。
	// 旧令牌无此字段 → 解析为空切片。
	Roles []string `json:"roles,omitempty"`
	// TokenType = access / refresh(旧令牌无此字段 → 校验按"非对应类型"拒,即强制重登)。
	TokenType string `json:"token_type,omitempty"`
}

type jwtFeature struct {
	Config       *config.JWTConfig
	RedisService RedisService `inject:""`
}

func NewJWTFeature() contracts.Features {
	cfg := &config.JWTConfig{}
	if err := config.ResolveConfig(cfg); err != nil {
		log.Fatalf("Failed to load JWT config: %v", err)
	}
	return &jwtFeature{Config: cfg}
}

// NewJWTService 从显式配置构造一个独立的 JWTService 引擎,供「同一进程需要用多把密钥
// 校验多个签发来源的令牌」的场景使用 —— 即一个进程里同时存在默认实例之外、校验**另一把
// 密钥**的额外引擎。
//
// 与默认实例(NewJWTFeature:从 env 读密钥、走 feature 生命周期并注册进 DI)不同,本引擎
// 不走 feature 生命周期、**不 AddFeature、不注册进 DI**,由调用方持有并直接调用。其校验
// 逻辑(签名 / token_type / 黑名单吊销)与默认实例完全一致,只是密钥不同 —— 调用方因此
// 无需自造一套平行的校验路径(那会随默认实例的演进而悄悄语义漂移)。
//
// 依赖解析沿用 house pattern(与其它 NewXxx(app) 一致):收 app、自己从容器解析依赖。
// redis 从容器解析(app.Find),而不是由调用方显式传入 —— 容器里必须已注册 RedisService:
// 黑名单吊销是安全关键,缺了会让「登出 / 踢设备」被绕过(校验对吊销检查是 fail-closed 的)。
// 因此须在 RedisService 已注册之后调用,典型是服务自身的 wiring 阶段(此刻 DI 已就绪)。
func NewJWTService(app contracts.App, cfg *config.JWTConfig) (JWTService, error) {
	if app == nil {
		return nil, fmt.Errorf("jwt: NewJWTService requires a non-nil app")
	}
	if cfg == nil {
		return nil, fmt.Errorf("jwt: NewJWTService requires a non-nil config")
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("jwt: config validation failed: %w", err)
	}
	var redis RedisService
	if err := app.Find(&redis); err != nil {
		return nil, fmt.Errorf("jwt: resolve RedisService from container failed (blacklist revocation is security-critical, a RedisService must be registered): %w", err)
	}
	if redis == nil {
		return nil, fmt.Errorf("jwt: RedisService resolved from container is nil (blacklist revocation is security-critical, a RedisService must be registered)")
	}
	return &jwtFeature{Config: cfg, RedisService: redis}, nil
}

func (f *jwtFeature) Name() string {
	return "jwt"
}

func (f *jwtFeature) Setup(app contracts.App) error {
	if err := f.Config.Validate(); err != nil {
		return fmt.Errorf("JWT configuration validation failed: %w", err)
	}

	if err := app.Resolve(f); err != nil {
		return fmt.Errorf("failed to resolve JWT feature dependencies: %w", err)
	}

	app.ProvideAs(f, (*JWTService)(nil))
	return nil
}

func (f *jwtFeature) Close() error {
	return nil
}

func (f *jwtFeature) GenerateToken(userID int64, email string, features []string) (*TokenResponse, error) {
	return f.GenerateTokenWithRoles(userID, email, features, nil)
}

func (f *jwtFeature) GenerateTokenWithRoles(userID int64, email string, features []string, roles []string) (*TokenResponse, error) {
	accessToken, err := f.generateAccessToken(userID, email, features, roles)
	if err != nil {
		return nil, err
	}
	refreshToken, err := f.generateRefreshToken(userID, email, features, roles)
	if err != nil {
		return nil, err
	}

	expiresIn := int64(f.Config.ExpireTime.Seconds())

	return &TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    expiresIn,
	}, nil
}

// newTokenID 生成 jti(JWT ID):128 bit 随机,十六进制。
//
// 为什么必须有:除了时间戳,claims 里其余字段(issuer/subject/user_id/email/features)对同一个
// 用户都是固定的,而 NumericDate 只精确到**秒**。没有 jti 的话,同一用户在同一秒内签发的两个
// token 会**字节完全相同** —— 它们在密码学意义上就是同一个凭据。后果:
//   - 按 token 拉黑的登出:登出一台设备,同秒登录的另一台也一起掉线;
//   - 上层按 token 哈希做的会话管理(如登录设备数限制)根本没法区分两台设备。
//
// 用 crypto/rand 而不是 math/rand:token 标识必须不可预测,且这里不引入新依赖。
func newTokenID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate token id: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

func (f *jwtFeature) generateAccessToken(userID int64, email string, features []string, roles []string) (string, error) {
	now := time.Now()
	expiresAt := now.Add(f.Config.ExpireTime)

	tokenID, err := newTokenID()
	if err != nil {
		return "", err
	}

	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        tokenID,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    f.Config.Issuer,
			Subject:   fmt.Sprintf("%d", userID),
		},
		UserID:    userID,
		Email:     email,
		Features:  features,
		Roles:     roles,
		TokenType: TokenTypeAccess,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(f.Config.Secret))
}

func (f *jwtFeature) ValidateToken(tokenString string) (*Claims, error) {
	claims, err := f.parseClaims(tokenString)
	if err != nil {
		return nil, err
	}
	// 只认 access:refresh 令牌(同密钥同结构)不能当 access 用去调业务接口,否则退出/踢设备被绕过。
	if claims.TokenType != TokenTypeAccess {
		return nil, fmt.Errorf("token is not an access token")
	}

	key := fmt.Sprintf("%s:%s", RedisKeyBlackAccessTokenPrefix, tokenString)
	blacklisted, berr := f.isBlacklisted(context.Background(), key)
	if berr != nil {
		// fail-closed: when the revocation store is unavailable, reject rather than accept.
		// Swallowing the error would let a single transient Redis hiccup resurrect a
		// logged-out / revoked (but unexpired) token — logout & revocation are load-bearing
		// security controls, so a failed revocation check must deny, not allow.
		return nil, fmt.Errorf("token revocation check unavailable: %w", berr)
	}
	if blacklisted {
		return nil, fmt.Errorf("token is blacklisted")
	}

	return claims, nil
}

func (f *jwtFeature) RefreshToken(tokenString string) (*TokenResponse, error) {
	claims, err := f.ValidateRefreshToken(tokenString)
	if err != nil {
		return nil, err
	}

	if claims.ExpiresAt == nil {
		return nil, errors.New("token has no expiration time")
	}
	expiresAt := claims.ExpiresAt.Time
	if time.Now().After(expiresAt) {
		return nil, errors.New("refresh token has expired")
	}

	accessToken, err := f.generateAccessToken(claims.UserID, claims.Email, claims.Features, claims.Roles)
	if err != nil {
		return nil, err
	}

	refreshToken, err := f.generateRefreshToken(claims.UserID, claims.Email, claims.Features, claims.Roles)
	if err != nil {
		return nil, err
	}

	expiresIn := int64(f.Config.ExpireTime.Seconds())

	return &TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    expiresIn,
	}, nil
}

func (f *jwtFeature) ExtractUserID(tokenString string) (int64, error) {
	claims, err := f.ValidateToken(tokenString)
	if err != nil {
		return 0, err
	}
	return claims.UserID, nil
}

func (f *jwtFeature) generateRefreshToken(userID int64, email string, features []string, roles []string) (string, error) {
	now := time.Now()
	refreshExpire := f.Config.RefreshExpireOrDefault()

	tokenID, err := newTokenID()
	if err != nil {
		return "", err
	}

	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        tokenID,
			ExpiresAt: jwt.NewNumericDate(now.Add(refreshExpire)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    f.Config.Issuer,
			Subject:   fmt.Sprintf("%d", userID),
		},
		UserID:    userID,
		Email:     email,
		Features:  features,
		Roles:     roles,
		TokenType: TokenTypeRefresh,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	refreshToken, err := token.SignedString([]byte(f.Config.Secret))
	if err != nil {
		return "", err
	}
	return refreshToken, nil
}

func (f *jwtFeature) ValidateRefreshToken(tokenString string) (*Claims, error) {
	claims, err := f.parseClaims(tokenString)
	if err != nil {
		return nil, err
	}
	// 只认 refresh:access 令牌不能拿来续期(对称加固)。
	if claims.TokenType != TokenTypeRefresh {
		return nil, fmt.Errorf("token is not a refresh token")
	}

	key := fmt.Sprintf("%s:%s", RedisKeyBlackRefreshTokenPrefix, tokenString)
	blacklisted, berr := f.isBlacklisted(context.Background(), key)
	if berr != nil {
		// fail-closed: revocation store unavailable → deny (see ValidateToken for rationale).
		return nil, fmt.Errorf("refresh token revocation check unavailable: %w", berr)
	}
	if blacklisted {
		return nil, fmt.Errorf("refresh token is blacklisted")
	}
	return claims, nil
}

func (f *jwtFeature) parseClaims(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(f.Config.Secret), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("invalid token")
}

func (f *jwtFeature) isBlacklisted(ctx context.Context, key string) (bool, error) {
	val, err := f.RedisService.Get(ctx, key)
	if err != nil {
		return false, err
	}
	if val == "" {
		return false, nil
	}
	return true, nil
}

func (f *jwtFeature) Logout(accessToken, refreshToken string) error {
	ctx := context.Background()

	process := func(token string, isRefresh bool) error {
		if token == "" {
			return nil
		}

		var claims *Claims
		var err error
		if isRefresh {
			claims, err = f.ValidateRefreshToken(token)
		} else {
			claims, err = f.ValidateToken(token)
		}
		if err != nil {
			return err
		}

		if claims.ExpiresAt == nil {
			return fmt.Errorf("token has no expiration")
		}
		expiresAt := claims.ExpiresAt.Time
		ttl := time.Until(expiresAt)
		if ttl <= 0 {
			return nil
		}

		var key string
		if isRefresh {
			key = fmt.Sprintf("%s:%s", RedisKeyBlackRefreshTokenPrefix, token)
		} else {
			key = fmt.Sprintf("%s:%s", RedisKeyBlackAccessTokenPrefix, token)
		}

		if err := f.RedisService.Set(ctx, key, "1", ttl); err != nil {
			return err
		}
		return nil
	}

	if err := process(accessToken, false); err != nil {
		return err
	}
	if err := process(refreshToken, true); err != nil {
		return err
	}
	return nil
}
