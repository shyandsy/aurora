# JWT Feature

签发 / 校验 access+refresh 双 token,支持基于 Redis 的**黑名单登出**。依赖 Redis feature(黑名单存 Redis),所以注册顺序要在 redis 之后。

源码:[feature/jwt.go](../../feature/jwt.go)、[config/jwt.go](../../config/jwt.go)

---

## 创建 & 用法

`config.JWTConfig` 是 JWT 引擎的**输入形状**(Secret/Issuer/ExpireTime/RefreshExpireTime),**不带任何 env tag** —— 框架不读 env、也不规定 env 叫什么。"用哪把密钥、env 叫什么"完全归项目:项目自己组 `*config.JWTConfig` 显式传入。它既不进 `InitDefaultApp` 默认集,也不偷读 env。

**feature 实例**(走 feature 生命周期、注册进 DI,供 `inject:""` 解析):

```go
// 项目定义自己的、带 env tag 的配置结构体,用 aurora 通用 config.ResolveConfig 从 env 读出
// (duration 解析/必填校验都还在),再映射进 aurora 的 config.JWTConfig。env 名由项目决定。
var env struct {
    Secret        string        `env:"JWT_SECRET"`       // 单域;多域改 ADMIN_JWT_SECRET / CUSTOMER_JWT_SECRET
    Issuer        string        `env:"JWT_ISSUER"`
    ExpireTime    time.Duration `env:"JWT_EXPIRE_TIME"`
    RefreshExpire time.Duration `env:"JWT_REFRESH_EXPIRE_TIME,omitempty"`
}
if err := config.ResolveConfig(&env); err != nil {
    log.Fatalf("load JWT config: %v", err)
}
app.AddFeature(feature.NewJWTFeature(&config.JWTConfig{   // 必须在 NewRedisFeature() 之后
    Secret: env.Secret, Issuer: env.Issuer,
    ExpireTime: env.ExpireTime, RefreshExpireTime: env.RefreshExpire,
}))
```

> Setup 时这个 `*config.JWTConfig` 会连同 `JWTService` 一起注册进 DI —— 需要原始配置值的组件(如 `tokenguard` 按 refresh 寿命定 denyTTL、`modules/user` 的微服务 token 用 Secret/Issuer 自签)从 DI 取同一份,不必各自再读 env。因此**宿主须在这些组件之前** `AddFeature` 一个 JWT feature。
>
> 多域(同一进程要校验后台域 / 客户域两把密钥):每个域定义各自前缀(`ADMIN_` / `CUSTOMER_`)的配置结构体,组出各自的 `JWTConfig`。谁进 DI 当"默认"由 `NewJWTFeature` 决定,另一把用下面的额外实例。

以接口 `JWTService` 注入([feature/jwt.go](../../feature/jwt.go)):

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

当一个进程需要用多把密钥校验多个签发来源的令牌时,用 `NewJWTService(app, cfg)` 构造 feature 实例之外的额外引擎:传入显式 `cfg`,调用方自己持有并直接调用,它不走 feature 生命周期、**不 AddFeature、不注册进 DI**。它的校验逻辑(签名 / token_type / 黑名单吊销)与 feature 实例完全一致,只是密钥不同 —— 无需自造一套平行的校验路径。

依赖解析沿用 house pattern(与其它 `NewXxx(app)` 一致):收 `app`、自己从容器解析依赖。`RedisService` 从容器解析(`app.Find`),而不是显式传入 —— 因此须在 `RedisService` 已注册之后调用(典型是服务自身的 wiring 阶段,DI 已就绪)。

- `app` 为 nil、`cfg` 为 nil 或非法(`Validate()` 不过)→ 返回 error;
- 容器里解析不到 `RedisService`(未注册)→ 返回 error(黑名单吊销是安全关键,缺了会让「登出 / 踢设备」被绕过)。

## JWTService 接口

[feature/jwt.go](../../feature/jwt.go):

```go
GenerateToken(userID int64, email string, features []string) (*TokenResponse, error)
ValidateToken(tokenString string) (*Claims, error)
RefreshToken(tokenString string) (*TokenResponse, error)
ExtractUserID(tokenString string) (int64, error)
ValidateRefreshToken(tokenString string) (*Claims, error)
Logout(accessToken, refreshToken string) error
```

