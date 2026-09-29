# Aurora 框架文档

Aurora 是一个约定优于配置的 Go 后端框架:把「HTTP 服务器 + 数据库 + Redis + JWT + i18n + geoip + 限流/防护 + 迁移」收敛成一组可插拔的 **Feature**,用一个 **App** 容器统一装配、按环境变量配置、统一启停。

> **怎么读**:先读 **[架构与核心机制](./architecture.md)**(App / Feature / DI / 生命周期)——理解这个再看别的。
> 下面「能力总览」按**消费方式**分组:一眼看清哪些装上即用、哪些带前端、哪些是脚手架。深度细节进各自专文。
> 快速上手与完整示例见仓库根 [README.md](../README.md) 与 [sample/](../sample/)。

---

## 能力总览(按消费方式分组)

消费方式标签,决定你怎么拿来用:

- 🔌 **直接用 Feature** —— `app.AddFeature(xxx.NewXxxFeature(...))`(构造函数名写全,一眼看出 new 的是什么)+ 结构体 `inject:""` 注入。
- 🖥️ **带前端 Feature** —— 后端 `AddFeature`,**再把 `feature/<x>/web/` 组件拷进你的前端工程**。
- 📦 **库(非 Feature)** —— 不走 AddFeature,自己 `New(...)` 构造。
- 🏗️ **脚手架/约定** —— 照着搭 / fork,不是拿来注入。
- 🧩 **体系专题** —— 多个 Feature 组合成的系统,读组合叙事再选用。

### 🔌 直接用的 Feature

| Feature | 内容 | 关键点 |
|---|---|---|
| [server](./features/server.md) | HTTP 服务器、路由、Handler 约定、健康检查、优雅停机 | Handler 返回 `(data, BizError)`;可信代理→真实 `ClientIP()`;`SHUTDOWN_TIMEOUT` |
| [gorm](./features/gorm.md) | 数据库、连接池 | 多服务共库要算连接总账;`ConnMaxLifetime/IdleTime` |
| [redis](./features/redis.md) | Redis 封装、分布式锁 | `WithLock` 是 skip-if-running;`REDIS_PASSWORD` 强制非空 |
| [jwt](./features/jwt.md) | access/refresh token、黑名单登出 | jti;黑名单 TTL=剩余寿命;撤销检查 fail-close |
| [i18n](./features/i18n.md) | 多语言翻译 | 请求语言:`?lang=`>Accept-Language |
| [geoip](./features/geoip.md) | IP→归属地(国家/省/市/运营商)本地离线解析 | 双库 `//go:embed` 自包含、零配置、不外发;可选 ASN 面识别机房/云/Tor |
| [ratelimit](./features/ratelimit.md) | 计数地基:按 key 计数 / 失败锁 / 冷却 | fail-open;shape 在码值走 Provider;key 带服务 namespace(默认 SERVICE_NAME)防多服务共库撞键 |
| [loginguard](./features/loginguard.md) | 登录暴力破解防护(登录前) | 建在 ratelimit 上;账号硬锁 vs 只计数;fail-open |
| [tokenguard](./features/tokenguard.md) | 会话有效性(登录后) | jti 黑名单 + IP 绑定 + scope;**fail-close** |
| [controlgate](./features/controlgate.md) | 可信授权门禁(防搬走/防盗用):control 签发裁决 → 过期宽限/吊销/降级 | wrap `sealkit/guard`;coords 注入;**fail-close**;NTS 可信时钟;exempt 探针路径可配 |
| [错误模型 & 日志](./features/bizerr.md) | bizerr / logger | 默认响应只含 message;`LOG_LEVEL`>`RUN_LEVEL` |
| [migration](./features/migration.md) | goose 迁移 | `GOOSE_TABLE_PREFIX` 隔离共库版本表;worker 别跑迁移 |

### 🖥️ 带前端的 Feature

| Feature | 内容 | 关键点 |
|---|---|---|
| [doorman](./features/doorman.md) | 门房 / 风险评估器(可配规则 → 风险等级),含管理 API + **可复用前端配置台** | 只出风险等级不做动作;schema 驱动(后端加插件前端零改动);后端 `AddFeature` + 拷 `feature/doorman/web/` 组件 |

### 📦 库(非 Feature)

| 库 | 内容 | 关键点 |
|---|---|---|
| [mail](./features/mail.md) | 发信(供应商无关,SMTP + 可插拔授权 + 可配加密) | **不走 AddFeature、不读 env**;`mail.NewSMTP(...)` 按需构造 |

### 🏗️ 脚手架 / 怎么搭一个服务

- [搭建指南 building/](./building/) —— `bootstrap.InitDefaultApp`、分层结构(controller/service/datalayer/model)、以现有服务为骨架 fork。**这些是"照着搭",不是拿来注入的 Feature。**

### 🧩 体系专题(跨多个 Feature)

- **[防护体系 security suite](./topics/security-suite.md)** —— 防滥用/限流/登录防护/会话/风险决策的总设计:`ratelimit` 计数地基 + `loginguard`/`doorman`/`tokenguard` 如何成体系、跨项目复用。**要理解上面几个防护 Feature 如何配合,先读这篇。**

---

## 核心

- **[架构与核心机制](./architecture.md)** —— App 生命周期、Feature 系统、依赖注入、启停时序。**先读这篇。**
- **[配置系统 ResolveConfig](./features/config.md)** —— env tag / omitempty / Validate;⚠️ 含 `envDefault` 无效等致命坑。

---

## 贯穿全框架的几个"坑"(务必知道)

1. **`envDefault` 是死标签** —— `ResolveConfig` 不读它。要默认值靠代码,不靠 tag。见 [config.md](./features/config.md)。
2. **`omitempty` 决定必填性** —— 无 `omitempty` 的字段缺 env 就启动失败。可选字段务必加。
3. **`Validate` 要手动调** —— `ResolveConfig` 不自动校验。
4. **`REDIS_PASSWORD` 强制非空** —— 无密码 Redis 也得填占位(如 `none`)。
5. **优雅停机只关 HTTP** —— 不自动关其它 feature、不排空后台任务。后台排空要在 `main` 里自己编排。
6. **默认错误响应只有 `message`** —— 字段级校验明细要自定义 ErrorHandler。见 [bizerr.md](./features/bizerr.md)。

## 已知待改进项(文档暴露的框架缺口)

- `config/server.go` 的 `envDefault` 标签是死代码,建议让 `ResolveConfig` 真正支持,或删掉误导标签 + 给字段加 `omitempty` + 代码默认值。
- `i18n` 的 `LoadEmbedded`(`I18N_LOAD_EMBEDDED`)声明了但从未被读取,要么接上要么删。
- `RedisService` 未设 `PoolSize`,高并发下走默认池,可考虑做成可配。
