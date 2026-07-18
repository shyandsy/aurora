# Aurora 框架文档

Aurora 是一个约定优于配置的 Go 后端框架:把「HTTP 服务器 + 数据库 + Redis + JWT + i18n + 邮件 + 迁移」收敛成一组可插拔的 **Feature**,用一个 **App** 容器统一装配、按环境变量配置、统一启停。

> 快速上手和完整示例见仓库根 [README.md](../README.md) 与 [sample/](../sample/)。本目录是**成体系的深度文档**:讲清每个机制的真实行为和坑。

## 核心

- **[架构与核心机制](./architecture.md)** —— App 生命周期、Feature 系统、依赖注入、启停时序。**先读这篇。**
- **[配置系统 ResolveConfig](./features/config.md)** —— env tag / omitempty / Validate;⚠️ 含 `envDefault` 无效等致命坑。

## Features

| 文档 | 内容 | 关键点 |
|---|---|---|
| [server](./features/server.md) | HTTP 服务器、路由、Handler 约定、健康检查、优雅停机 | Handler 返回 `(data, BizError)`;`SHUTDOWN_TIMEOUT` |
| [gorm](./features/gorm.md) | 数据库、连接池 | 多服务共库要算连接总账;`ConnMaxLifetime/IdleTime` |
| [redis](./features/redis.md) | Redis 封装、分布式锁 | `WithLock` 是 skip-if-running;`REDIS_PASSWORD` 强制非空 |
| [jwt](./features/jwt.md) | access/refresh token、黑名单登出 | jti;黑名单 TTL=剩余寿命;Redis 故障 fail-open |
| [i18n](./features/i18n.md) | 多语言翻译 | 请求语言:`?lang=`>Accept-Language;`LoadEmbedded` 未用 |
| [mail](./features/mail.md) | SMTP 发信 | MAIL_* 全强制必填,不发信也要填占位 |
| [migration](./features/migration.md) | goose 迁移 | `GOOSE_TABLE_PREFIX` 隔离共库版本表;worker 别跑迁移 |
| [错误模型 & 日志](./features/bizerr.md) | bizerr / logger | 默认响应只含 message;LOG_LEVEL>RUN_LEVEL |

## 贯穿全框架的几个"坑"(务必知道)

这些是文档里反复出现、最容易踩的点,集中列一下:

1. **`envDefault` 是死标签** —— `ResolveConfig` 不读它。`ServerConfig` 里那些 `envDefault` 全不生效,且字段无 `omitempty` = **实际必填**。要默认值靠代码,不靠 tag。见 [config.md](./features/config.md)。
2. **`omitempty` 决定必填性** —— 无 `omitempty` 的字段缺 env 就启动失败。可选字段务必加。
3. **`Validate` 要手动调** —— `ResolveConfig` 不自动校验。
4. **`REDIS_PASSWORD` 强制非空** —— 无密码 Redis 也得填占位(如 `none`)。
5. **优雅停机只关 HTTP** —— 不自动关其它 feature、不排空后台任务(asynq 等)。后台排空要在 `main` 里自己编排。见 [server.md](./features/server.md) 和 [architecture.md](./architecture.md)。
6. **默认错误响应只有 `message`** —— 字段级校验明细要自定义 ErrorHandler。见 [bizerr.md](./features/bizerr.md)。

## 已知待改进项(文档暴露的框架缺口)

写文档过程中发现的、值得后续修的框架问题(非阻塞,记录在案):

- `config/server.go` 的 `envDefault` 标签是死代码,建议要么让 `ResolveConfig` 真正支持 `envDefault`,要么删掉这些误导标签并给字段加 `omitempty` + 代码默认值。
- `i18n` 的 `LoadEmbedded`(`I18N_LOAD_EMBEDDED`)声明了但从未被读取,要么接上要么删。
- `mail` 全字段强制必填,不利于"可选发信"场景,可考虑改 `omitempty` + 用时判空。
- `RedisService` 未设 `PoolSize`,高并发下走默认池(`10×GOMAXPROCS`),可考虑做成可配。
