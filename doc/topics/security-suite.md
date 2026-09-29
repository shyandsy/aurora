# aurora 防护体系(security suite)

> 面向:任何基于 aurora 搭后端的项目的工程师。
> 这是 aurora 提供的一组**防滥用 / 限流 / 登录防护 / 会话有效性 / 风险决策**的可复用基础设施的**总设计**。
> 各项目只**声明 + 配置 + 挂点**,不重写这类基础设施。本文只讲框架契约与心智模型;某个项目怎么接、怎么迁,写在该项目自己的接入文档里。

---

## 一、背景与动机

几乎每个对外服务都要面对同一批问题:

- 有人在**猜密码**(登录爆破)、**刷注册 / 刷验证码 / 灌邮件**(资源滥用)、**探测接口**(枚举);
- 某些操作(注册 / 登录 / 找回密码)需要**据请求特征判断风险**,再决定放行 / 加验证 / 拦截;
- 登录之后,每个请求要确认**会话仍有效**(没被登出 / 撤销 / 换了环境)。

这些逻辑**与具体业务无关**,是基础设施。若每个项目各写一遍,会出现三种典型退化:①同一件事(按 key 计数/锁)被重复实现多份,key 还可能互相踩;②散落在中间件、service、管理端多处,难维护;③新项目只能把旧项目的这套**抄一遍改一遍** —— 改了 A 项目又要改 B 项目,还互相冲突。

**aurora 把这些收成一套体系,一份实现,跨项目复用。** 目标:一个新项目接入,只需声明"我有哪些防护点、阈值多少、挂在哪些路由",而**不必碰框架、不必 fork**。

---

## 二、心智模型:两根轴给一切防护定位

任何一块防护,先用两根轴定位,再谈实现:

- **轴一:请求生命周期** —— 登录**前**(未认证的滥用面)/ 敏感操作**时**(要判风险)/ 登录**后**(已认证请求的会话校验)。
- **轴二:机制 vs 决策** —— **机制**是客观地"数、锁、校验"(不含业务判断);**决策**是"据机制给的信号 + 其它事实,判定该怎么处置"。

四个组件正好落在这张图上:

```
请求进来
  │
  ▼  ── 机制层:防滥用 / 流量 ───────────────────────────────
  │   ratelimit        唯一「计数 + 锁 + 冷却」引擎(地基)
  │      ├ loginguard    登录桶(IP 失败锁 / IP 时上限 / 账号)   建其上
  │      └ 注册·发信·通用·冷却… 桶                              建其上
  │            │ 提供"速度/次数"信号
  ▼  ── 决策层:风险判定 ─────────────────────────────────────
  │   doorman          条件(可含 rate_limit 条件,读上面的计数)→ 风险等级
  │                    业务据风险映射动作(放行 / 挑战 / 拦)
  │            │ 登录成功签发 token
  ▼  ── 机制层:会话有效性(登录后)─────────────────────────
      tokenguard       jti 黑名单 + IP 绑定 + scope
```

**一句话**:`ratelimit` 数数 → `loginguard` 用它做登录爆破锁、`doorman` 用它做风险信号;`tokenguard` 管登录之后。机制层负责"发生了多少",决策层负责"据此怎么判",二者组合不重叠。

---

## 三、构建原则

**原则 1 — 唯一计数地基。** 所有"数次数 / 短锁 / 冷却"的东西(登录失败锁、注册限频、发信限频、通用请求限频、重发冷却)**是同一个原语**,只在 key 维度 / 窗口 / 计什么上不同。抽出唯一的 `ratelimit` 引擎,其余全部建其上,绝不各自再数一份、更不各自拼同名 key。

**原则 2 — 机制与决策分离。** `ratelimit` / `loginguard` / `tokenguard` 是机制;`doorman` 是决策,**消费**机制层的信号(如读 `ratelimit` 计数当风险因子)。不要让决策层自己再数一份,也不要让机制层夹带业务判断。

**原则 3 — shape 在代码,值在配置。** "有哪些防护桶 / 规则 / 会话策略"由**代码声明**(权威,防幻影配置);"阈值是多少"由**配置**提供(可来自静态常量,也可来自项目的运行时设置)。二者分离,才能既统一形状、又让每个项目独立调值。

**原则 4 — 一切项目差异外置成注入点。** 框架不认识任何项目的具体情况,所以下列全部由项目注入:
- **key 怎么拼**(按 IP / 账号 / 邮箱 / 路由,哈希还是明文)→ 项目给 key 生成器;
- **何时计、计成败**(成功 / 失败 / 每次)→ 中间件钩子 或 判定点显式调用;
- **阈值哪来** → `LimitsProvider`(静态 / 接项目设置);
- **响应怎么渲染**(状态码 / 机器码 / 信封)→ 框架给中性默认 + 覆盖钩子;
- **fail 策略** → 按角色定默认,允许覆盖。

