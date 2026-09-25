# tokenguard — token 会话有效性内核(aurora feature,防护三件套之「登录后」)

一个 aurora feature,收口「一枚已签发的 JWT 现在还算不算有效会话」这一领域:撤销(jti 黑名单)、
IP 绑定(防盗用重放)、token 自描述作用域约定。配合 `aurora/feature/jwt` 使用——jwt 管「密码学上
有效吗」,tokenguard 管「这枚有效 token 在会话层面还该被接受吗」(登出了没、被踢了没、异地重放没)。

## 对外只有一个契约:`Guard`

**opt-in feature**(不在 bootstrap 默认集——不像 jwt/redis 那么普适,纯 worker/非鉴权服务不必带)。
鉴权服务显式注册一行(须在 redis + jwt 之后),再注入即用:

```go
// service main / providers(须在 redis + jwt 之后)。denyTTL 由调用方显式传,通常 = jwt refresh 寿命:
var jc config.JWTConfig
_ = config.ResolveConfig(&jc)
app.AddFeature(tokenguard.NewTokenGuardFeature(jc.RefreshExpireOrDefault()))

type userService struct {
    TokenGuard tokenguard.Guard `inject:""`
}
```

```go
type Guard interface {
    VerifySession(ctx, claims, clientIP) error       // 浏览器面:撤销 + IP 绑定
    VerifySessionProxied(ctx, claims) error          // 被代理/内部面:只查撤销,不校 IP
    BindLoginIP(ctx, jti, ip, ttl) error             // 签发时绑定登录 IP
    RevokeClaims(ctx, claims) error                  // 有 claims:精确 TTL(轮换热路径)
    Revoke(ctx, jti) error                           // 只有 jti:denyTTL 兜底(会话表撤销)
}
```

外加纯 scope 约定助手(无需构造,签发/网关用):

```go
tokenguard.SessionTags(scope string, bindIP bool) []string  // 生成 scope:/sess:noip 标记
tokenguard.ScopeOf(features []string) string                // 解析作用域名(路由网关施策)
tokenguard.PublicFeatures(features []string) []string       // 过滤内部标记,得可回给客户端的 features
```

**实现全在 `internal/`**(`denylist` jti 黑名单、`loginip` IP 绑定、`scope` 标记约定),Go 编译器强制
外部无法 import —— 内部随便重构不破坏契约。对外就 `Guard` + 三个纯助手 + feature。

## 装配:一行注册,denyTTL 显式传

鉴权服务 `app.AddFeature(tokenguard.NewTokenGuardFeature(denyTTL))`(在 redis + jwt 之后)。feature 的
`Setup` 注入 RedisService、`ProvideAs` 出 `Guard`。除这一行外,服务什么都不用写,注入就有。

`Setup` 的任何失败(redis 未注入 / denyTTL 非法)都 `return error` → aurora `AddFeature` `log.Fatalf`
**启动即退出**,绝不静默降级成"全站 401"。

**denyTTL 由调用方显式传**(不写死从 jwt 读):它是黑名单条目在"拿不到 token 精确到期"时(仅 `Revoke(jti)`
兜底路径,`RevokeClaims` 永远按 `claims.ExpiresAt` 精确推)的存活上界,**必须 >= 本服务最长 token 寿命**,
否则不绑 IP 的 token(黑名单是其唯一吊销手段)会在到点后"复活"。通常 = jwt refresh 寿命,直接传:

```go
var jc config.JWTConfig
_ = config.ResolveConfig(&jc)
app.AddFeature(tokenguard.NewTokenGuardFeature(jc.RefreshExpireOrDefault()))
```

但由调用方传、而非 feature 偷偷读 jwt,是为了不把"最长 token 寿命 == jwt refresh"这个假设焊死——某服务若
绕过 jwt feature 自签更长寿 token,直接传那个更大的值即可,不必改框架。

### 薄服务(无 aurora DI)怎么用同一个 Guard

没跑完整 aurora app 的薄 BFF,自带一个几方法的 redis 适配器,拿同一个 `Guard`:

```go
type Redis interface {          // Guard 只需要这三个方法
    Get(ctx, key) (string, error)
    Set(ctx, key, value, ttl) error
    Delete(ctx, keys...) (int64, error)
}
g := tokenguard.NewGuardWithRedis(myRedisAdapter, denyTTL)
```

## 设计原则

**1. fail-close 读 + fail-loud 写,绝不静默 no-op。** 读(VerifySession)任何 Redis 读失败一律拒(401);
写(BindLoginIP/Revoke)一律返回 error —— 建立/撤销一半防护若静默跳过,调用方会以为防护生效、实际没有
(web token 该绑没绑 → 登录成功却每请求 401 的静默故障)。建立不了保护就报错让调用方拒绝签发,撤销没
成功就报错别假装登出成功。

