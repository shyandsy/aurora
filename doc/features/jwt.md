# JWT Feature

签发 / 校验 access+refresh 双 token,支持基于 Redis 的**黑名单登出**。依赖 Redis feature(黑名单存 Redis),所以注册顺序要在 redis 之后。

源码:[feature/jwt.go](../../feature/jwt.go)、[config/jwt.go](../../config/jwt.go)

---

## 创建 & 用法

```go
app.AddFeature(feature.NewJWTFeature())   // 必须在 NewRedisFeature() 之后
```

以接口 `JWTService` 注入([feature/jwt.go:63](../../feature/jwt.go#L63)):

```go
type authService struct {
    JWT feature.JWTService `inject:""`
}
```

## JWTService 接口

[feature/jwt.go:23](../../feature/jwt.go#L23):

```go
GenerateToken(userID int64, email string, features []string) (*TokenResponse, error)
ValidateToken(tokenString string) (*Claims, error)
RefreshToken(tokenString string) (*TokenResponse, error)
ExtractUserID(tokenString string) (int64, error)
ValidateRefreshToken(tokenString string) (*Claims, error)
Logout(accessToken, refreshToken string) error
```

- `TokenResponse`([feature/jwt.go:33](../../feature/jwt.go#L33)):`AccessToken` / `RefreshToken` / `ExpiresIn int64`(秒)。
  > ⚠️ `ExpiresIn` **始终只反映 access 的寿命**(`ExpireTime`),即使在 `RefreshToken` 的返回里也是如此,不代表 refresh 寿命。
- `Claims`([feature/jwt.go:39](../../feature/jwt.go#L39)):`jwt.RegisteredClaims` + `UserID` / `Email` / `Features`。

## 签发

`GenerateToken` 同时签 access + refresh([feature/jwt.go:80](../../feature/jwt.go#L80)):
- access 寿命 = `JWT_EXPIRE_TIME`;refresh 寿命 = `RefreshExpireOrDefault()`;
- 算法固定 `HS256`,密钥 `JWT_SECRET`;
- 写入 `Issuer=JWT_ISSUER`、`Subject=userID`、`ID=jti`。

### jti(每 token 唯一 ID)

[feature/jwt.go:108](../../feature/jwt.go#L108) `newTokenID`:16 字节 `crypto/rand` → hex。**为什么必须有**:claims 其余字段对同一用户固定、时间戳只精确到秒,没有 jti 时**同一秒签发的两个 token 字节完全相同** → 按 token 拉黑会误伤同秒登录的其它设备、按 token 哈希做的会话管理无法区分设备。

## 校验

- `ValidateToken`:`parseClaims` → 查 access 黑名单,命中报 `token is blacklisted`([feature/jwt.go:143](../../feature/jwt.go#L143))。
- `parseClaims`:强制 HMAC 签名方法,`token.Valid` 才通过([feature/jwt.go:242](../../feature/jwt.go#L242))。
- `RefreshToken`:先 `ValidateRefreshToken`(含黑名单)+ 显式过期检查,再重签 access+refresh([feature/jwt.go:157](../../feature/jwt.go#L157))。

## 黑名单登出(重点)

key 前缀([feature/jwt.go:18](../../feature/jwt.go#L18)):
- `jwt:blacklist:accesstoken:<完整token>`
- `jwt:blacklist:refreshtoken:<完整token>`

> key 用的是**完整 token 字符串**,不是 jti。

`Logout(access, refresh)`([feature/jwt.go:269](../../feature/jwt.go#L269)):
1. 分别校验两个 token、取其 `ExpiresAt`;
2. `ttl = 剩余寿命`;`ttl <= 0`(已过期)则跳过不写;
3. 否则 `Set(黑名单key, "1", ttl)` —— **黑名单条目 TTL 恰好等于 token 剩余寿命**,token 自然过期后条目自动清除,不留垃圾。

> ⚠️ **Redis 故障时放行**:`ValidateToken`/`ValidateRefreshToken` 查黑名单用 `ok, _ :=` **忽略了 Redis 错误**([feature/jwt.go:150](../../feature/jwt.go#L150))→ Redis 挂了时"已登出"的 token 会被放行。这是 fail-open 取舍(不因 Redis 抖动打挂鉴权),需知悉。

## 寿命配置

`RefreshExpireOrDefault()`([config/jwt.go:24](../../config/jwt.go#L24)):`JWT_REFRESH_EXPIRE_TIME > 0` 用配置值,否则**默认 = `JWT_EXPIRE_TIME × 2`**。

## 环境变量

| Env | 字段 | 必填 | 说明 |
|---|---|---|---|
| `JWT_SECRET` | Secret | ✅ | 签名密钥;**不能等于占位默认值**(生产防呆,[config/jwt.go:31](../../config/jwt.go#L31)) |
| `JWT_EXPIRE_TIME` | ExpireTime | ✅(>0) | access 寿命(duration) |
| `JWT_ISSUER` | Issuer | ✅ | 签发者 |
| `JWT_REFRESH_EXPIRE_TIME` | RefreshExpireTime | ❌(omitempty) | refresh 寿命;不设/0 → 默认 = access×2;不得为负 |
