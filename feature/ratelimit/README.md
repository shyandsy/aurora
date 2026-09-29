# ratelimit — 计数地基(aurora 防护体系)

按不透明字符串 key,在窗口内做**计数 / 失败锁 / 冷却**。不认识任何业务概念,只收 key。
登录爆破锁、注册/发信/通用请求限频、重发冷却,都是它的不同用法(桶),建其上、共用一份实现。
体系全景见 [../../doc/topics/security-suite.md](../../doc/topics/security-suite.md)。

## 三原语(引擎内部,业务不直接碰)
- **计数+窗口** `Hit`/`Peek`:窗口内计数,超上限 `Over`。
- **失败计数→锁** `Fail`/`LockState`/`Clear`:失败超阈值上短锁(锁 value 存解除时刻,给 RetryAfter)。
- **冷却** `Cooldown`:gap 内重复即拦。

底层用**原子 Lua**(`INCR` + 仅首次 `PEXPIRE`(毫秒),经 `RedisService.Eval` 一条脚本)计数——杜绝"INCR 与过期设置分两条命令、中间 key 过期被重建成无 TTL → 永久锁死 = 自我 DoS";用 PEXPIRE 毫秒而非整秒,亚秒窗口也如实不被截成 0(EXPIRE 0 = 立即删键 = 静默失效)。

## 用 Service + 类型化桶句柄
桶用 `NewCountBucket / NewFailLockBucket / NewCooldownBucket` 声明成**类型化句柄**(通常包变量);注册与调用点复用同一句柄:
- **桶名不可能 typo**(句柄是声明好的变量,不是每次手写字符串);
- **调错方法 = 编译错**(`Fail` 只收 `FailLockBucket`,把 `CountBucket` 传进去编译不过)——从根上消灭"桶没注册/调错方法却 fail-open 静默关限流"。

```go
// 声明句柄(包变量)
var (
    LoginIPFail = ratelimit.NewFailLockBucket("login_ip_fail", "ip")
    RegIPHour   = ratelimit.NewCountBucket("reg_ip_hour", "ip")
    EmailResend = ratelimit.NewCooldownBucket("email_resend", "email")
)

// 阈值来源是依赖 → 走 DI(先 ProvideAs);namespace 留空默认取 SERVICE_NAME;桶 → WithBucket(Setup 校验已配阈值)
app.ProvideAs(ratelimit.StaticLimits(map[string]ratelimit.Limits{
    "login_ip_fail": {Window: 5 * time.Minute, Limit: 5, LockSeconds: 900},
    "reg_ip_hour":   {Window: time.Hour, Limit: 8},
    "email_resend":  {Gap: 60 * time.Second},
}), (*ratelimit.LimitsProvider)(nil))
app.AddFeature(ratelimit.NewRateLimitFeature("",               // namespace 留空 → 自动取 SERVICE_NAME(也可显式传自定义 realm)
    ratelimit.WithBucket(LoginIPFail),
    ratelimit.WithBucket(RegIPHour),
    ratelimit.WithBucket(EmailResend),
))
```
```go
type someService struct { RL ratelimit.Service `inject:""` }

// 失败锁桶(方法只收 FailLockBucket)
if locked, retryAfter := s.RL.Locked(ctx, LoginIPFail, map[string]string{"ip": ip}); locked { /* 429 + retryAfter */ }
o := s.RL.Fail(ctx, LoginIPFail, map[string]string{"ip": ip})   // 凭据失败时
s.RL.ClearFail(ctx, LoginIPFail, map[string]string{"ip": ip})   // 成功时
// 计数桶 / 冷却桶
if o := s.RL.Hit(ctx, RegIPHour, map[string]string{"ip": ip}); o.Over { /* 拦 */ }
if ok, retryAfter := s.RL.Cooldown(ctx, EmailResend, map[string]string{"email": e}); !ok { /* 稍后再试 */ }
```
- **阈值走 `LimitsProvider`**:静态用 `StaticLimits`;运行时可调自实现(内部内存缓存,`Limits()` 里**绝不查 DB**)。每个 `WithBucket` 注册的桶,Setup 都校验其阈值已配(否则 fail-startup),防"声明了却没配 → 静默不限流"。
- **shape 在码、值在配**:桶名/种类/维度由句柄承载(代码),窗口/上限/锁/间隔由 `LimitsProvider`(配置)。

## Key 命名空间(多服务共库时的正确性关键)
所有 key 前缀 `rate_limit:<namespace>:<bucket>:<dim=val…>`。namespace 语义上就是「哪个服务」,故 `NewRateLimitFeature` 第一个参数**留空则默认取 `SERVICE_NAME`**(按服务身份天然解耦、零手传);要自定义 realm 才显式传;空且 `SERVICE_NAME` 也未配才 fail-startup。
- 常见部署里 Redis DB 按**环境**分、全服务同库 → 没有 namespace,`user` 与 `customer` 的同名桶会**撞 key**(尤其账号维度:两服务的同名账号是不同的人)。namespace 把它们**由构造隔离**。
- **`Service.Namespace()`**:暴露引擎实际用的 namespace,供在同一引擎上另建的附属结构(如登录「被锁列表」索引)用**同一前缀**拼 key、单一源不脱钩。
- **改 namespace = 换前缀,旧 key 成孤儿**:把显式 namespace 改成留空、或重命名 `SERVICE_NAME`,都会改变 key 前缀 → 旧前缀下的失败计数/锁**不迁移**、成为孤儿。因带 TTL,会自动过期清掉(短暂无害:个别正被锁的 IP/账号相当于提前解锁一次)。属一次性影响,知悉即可,不需迁移动作。

## 维度值:转义 / PII / 规范化
- **引擎自动转义**维度值里的 `:`/`=`/`%`,保证 key 对 (维度, 值) 单射——否则 IPv6 值自带的冒号、或 crafted 值里的 `:`/`=` 会冲乱 key 结构、串到别的桶(破坏计数)。
- **调用方责任**:①敏感维度(email/account)**传哈希**(别让明文 PII 进 Redis key,也顺带解决超长值);②IP 先 `net.ParseIP` **规范化**再传(躲 IPv6 多写法绕限流)。

## fail 原则
运行时 **fail-open**:Redis 抖动 → 放行、记录 no-op。限流是次级防护,不能因基础设施抖动把所有人挡门外(与 tokenguard 的 fail-close 刻意相反)。装配错误(缺 redis/provider,或 namespace 空且 SERVICE_NAME 也没配)则 fail-startup。