- `TokenResponse`([feature/jwt.go](../../feature/jwt.go)):`AccessToken` / `RefreshToken` / `ExpiresIn int64`(秒)。
  > ⚠️ `ExpiresIn` **始终只反映 access 的寿命**(`ExpireTime`),即使在 `RefreshToken` 的返回里也是如此,不代表 refresh 寿命。
- `Claims`([feature/jwt.go](../../feature/jwt.go)):`jwt.RegisteredClaims` + `UserID` / `Email` / `Features`。

## 签发

`GenerateToken` 同时签 access + refresh([feature/jwt.go](../../feature/jwt.go)):
- access 寿命 = `cfg.ExpireTime`;refresh 寿命 = `RefreshExpireOrDefault()`;
- 算法固定 `HS256`,密钥 = `cfg.Secret`;
- 写入 `Issuer=cfg.Issuer`、`Subject=userID`、`ID=jti`。

### jti(每 token 唯一 ID)

[feature/jwt.go](../../feature/jwt.go) `newTokenID`:16 字节 `crypto/rand` → hex。**为什么必须有**:claims 其余字段对同一用户固定、时间戳只精确到秒,没有 jti 时**同一秒签发的两个 token 字节完全相同** → 按 token 拉黑会误伤同秒登录的其它设备、按 token 哈希做的会话管理无法区分设备。

## 校验

- `ValidateToken`:`parseClaims` → 查 access 黑名单,命中报 `token is blacklisted`([feature/jwt.go](../../feature/jwt.go))。
- `parseClaims`:强制 HMAC 签名方法,`token.Valid` 才通过([feature/jwt.go](../../feature/jwt.go))。
- `RefreshToken`:先 `ValidateRefreshToken`(含黑名单)+ 显式过期检查,再重签 access+refresh([feature/jwt.go](../../feature/jwt.go))。

## 黑名单登出(重点)

key 前缀([feature/jwt.go](../../feature/jwt.go)):
- `jwt:blacklist:accesstoken:<完整token>`
- `jwt:blacklist:refreshtoken:<完整token>`

> key 用的是**完整 token 字符串**,不是 jti。

`Logout(access, refresh)`([feature/jwt.go](../../feature/jwt.go)):
1. 分别校验两个 token、取其 `ExpiresAt`;
2. `ttl = 剩余寿命`;`ttl <= 0`(已过期)则跳过不写;
3. 否则 `Set(黑名单key, "1", ttl)` —— **黑名单条目 TTL 恰好等于 token 剩余寿命**,token 自然过期后条目自动清除,不留垃圾。

> ⚠️ **Redis 故障时拒绝(fail-closed)**:`ValidateToken`/`ValidateRefreshToken` 查黑名单时若后端报错,**一律拒绝放行**(返回 `revocation check unavailable`)。否则一次瞬时 Redis 抖动就会让"已登出 / 已吊销(但未过期)"的 token 复活 —— 登出 / 吊销是承重安全控制,校验失败必须拒,不能放行。

## 寿命配置

`RefreshExpireOrDefault()`([config/jwt.go](../../config/jwt.go)):`cfg.RefreshExpireTime > 0` 用配置值,否则**默认 = `cfg.ExpireTime × 2`**。

## 配置字段与约定 env 名

`config.JWTConfig` 的字段 + `Validate()` 的约束如下。**env 名不是框架定的**(结构体无 tag),下表第一列只是**单域项目的约定名**,供参考 —— 多域项目按自己的前缀(`ADMIN_` / `CUSTOMER_`…)命名。

| 字段 | 约定 env 名(单域) | 必填 | 说明 |
|---|---|---|---|
| Secret | `JWT_SECRET` | ✅ | 签名密钥;**不能等于占位默认值**(生产防呆,[config/jwt.go](../../config/jwt.go)) |
| ExpireTime | `JWT_EXPIRE_TIME` | ✅(>0) | access 寿命(duration) |
| Issuer | `JWT_ISSUER` | ✅ | 签发者 |
| RefreshExpireTime | `JWT_REFRESH_EXPIRE_TIME` | ❌ | refresh 寿命;不设/0 → 默认 = access×2;不得为负 |