框架里若硬编码了某项目的响应信封、机器码、配置结构或路由,别的项目复用时就得 fork = 又重写。**参数化做得越干净,复用面越大。** 这是整个体系能复用的前提。

---

## 四、组件契约

> 现状:`ratelimit` 引擎**已实现**(`feature/ratelimit`:对外 `Service`/桶/`LimitsProvider`,namespace + 测试;计数原语包内私有);`loginguard` / `tokenguard` / `doorman` 已在 aurora。`loginguard` 重构为建在 `ratelimit` 上**已完成**(经 `ratelimit.NewEngine`);gin 中间件骨架仍是后续 PR。下面标注了已实现 / 待做。

### 4.1 `ratelimit` —— 计数地基(机制,默认 fail-open)

只做一件事:按不透明字符串 key,在窗口内**计数 / 失败锁 / 冷却**。不认识任何业务概念。

**引擎内部三原语**(包内私有,业务不直接碰、一律经下面的 `Service` 用;fail-open 在内部,Redis 抖动 → 放行 / no-op)。示意其能力:

```go
type Outcome struct { Count int64; Over bool; Remaining int; RetryAfter int64 }

// 包内私有,示意三类能力(对外只暴露 Service):
type limiter interface {
    Hit(ctx, key string, window time.Duration, limit int) Outcome      // 计一次(超限也计);首次落 TTL + until
    Peek(ctx, key string, window time.Duration, limit int) Outcome     // 只读不计(预检 / remaining 头)
    Fail(ctx, failKey, lockKey string, window time.Duration, limit, lockSeconds int) Outcome // 失败计数,超限上锁(锁存解除时刻)
    LockState(ctx, lockKey string) (locked bool, retryAfter int64)
    Clear(ctx, keys ...string)                                          // 成功清失败计数
    Cooldown(ctx, key string, gap time.Duration) (ok bool, retryAfter int64) // gap 内重复即 ok=false
}
```

底层用**原子 Lua**(`INCR` + 仅首次 `PEXPIRE`(毫秒),经 `RedisService.Eval` 一条脚本)计数——杜绝"INCR 与过期设置分两条命令、中间 key 过期被重建成无 TTL → 永久锁死 = 自我 DoS";用 PEXPIRE 毫秒而非整秒,亚秒窗口也如实不被截成 0(EXPIRE 0 = 立即删键 = 静默失效)。

**业务用上层 `Service` + 类型化桶句柄(对外唯一运行时接口)**:方法收句柄(不是字符串),内部据句柄形状 + `LimitsProvider` 组 namespaced key、调内部引擎:
```go
// 桶声明成类型化句柄(包变量):
var LoginIPFail = ratelimit.NewFailLockBucket("login_ip_fail", "ip")

type Service interface {
    Hit / Peek(ctx, b CountBucket, dims map[string]string) Outcome                    // 计数桶(Peek 只读)
    Fail / Locked / PeekFail / ClearFail(ctx, b FailLockBucket, dims map[string]string) // 失败锁桶(PeekFail 只读失败计数)
    Cooldown(ctx, b CooldownBucket, dims map[string]string) (ok bool, retryAfter int64) // 冷却桶
}
// 例:s.RL.Fail(ctx, LoginIPFail, map[string]string{"ip": ip})
// 传错类型(把 CountBucket 传给 Fail)、桶名 typo,都是编译错 → 从根上没有"桶没注册/调错方法却 fail-open"。
```

**桶模型(shape / 值分离)**:形状由**类型化句柄**承载(种类=句柄类型,不再是可填错的 Kind 字段);值由 `LimitsProvider` 提供。
```go
b := ratelimit.NewCountBucket("register_ip_hour", "ip")    // 计数桶(Hit/Peek)
b := ratelimit.NewFailLockBucket("login_ip_fail", "ip")     // 失败锁桶(Fail/Locked/ClearFail)
b := ratelimit.NewCooldownBucket("email_resend", "email")   // 冷却桶(Cooldown)
type Limits struct { Window time.Duration; Limit int; LockSeconds int; Gap time.Duration }
type LimitsProvider interface { Limits(bucket string) Limits } // 静态 或 接项目设置(内存缓存,热路径不查 DB)
```

