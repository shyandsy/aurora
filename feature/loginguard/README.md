# loginguard — 登录暴力破解防护(aurora feature,防护三件套之「登录前」)

按 **IP + 账号** 维度做登录失败计数、短期锁定、成功清理。是限流/锁定策略与 Redis key 的单一真源。

## 对外只有一个契约:`Guard`

opt-in feature(不在 bootstrap 默认集,只有**处理登录**的服务需要)。登录服务注册一行,注入即用:

```go
// service main:redis 与 LoginPolicyProvider 都经 DI —— 先 ProvideAs provider,再 AddFeature(无参)。
app.ProvideAs(loginguard.StaticPolicy(loginguard.DefaultPolicy()), (*loginguard.LoginPolicyProvider)(nil))
app.AddFeature(loginguard.NewLoginGuardFeature())           // 须在 redis + 上面的 provider 之后

type userService struct {
    LoginGuard loginguard.Guard `inject:""`
}
```

```go
type Guard interface {
    PrecheckIP(ctx, ip) Decision            // 中间件(handler 前,仅有 IP):被锁 / 超小时上限则 Blocked
    PrecheckAccount(ctx, account) Decision  // 拿到账号后:硬锁模式下被锁则 Blocked
    RecordFailure(ctx, ip, account)         // 权威判定点(密码错等)调用
    RecordPending(ctx, ip, account)         // 密码对但登录未完成(如待 2FA):清失败,不计成功
    RecordSuccess(ctx, ip, account)         // 登录成功:清失败 + 计入每小时成功数
}
```

`Decision` 除 `Blocked`/`Reason` 外带 `IPFailRemaining` / `IPHourRemaining` / `AcctFailRemaining`,供调用方设「还剩几次」提示头(不影响放行)。

## 与「登录后」tokenguard 相反:fail-open

Redis 不可用时,预检**一律放行**、记录**一律 no-op**。登录限流是**次级**防护,绝不能因 Redis 抖动把所有人
锁在登录之外(可用性优先)。而 tokenguard 的撤销/IP 是**主级**安全控制,故 fail-close。两者哲学相反,都是刻意的。
(启动时若 redis / provider 未配 = 装配 bug,Setup fail-startup;这与**运行时** fail-open 不矛盾。)

## 一套代码,三种产品形态 —— 全靠 config

| 产品 | provider | 账号维度 | 账号标识 |
|---|---|---|---|
| deploy user / homeserver user | `StaticPolicy(默认)` | 硬锁(`AcctLockSeconds>0`) | 明文 email(便于 redis 手动解锁) |
| homeserver customer | 包住 `SystemSettingService` 的 adapter(运行时可调) | 只计数(`AcctLockSeconds=0`,防 DoS 锁他人) | email 哈希(不落客户明文) |

统一点:
- **配置**:依赖倒置 —— loginguard 只认 `LoginPolicyProvider`,不认任何产品的 `SecurityConfig`。静态用
  `StaticPolicy`;运行时可调自实现 provider(内部**内存缓存**自己的配置源,**绝不在 `LoginPolicy()` 里查 DB**)。
- **账号锁模式**:由 `AcctLockSeconds` 编码,不设额外开关 ——
  `>0` 硬锁(超阈值锁账号、预检拦);`=0` 只计数(照常累计供提示/监控,但**从不上锁、预检从不拦**);
  `AcctFailLimit<=0` 则整个账号维度关闭。
- **账号标识**:由**调用方**传入的 `account` 决定(明文 or 哈希),loginguard 不掺和。

## LoginPolicy

```go
type LoginPolicy struct {
    IPFailLimit     int  // 窗口内 IP 失败上限,超即锁 IP;<=0 关闭 IP 失败锁
    IPWindowSeconds int  // IP/账号 失败计数窗口;启用锁/计数时必须 >0
    IPLockSeconds   int  // IP 锁时长;IPFailLimit>0 时必须 >0
    IPPerHour       int  // IP 每小时成功登录上限;<=0 关闭
    AcctFailLimit   int  // 账号失败上限;<=0 关闭账号维度
    AcctLockSeconds int  // >0 硬锁 / =0 只计数(见上)
}
```

`DefaultPolicy()` 给一套安全起步值(账号维度默认 **only-count**,要硬锁的产品显式设 `AcctLockSeconds>0`)。
`Valid()` 校验自洽(开了锁/上限就必须配齐窗口/时长),feature 启动时对初始快照校验、不自洽即 fail-startup。

## 范围

loginguard 只管**登录爆破**。注册闸 / 发信闸 / 通用请求速率(homeserver customer 另有的那套)不在此 —— 那是
「请求级」doorman 的活。三件套:loginguard(登录前)/ tokenguard(登录后)/ doorman(每请求)。
