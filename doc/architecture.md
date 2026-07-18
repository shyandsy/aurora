# Aurora 架构与核心机制

> 本文讲 Aurora 的"骨架"——App 生命周期、Feature 系统、依赖注入、配置解析。
> 各 Feature 的具体用法见 [features/](./features/)。

Aurora 是一个约定优于配置的 Go 后端框架:把「HTTP 服务器 + 数据库 + Redis + JWT + i18n + 邮件 + 迁移」这些每个服务都要重写的基础设施,收敛成一组可插拔的 **Feature**,用一个 **App** 容器统一装配、统一按环境变量配置、统一启停。

---

## 1. 三个核心抽象

| 抽象 | 定义位置 | 职责 |
|---|---|---|
| **App** | [contracts/app.go](../contracts/app.go) | 容器 + 生命周期。装 Feature、注册路由、Run/Shutdown。内嵌 `di.Container`,本身就是 DI 容器。 |
| **Feature** | [contracts/feature.go](../contracts/feature.go) | 一个可插拔的基础设施单元(gorm/redis/jwt…)。三个方法:`Name() / Setup(app) / Close()`。 |
| **Config** | [config/](../config/) | 每个 Feature 的配置结构体,字段用 `env:"XXX"` tag 声明,由 `ResolveConfig` 从环境变量填充。 |

Feature 接口极简([contracts/feature.go](../contracts/feature.go)):

```go
type Features interface {
    Name() string        // feature 名(日志/错误用)
    Setup(app App) error // 初始化:建连接、Provide 依赖到容器、注册路由
    Close() error        // 释放:关连接
}
```

