# ratelimit — 计数地基(🔌 直接用 Feature)

防护体系的**计数地基**:按不透明字符串 key,在窗口内做**计数 / 失败锁 / 冷却**。不认识任何业务概念(login/register/email…),只收 key。登录爆破锁、注册/发信/通用请求限频、重发冷却,都是它的不同用法(桶),**建其上、共用一份实现**。

> 体系全景与四件套如何配合,见专题 **[security-suite](../topics/security-suite.md)**。仓内速用见 [feature/ratelimit/README.md](../../feature/ratelimit/README.md)。

## 消费方式(类型化桶句柄)
```go
var LoginIPFail = ratelimit.NewFailLockBucket("login_ip_fail", "ip")   // 声明成句柄(包变量)
app.ProvideAs(ratelimit.StaticLimits(map[string]ratelimit.Limits{ ... }), (*ratelimit.LimitsProvider)(nil)) // 阈值=依赖,走 DI
app.AddFeature(ratelimit.NewRateLimitFeature("<服务名>",          // namespace 必填(位置参数,见下"命名空间")
    ratelimit.WithBucket(LoginIPFail),
))
// 业务:  RL ratelimit.Service `inject:""`  → s.RL.Fail(ctx, LoginIPFail, map[string]string{"ip": ip})
```

## 三原语(引擎内部,业务经 Service + 桶句柄用)
- **计数+窗口** `Hit`/`Peek`;**失败计数→锁** `Fail`/`Locked`/`ClearFail`;**冷却** `Cooldown`。
- 底层用**原子 Lua**(`INCR` + 仅首次 `PEXPIRE`(毫秒),经 `RedisService.Eval` 一条脚本)计数——杜绝"INCR 与过期设置分两条命令、中间 key 过期被重建成无 TTL → 永久锁死 = 自我 DoS";用 PEXPIRE 毫秒而非整秒,亚秒窗口也如实不被截成 0(EXPIRE 0 = 立即删键 = 静默失效)。
- 业务用 **`Service`** + **类型化桶句柄**(`NewCountBucket`/`NewFailLockBucket`/`NewCooldownBucket`):桶名不可能 typo(句柄是变量),调错方法(如对 `CountBucket` 调 `Fail`)= **编译错**——从根上没有"桶没注册/调错方法却 fail-open"。

## 关键点
- **fail-open**:Redis 抖动 → 放行 / no-op(次级防护,不因基础设施抖动挡住所有人);装配错误(缺 redis/namespace/provider、或注册的桶没配阈值)则 fail-startup。
- **Key 命名空间**:所有 key 前缀 `rate_limit:<namespace>:<bucket>:<dims>`,namespace **必填**(`NewRateLimitFeature` 位置参数)**、禁空** → 多服务共用一个 Redis DB 也不撞键(账号维度尤甚:两服务同名账号是不同的人)。
- **维度值转义**:引擎对维度值里的 `:`/`=`/`%` 自动转义(防 IPv6 冒号/crafted 值冲乱 key);调用方负责敏感维度(email/account)传哈希、IP 先 `net.ParseIP` 规范化。
- **shape/值分离**:桶名/种类/维度由句柄承载(代码),阈值走 `LimitsProvider`(静态 / 接项目设置、内存缓存热路径不查 DB)。
- **兄弟 feature 复用引擎**:`ratelimit.NewEngine(redis, namespace, provider)` 直接拿一个 `Service`,不经 app/DI 装配——给 `loginguard` 这类要在自己包内内嵌计数地基的 feature 用(它声明登录桶 + 用 `LoginPolicy` 适配阈值,内部持一个引擎)。app 级用法仍走 `NewRateLimitFeature`。