**扩展点(注入)**:桶用 `WithBucket(句柄)` 注册(Setup 校验其阈值已配,否则 fail-startup);阈值用 `LimitsProvider`(`StaticLimits` 或接设置的 adapter)提供;key 由句柄的维度 + 调用方传的维度值 + 服务 namespace 拼成;超限响应由调用方渲染(框架给中性默认 + 覆盖钩子,后续)。

**两种接入姿势**:①**判定点显式调用**(已实现,精确场景如登录):service 里注入 `Service`,在真判定点 `Fail/ClearFail/Hit`(不靠 HTTP 码猜结果);②**gin 中间件骨架**(待做,给简单路由声明式挂载)。

**fail 原则**:默认 **fail-open**(Redis 抖动 → 放行 / 记录 no-op),限流是次级防护,不因基础设施抖动把所有人挡门外。编程错(桶名/方法)已在编译期消灭,不走 fail-open。

**Key 命名空间与多服务隔离(重要)**:所有 key 前缀 `rate_limit:<namespace>:<bucket>:<dim=val…>`,namespace = 哪个服务,`NewRateLimitFeature` 首参**留空则默认取 `SERVICE_NAME`**(按服务身份天然解耦;要自定义 realm 才显式传;空且 SERVICE_NAME 也没配才 fail-startup)。`Service`/`Guard` 的 `Namespace()` 暴露该前缀,供附属结构(如被锁列表索引)用同一 ns 拼 key、单一源。

- 常见部署 Redis DB 按**环境**分、全服务同库 → 没有 namespace,`user` 与 `customer` 的同名桶会**撞 key**(账号维度尤甚:两服务同名账号是不同的人)。namespace 把它们**由构造隔离**。
- **维度值转义**:引擎对值里的 `:`/`=`/`%` 自动转义,保证 key 单射(防 IPv6 冒号 / crafted 值冲乱 key、串桶)。调用方另负责:敏感维度(email/account)传哈希(别落明文 PII)、IP 先 `net.ParseIP` 规范化。

### 4.2 `loginguard` —— 登录防护(机制,建在 ratelimit)

**与 ratelimit 的关系(为什么两个都要)**:`ratelimit` 是**通用计数引擎**,不懂"登录";`loginguard` 是它上面的**"登录档位"**,把登录特有的领域知识封起来。分工:

| | ratelimit(L0 引擎) | loginguard(L2 登录壳) |
|---|---|---|
| 懂什么 | 只懂"按 key 计数/锁/冷却" | 懂登录:账号 vs IP、待 2FA、成功/失败 |
| 对外 | `Service` + `NewEngine`(桶+维度;计数原语包内私有) | `Guard`(Precheck/Record*),登录流直接调 |
| 特有策略 | 无 | 账号硬锁 vs 只计数、`RecordPending` 清账号保留 IP、IP桶+账号桶协同 |

**已把 loginguard 的内脏换成 ratelimit 引擎**(自带的 Redis 计数改为经 `ratelimit.NewEngine` 声明 3 个登录桶 + 用 `LoginPolicy` 适配阈值),对外 `Guard` 接口与登录语义**一字未变**;**loginguard 未被删** —— 删了这些登录逻辑就得散进每个用登录的服务里各写一遍。**引擎统一,登录档位保留。**

登录暴力破解防护的**好用外壳**:对外 `Guard` 接口(`PrecheckIP` / `PrecheckAccount` / `RecordFailure` / `RecordPending` / `RecordSuccess`),内部用 `ratelimit` 的登录桶(IP 失败锁 + IP 时上限 + 账号)。要点:
- 账号维度模式由 `AcctLockSeconds` 编码:公网场景**只计数**(硬锁会被拿来锁死他人 = DoS),内部/管理场景可**硬锁**。
- 在**真判定点**调用(密码错 → `RecordFailure`;待 2FA → `RecordPending`;完成 → `RecordSuccess`),因此不需要"靠 HTTP 状态码猜成败"那套。
- `RecordPending` 只清账号失败、**保留 IP**(密码对不代表同 IP 对别的账号善意,否则可被当 IP 锁重置原语)。
- 默认 **fail-open**。

### 4.3 `doorman` —— 风险决策层

对敏感操作,按可配规则评估**一次请求的事实**(UA / IP / 归属地 / 是否机房 / 速度…)→ 一个**风险等级**;doorman **不执行动作**,业务据风险映射(放行 / 挑战 / 拦)。
- **与机制层的关系**:doorman 的条件(如 `rate_limit` 条件)读 `ratelimit` 的**同一份计数**当风险因子——"限流客观计数,doorman 据计数 + 其它事实综合判风险"。二者组合。
- doorman 自带规则/策略存储 + 管理 API + schema 驱动配置前端;这套"配置存储 + 管理台 + 可拷前端"是横切层(见 4.5)的模板。