**2. 吊销必须两半都做。** 删 IP 绑定(web 靠它即时失效)+ 入 jti 黑名单(不绑 IP 的 App 靠它)。`Revoke`
两半都尽力做完、`errors.Join` 合并上报,半成功也不吞(对 App token,黑名单没写=没真吊销)。

**3. 黑名单 TTL 覆盖 token 完整剩余寿命,且不进调用签名。** `RevokeClaims` 从 `claims.ExpiresAt` 精确推
(自然过期即清,轮换热路径不留垃圾);`Revoke` 用构造时 denyTTL 兜底。调用点既不传 redis、也不传 ttl。

**4. 零客户端类型判断。** 无 `if isApp`。绑不绑 IP 依 token 自带 `sess:noip` 标记 + 调用方选
`VerifySession`(校 IP)/ `VerifySessionProxied`(不校),没有 bool 标志参数。

## 支持多种服务(user / customer …)——纯配置

tokenguard 无任何 user-vs-customer 判断,两类服务各起各的 Guard,差异全是配置/数据:

| 差异 | user(内部管理) | customer(外部客户) | 配置在哪 |
|---|---|---|---|
| token 寿命 / denyTTL | 短 | 长(App 30 天) | jwt refresh env(feature 自动取) |
| 绑不绑 IP | 绑 | 不绑(`sess:noip`) | 签发时 `SessionTags(scope, bindIP)` 按 scope 策略 |
| scope 名 | web / admin | web / app | `SessionTags` 参数,纯数据 |
| Redis 隔离 | 各自 `REDIS_DB` / `REDIS_ADDR` | 同上 | aurora RedisConfig |

## 集成配方

```go
// 签发(登录):按 scope 策略打标 + 绑 IP(绑不上就别发)
tags := tokenguard.SessionTags("web", bindIP)         // bindIP=false → 带 sess:noip
resp, _ := jwt.GenerateToken(uid, email, append(bizFeatures, tags...))
if bindIP {
    if err := g.BindLoginIP(ctx, accessJTI, loginIP, accessTTL); err != nil {
        return fmt.Errorf("建立 IP 绑定失败,拒绝签发: %w", err)
    }
}

// 中间件(每请求)
if err := g.VerifySession(ctx, claims, c.ClientIP()); err != nil { abort401(c); return }
if !slices.Contains(allowedScopes, tokenguard.ScopeOf(claims.Features)) { abort403(c); return }

// /refresh 轮换:吊销旧 refresh(没成功就别轮换)
if err := g.RevokeClaims(ctx, oldRefreshClaims); err != nil {
    return fmt.Errorf("吊销旧 refresh 失败: %w", err)
}

// 会话表撤销(只存了 jti)
if err := errors.Join(g.Revoke(ctx, sess.CurAccessJTI), g.Revoke(ctx, sess.CurRefreshJTI)); err != nil {
    return fmt.Errorf("撤销会话失败: %w", err)
}

// 被代理面(请求 IP 是代理容器 IP):只认吊销
if err := g.VerifySessionProxied(ctx, claims); err != nil { ... }
```

## 政策取舍

- **IP 绑定是高摩擦控制**:开了(未标 `sess:noip` 且走 `VerifySession`),会话中途换 IP(家宽重拨/移动网/
  CGNAT/v4v6 切换)会被 fail-close 踢 401。建议只对安全敏感、会话短、IP 稳的 scope(管理后台 web)开;
  公网/移动端 scope 打 `sess:noip`,靠黑名单吊销即可。
- **scope 标记复用 JWT features 数组**:靠 `PublicFeatures` 在对外输出时过滤,不泄漏给客户端。

## 三件套里的位置

`tokenguard` 是 aurora 防护三件套里的「登录后」。三者平级、独立、各自 `feature/<name>` 一个 feature,
同一套哲学(引擎给判定、动作/策略留业务):

| feature | 护哪一段 | 给什么判定 |
|---|---|---|
| `feature/loginguard` | 登录**前**(撞库/爆破限流) | decision(是否拦) |
| `feature/tokenguard` | 登录**后**(会话有效性:撤销 + 绑 IP) | error(是否拒) |
| `feature/doorman` | **每个请求**(风险评估) | RiskLevel(业务据此映射动作) |

loginguard、doorman 后续照本 feature 的样板(`feature/<name>` + `New<Name>Feature()` + `internal/` 藏实现 +
电池自含)抽进 aurora。