**任何实现了这三个方法的类型都能作为 Feature 挂进 App** —— 这是框架的扩展点(见 [§5 自定义 Feature](#5-写一个自定义-feature))。

---

## 2. App 生命周期

一个典型 `main` 就三行:

```go
func main() {
    app := bootstrap.InitDefaultApp()          // 1. 装配
    app.RegisterRoutes(controller.GetRoutes(app)) // 2. 注册业务路由
    app.Run()                                   // 3. 启动,阻塞到退出信号
}
```

### 2.1 装配阶段 —— `InitDefaultApp`

[bootstrap/default.go](../bootstrap/default.go) 定义了"默认全家桶",**注册顺序即依赖顺序**:

```go
func InitDefaultApp() contracts.App {
    a := app.NewApp()                    // 建容器,解析 ServerConfig,注册基础依赖

    server := feature.NewServerFeature() // ★ server 必须最先
    a.AddFeature(server)

    a.AddFeature(feature.NewGormFeature())  // DB
    a.AddFeature(feature.NewRedisFeature()) // Redis
    a.AddFeature(feature.NewJWTFeature())   // JWT(依赖 Redis 做黑名单)
    a.AddFeature(feature.NewI18NFeature())  // i18n
    a.AddFeature(feature.NewMailFeature())  // 邮件

    migration.RunMigrations(a)           // ★ 最后跑 DB 迁移
    return a
}
```

> **为什么 server 最先**:`AddFeature` 里对 `ServerFeature` 做了特殊处理(见下),后续 feature 若要注册路由需要 server 已就位。
> **为什么迁移最后**:迁移要用已建好的 `*gorm.DB`。

如果默认全家桶不合适(比如 worker 进程不想跑迁移),就**不要用 `InitDefaultApp`**,自己 `app.NewApp()` 再手动 `AddFeature` 想要的子集。homeserver 的 `wire.InitWorkerApp()` 就是这么做的(worker 不跑迁移,避免和 API 并发抢 goose)。

### 2.2 `NewApp` 做了什么

[app/app.go:24](../app/app.go#L24):

1. `ResolveConfig(&cfg.Server)` —— 解析 `ServerConfig`(端口、名字、超时等),失败直接 `log.Fatalf`。
2. `di.NewContainer()` —— 建依赖注入容器。
3. `registerBaseDependencies()` —— 把 `ServerConfig` 自身 Provide 进容器,供后续 feature 取用。

### 2.3 `AddFeature` 做了什么

[app/app.go:54](../app/app.go#L54):

```go
func (a *app) AddFeature(f contracts.Features) {
    if server, ok := f.(contracts.ServerFeature); ok {
        a.serverFeature = server   // ★ 若是 ServerFeature,单独记住(Run 时用)
    }
    if err := f.Setup(a); err != nil {
        log.Fatalf("Failed to setup feature %s: %v", f.Name(), err)  // Setup 失败 = 进程退出
    }
    a.features = append(a.features, f) // 记录以便反序 Close
}
```

关键点:
- **Setup 立即执行**,不是延迟到 Run。所以 `AddFeature` 的调用顺序 = 初始化顺序。
- **Setup 失败 = `log.Fatalf` 直接终止进程**。这是框架的"fail-fast"原则:配置缺失/连不上 DB 就别启动。
- feature 按加入顺序存进 `a.features`,**Shutdown 时反序 Close**。

### 2.4 运行阶段 —— `Run`

[app/app.go:74](../app/app.go#L74):

```go
func (a *app) Run() error {
    a.printStartupInfo()             // 打印 banner(服务名/版本/地址/RunLevel)
    a.serverFeature.Start()          // 起 HTTP server(非阻塞,内部 goroutine)
    a.serverFeature.Wait()           // ★ 阻塞在这里,直到 server 退出
    return nil
}
```

`Run` 会一直阻塞在 `serverFeature.Wait()`,直到 server feature 收到退出信号并完成关停。**优雅停机的逻辑在 server feature 内部**(监听 SIGINT/SIGTERM → `http.Server.Shutdown(ctx)`),详见 [features/server.md](./features/server.md)。

### 2.5 关停 —— `Shutdown`

[app/app.go:85](../app/app.go#L85) 提供了**反序**关闭所有 feature 的 `Shutdown()`:

```go
func (a *app) Shutdown() error {
    for i := len(a.features) - 1; i >= 0; i-- { // 后进先出
        a.features[i].Close()
    }
    return nil
}
```

> ⚠️ **注意**:`Run()` 内部**并不会自动调用 `a.Shutdown()`**。server feature 收到信号只关 HTTP server,然后 `Run` 返回、`main` 结束、进程退出;gorm/redis 的 `Close()` 靠进程退出由 OS 兜底,而非显式调用。若你需要在退出前显式排空后台任务(如 asynq worker),要在 `main` 里自己编排 —— 参见 homeserver worker 的 `defer scheduleWorker.Stop()` 模式。

---

## 3. Feature 启停时序图

```
main
 │
 ├─ bootstrap.InitDefaultApp()
 │    ├─ app.NewApp() ──────────── ResolveConfig(Server) → DI 容器 → Provide(ServerConfig)
 │    ├─ AddFeature(server) ─────── server.Setup(app)   [建 gin engine]
 │    ├─ AddFeature(gorm) ───────── gorm.Setup(app)     [连 DB, Provide *gorm.DB / *sql.DB]
 │    ├─ AddFeature(redis) ──────── redis.Setup(app)    [连 Redis, Provide RedisService]
 │    ├─ AddFeature(jwt) ────────── jwt.Setup(app)      [取 RedisService, Provide JWT]
 │    ├─ AddFeature(i18n) ───────── i18n.Setup(app)     [加载 locale]
 │    ├─ AddFeature(mail) ───────── mail.Setup(app)     [校验 SMTP 配置]
 │    └─ RunMigrations(app) ─────── [用 *gorm.DB 跑 goose]
 │
 ├─ app.RegisterRoutes(routes) ──── serverFeature.RegisterRoutes()
 │
 └─ app.Run()
      ├─ serverFeature.Start() ──── ListenAndServe (goroutine) + signalListener (goroutine)
      └─ serverFeature.Wait() ───── 阻塞…
             │
        [SIGTERM]
             │
        server.Shutdown(ctx) ────── 排空在途请求(超时 SHUTDOWN_TIMEOUT)
             │
        Wait() 返回 → Run() 返回 → main 结束
```

---

## 4. 依赖注入(DI)

App 内嵌了 `github.com/shyandsy/di` 的 `Container`([contracts/app.go:17](../contracts/app.go#L17)),所以 **App 本身就是 DI 容器**,可直接调用容器方法。

### 4.1 两种角色

- **Feature 是"生产者"**:在 `Setup(app)` 里把自己建好的对象 `app.Provide(x)` 放进容器。
  例:gorm feature Provide 了 `*gorm.DB` 和 `*sql.DB`;redis feature Provide 了 `RedisService`。
- **业务代码是"消费者"**:用 `app.Find(&x)` 或结构体字段 `inject:""` tag 取出依赖。

### 4.2 消费依赖的两种写法

```go
// 写法 A:显式 Find
var db *gorm.DB
app.Find(&db)

// 写法 B:结构体 inject tag(推荐给 service/datalayer)
type customerService struct {
    DB    *gorm.DB              `inject:""`
    Redis feature.RedisService  `inject:""`
}
// app.Resolve(&svc) 会自动填充所有 inject:"" 字段
```

homeserver 里 `wire.RegisterProviders` + `app.Resolve` 就是批量装配 service/datalayer 依赖图的入口。

---

## 5. 写一个自定义 Feature

任何服务专属的基础设施(对象存储、消息队列消费者、指标上报…)都可以做成 Feature 挂进来。三步:

```go
package feature

type ossFeature struct {
    config *OSSConfig
    client *oss.Client
}

// 1. 实现 Features 接口
func (f *ossFeature) Name() string { return "oss" }

func (f *ossFeature) Setup(app contracts.App) error {
    if err := f.config.Validate(); err != nil {
        return err                       // 返回 error → App 会 Fatalf,fail-fast
    }
    f.client = oss.New(f.config.Endpoint, ...)
    app.Provide(f.client)                // 2. 把产物 Provide 进容器供业务用
    return nil
}

func (f *ossFeature) Close() error {
    return f.client.Close()              // 3. 释放
}

// 挂进去(替代 InitDefaultApp 或在其后 AddFeature)
app.AddFeature(NewOSSFeature())
```

配套的 `OSSConfig` 按 [配置系统](#6-配置系统) 的约定写 `env` tag 即可。

---

## 6. 配置系统

所有 Feature 配置统一走 `config.ResolveConfig(&cfg)`([config/resolve.go](../config/resolve.go)),从**环境变量**填充带 `env` tag 的字段。核心约定见 [features/config.md](./features/config.md),这里只记要点:

- 字段用 `env:"XXX"` 声明来源环境变量;
- **缺值 = 报错**(fail-fast),除非标 `,omitempty` → 缺值时跳过(保持零值);
- 支持 `string / int / bool / time.Duration` 等类型(duration 走 `time.ParseDuration`,如 `"30s"` `"1h"`);
- 每个 config 结构体实现 `Key()` 和 `Validate()`;`Validate` 在 feature `Setup` 里被调用做业务校验。

**给配置加字段的正确姿势**(以本仓刚加的连接寿命为例,[config/database.go](../config/database.go)):

```go
type DatabaseConfig struct {
    // 必填:缺 env 直接报错
    MaxOpenConns    int           `env:"DB_MAX_OPEN_CONNS"`
    // 可选:不设 = 零值 = 保持旧行为,向后兼容
    ConnMaxLifetime time.Duration `env:"DB_CONN_MAX_LIFETIME,omitempty"`
}
```

> ⚠️ **常见坑**:`ResolveConfig` **只认 `env` tag,不读 `envDefault`**。想要默认值,要么在构造 config 的地方写结构体字面量默认值,要么在 `Validate` 里兜底 —— 不要指望 `envDefault:"xxx"` 生效。

---

## 7. 相关文档

- 各 Feature 详解 → [features/](./features/)
- 配置解析机制详解 → [features/config.md](./features/config.md)
- 错误模型 → [features/bizerr.md](./features/bizerr.md)
- 快速开始 / 完整示例 → 仓库根 [README.md](../README.md) 与 [sample/](../sample/)
