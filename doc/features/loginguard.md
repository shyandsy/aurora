# loginguard — 登录暴力破解防护(🔌 直接用 Feature,防护三件套之「登录前」)

按 **IP + 账号** 维度做登录失败计数、短期锁定、成功清理。是登录爆破防护的**好用外壳**:懂登录的领域语义(账号 vs IP、待 2FA、成功/失败),对外只暴露 `Guard`。

> 它**建在 [ratelimit](./ratelimit.md) 引擎上**(内部持有一个 `ratelimit.NewEngine`,登录桶声明其上,阈值由 `LoginPolicy` 适配成引擎的 `LimitsProvider`):计数/锁/TTL/原子性/key 隔离全归地基,loginguard 只做登录领域语义。全体系只有一份计数实现。体系定位见专题 **[security-suite](../topics/security-suite.md)**。

## 消费方式(opt-in,只处理登录的服务装)
```go
app.ProvideAs(loginguard.StaticPolicy(loginguard.DefaultPolicy()), (*loginguard.LoginPolicyProvider)(nil))
app.AddFeature(loginguard.NewLoginGuardFeature("user")) // namespace 必填(位置参数,通常传服务名);须在 redis + 上面的 provider 之后
// 登录服务:  LoginGuard loginguard.Guard `inject:""`
```
```go
type Guard interface {
    PrecheckIP(ctx, ip) Decision            // handler 前(仅有 IP):被锁/超小时上限则 Blocked
    PrecheckAccount(ctx, account) Decision  // 拿到账号后:硬锁模式下被锁则 Blocked
    RecordFailure(ctx, ip, account)         // 凭据失败(密码错等)权威判定点调用
    RecordPending(ctx, ip, account)         // 密码对但未完成(如待 2FA):只清账号失败、保留 IP、不计成功
    RecordSuccess(ctx, ip, account)         // 完成登录:清失败 + 计每小时成功数
    Unlock(ctx, ip, account)                // 运维强制解锁:清失败计数 + 锁(比 RecordSuccess 多清锁 key)
}
```

## 关键点
- **在真判定点调用**(密码错 → RecordFailure;待 2FA → RecordPending;完成 → RecordSuccess),不靠 HTTP 状态码猜成败。
- **账号维度模式**由 `LoginPolicy.AcctLockSeconds` 编码:公网**只计数**(硬锁会被拿来锁死他人 = DoS);内部/管理场景可**硬锁**。
- **`RecordPending` 只清账号失败、保留 IP**:密码对不代表同 IP 对别的账号善意,否则可被当"IP 锁重置原语"绕过。
- **fail-open**:Redis 抖动 → 预检放行、记录 no-op(次级防护,可用性优先;与 tokenguard 的 fail-close 刻意相反)。
- **计数/锁/TTL/原子性全归 [ratelimit](./ratelimit.md) 引擎**:loginguard 只声明 3 个登录桶(IP 失败锁、IP 每小时上限、账号失败锁)+ 用 `LoginPolicy` 适配阈值,不自己碰 redis key。反自我-DoS 的原子 `INCR + PEXPIRE`、key 命名空间隔离都由地基保证;loginguard 侧另有 miniredis 集成测试验端到端行为。
- **namespace 必填**:透传给底层引擎做 key 前缀 `rate_limit:<ns>:...`,多服务共库不撞键(账号维度尤甚)。
- 阈值走 `LoginPolicyProvider`:静态用 `StaticPolicy`;运行时可调自实现(内存缓存,`LoginPolicy()` 不查 DB)。