### 4.4 `tokenguard` —— 会话有效性(登录后机制,fail-close)

每个已认证请求校验 token 会话是否仍有效(jti 黑名单 + IP 绑定 + scope)。与前三者**正交**。**fail-close**:Redis 报错时拒绝——这是主级安全控制,与限流的 fail-open 刻意相反。

### 4.5 横切层 —— 统一配置基座 + 管理台 + 前端

把每个项目都要重写的"安全配置页 + 后台被限流/被拦/会话列表"也做成 aurora 可复用(doorman 已示范):
- **配置基座**:中性「安全策略」存储 + `LimitsProvider` + 管理 API;项目**声明**桶/规则/会话策略,后台一个「安全」区填值。
- **观测/管理**:被限流实体、风险决策流水、活动会话,统一用 **管理 API + schema 驱动前端** 呈现(可列可解除)。
- 于是各项目的安全后台前后端也复用,新项目开箱即有。

---

## 五、接入指南(通用配方)

任何项目接入,大致五步(按需取,不必全上):

1. `app.ProvideAs(provider, (*ratelimit.LimitsProvider)(nil))` 供阈值(依赖,走 DI);静态用 `StaticLimits(...)`,运行时可调用包住项目设置的 adapter。
2. `app.AddFeature(ratelimit.NewRateLimitFeature("", ratelimit.WithBucket(...), ...))`(需已装 redis;namespace 留空 → 自动 SERVICE_NAME,要自定义 realm 才显式传)。
3. `WithBucket(...)` 声明本项目有哪些桶(shape)。
4. 需要精确判定的流程(如登录)在 service 判定点注入 `ratelimit.Service` 调 `Fail/ClearFail/Hit` / 用 `loginguard.Guard`;简单路由的 gin 中间件骨架待做。
5.(可选)敏感操作接 `doorman` 做风险决策;要统一后台就接横切层的管理 API + 拷配置前端。

**示例消费者(仅举例,细节各项目自定)**:
- *一个面向公网的 C 端*:登录账号维度**只计数**(防 DoS)、account 用邮箱哈希;有注册/发信/通用请求多种桶;阈值接自己的运行时设置。
- *一个内部管理/认证服务*:登录账号维度可**硬锁**、account 用明文(便于运维在 Redis 手动解锁);桶少;阈值用静态默认即可。

**判断是否"通用"的验收**:一个从没接过的项目,靠上面五步就能用,**不改 aurora 一行、不 fork**。

---

## 六、fail 原则矩阵

| 组件 | fail | 理由 |
|---|---|---|
| ratelimit / loginguard | **open** | 次级防护;Redis 抖动不能把所有人锁在门外(可用性优先) |
| doorman | 降级为判 none | 来源抖动时不额外加风险、不误伤 |
| tokenguard | **close** | 主级安全控制;撤销 / IP 校验失败必须拒 |

---

## 七、风险与待定

- **真实客户端 IP**:所有按 IP 的限流/绑定都依赖拿到真实 IP;经反代/网关时须正确透传,否则共享出口(CGNAT / 代理)会互相挤占额度而误锁。框架不解决,接入方保证。
- **key 落哈希还是明文**:由项目的 key 生成器决定(对外不落用户明文 / 内部可明文便于运维)。
- **每桶是否可单独配 fail-open/close**:默认全 open,预留覆盖。
- **横切层存储**:走 doorman 式独立表 + Console,还是复用项目已有设置表 —— 横切层落地时定。
- **`ratelimit` 与 `loginguard` 的收敛(已完成)**:`loginguard` **已建在 `ratelimit` 引擎上**(经 `ratelimit.NewEngine`:声明 3 个登录桶 + 用 `LoginPolicy` 适配阈值),不再自带计数——全体系只有一份计数实现。兄弟 feature 复用地基的入口就是 `NewEngine(redis, namespace, provider)`。
- **计数原子性**:计数/锁/TTL 全在 `ratelimit` 引擎里,用原子 `INCR + 首次 PEXPIRE`(毫秒)脚本(经 `RedisService.Eval`),无 TTL 竞态根治,miniredis 真集成测试守着(假 Redis 不跑 Lua,测不到 TTL);`loginguard` 经引擎自动获得同等保证。**待办**:homeserver 自己的 `common/ratelimit.incrWithTTL` 有同样竞态,迁到 aurora 时一并切。
- **redis 能力契约单一真源**:`ratelimit`(及经它的 `loginguard`)**直接依赖 redis feature 的 `RedisService`**,不各自另抄一份最小 redis 接口(契约只在 redis feature 一处;单测用内嵌接口只实现用到的方法的假实现)。
