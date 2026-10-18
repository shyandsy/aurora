# JWT Feature

签发 / 校验 access+refresh 双 token,支持基于 Redis 的**黑名单登出**。依赖 Redis feature(黑名单存 Redis),所以注册顺序要在 redis 之后。

源码:[feature/jwt.go](../../feature/jwt.go)、[config/jwt.go](../../config/jwt.go)

---

## 创建 & 用法

**默认实例**(走 feature 生命周期、从 env 读密钥、注册进 DI):

```go
app.AddFeature(feature.NewJWTFeature())   // 必须在 NewRedisFeature() 之后
```

以接口 `JWTService` 注入([feature/jwt.go:63](../../feature/jwt.go#L63)):

```go
type authService struct {
    JWT feature.JWTService `inject:""`
}
```

**额外实例**(同一进程内校验**另一把密钥**,调用方自持、不注册进 DI):

```go
cfg := &config.JWTConfig{Secret: "...", Issuer: "...", ExpireTime: 30 * time.Minute}
svc, err := feature.NewJWTService(app, cfg)   // redis 从容器解析,须在 RedisService 已注册后调用
```

当一个进程需要用多把密钥校验多个签发来源的令牌时,用 `NewJWTService(app, cfg)` 构造默认实例之外的额外引擎:传入显式 `cfg`,调用方自己持有并直接调用,它不走 feature 生命周期、**不 AddFeature、不注册进 DI**。它的校验逻辑(签名 / token_type / 黑名单吊销)与默认实例完全一致,只是密钥不同 —— 无需自造一套平行的校验路径。

依赖解析沿用 house pattern(与其它 `NewXxx(app)` 一致):收 `app`、自己从容器解析依赖。`RedisService` 从容器解析(`app.Find`),而不是显式传入 —— 因此须在 `RedisService` 已注册之后调用(典型是服务自身的 wiring 阶段,DI 已就绪)。

- `app` 为 nil、`cfg` 为 nil 或非法(`Validate()` 不过)→ 返回 error;
- 容器里解析不到 `RedisService`(未注册)→ 返回 error(黑名单吊销是安全关键,缺了会让「登出 / 踢设备」被绕过)。

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

> ⚠️ **Redis 故障时拒绝(fail-closed)**:`ValidateToken`/`ValidateRefreshToken` 查黑名单时若后端报错,**一律拒绝放行**(返回 `revocation check unavailable`)。否则一次瞬时 Redis 抖动就会让"已登出 / 已吊销(但未过期)"的 token 复活 —— 登出 / 吊销是承重安全控制,校验失败必须拒,不能放行。

## 寿命配置

`RefreshExpireOrDefault()`([config/jwt.go:24](../../config/jwt.go#L24)):`JWT_REFRESH_EXPIRE_TIME > 0` 用配置值,否则**默认 = `JWT_EXPIRE_TIME × 2`**。

## 环境变量

| Env | 字段 | 必填 | 说明 |
|---|---|---|---|
| `JWT_SECRET` | Secret | ✅ | 签名密钥;**不能等于占位默认值**(生产防呆,[config/jwt.go:31](../../config/jwt.go#L31)) |
| `JWT_EXPIRE_TIME` | ExpireTime | ✅(>0) | access 寿命(duration) |
| `JWT_ISSUER` | Issuer | ✅ | 签发者 |
| `JWT_REFRESH_EXPIRE_TIME` | RefreshExpireTime | ❌(omitempty) | refresh 寿命;不设/0 → 默认 = access×2;不得为负 |
