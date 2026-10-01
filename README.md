# Aurora Framework

A lightweight, modular web framework for Go, built on top of Gin with dependency injection support.

## Features

- 🚀 **Modular Architecture**: Feature-based design for easy extensibility
- 🔌 **Dependency Injection**: Built-in DI container (`github.com/shyandsy/di`) for clean dependency management
- 🗄️ **Database Support**: GORM integration with MySQL and SQLite drivers
- 🔐 **JWT Authentication**: Built-in JWT token generation, validation, and refresh with Redis blacklist support
- 📦 **Redis Support**: Redis integration with service interface for caching and session management
- 🔄 **Database Migrations**: Goose-based migration system with automatic version tracking
- ⚙️ **Configuration Management**: Environment-based configuration loading with validation
- 🛡️ **Error Handling**: Unified business error handling with validation error support
- 🌐 **CORS Support**: Configurable CORS middleware
- 🕵️ **Trusted Proxies**: Configurable trusted-proxy list so `c.ClientIP()` returns the real client IP behind a reverse proxy and can't be spoofed via `X-Forwarded-For` (secure-by-default: trusts private ranges only)
- 🔒 **Route Middlewares**: Support for route-specific Gin middlewares (e.g., JWT authentication, rate limiting)
- 🏥 **Health Checks**: Built-in `/health` and `/ready` endpoints
- 📝 **Request Context**: Extended request context with App instance for easy dependency access
- 🌍 **Internationalization (i18n)**: Multi-language support using go-i18n with automatic language detection
- 🌏 **GeoIP**: Offline IP→location (country/province/city/ISP) with embedded ip2region (CN) + DB-IP (international) databases — zero-config, self-contained, no network at build or runtime. Optional ASN面 (`WithASNEnabled`) adds ASN / AS-org / a heuristic hosting flag (embedded DB-IP ASN Lite) for spotting datacenter/cloud/Tor registrations
- 📊 **Structured Logging**: Built-in logger with log levels (Error, Info, Debug) and environment-based configuration

## Capabilities — 按"怎么消费"分组

一眼看清框架里有什么、每样**怎么拿来用**(标签即消费方式)。深度细节全在 **[doc/](doc/README.md)**。

**🔌 直接用的 Feature**（`app.AddFeature(...)` + 结构体 `inject:""`）:
`server` · `gorm` · `redis` · `jwt` · `i18n` · `geoip` · `migration` · `bizerr`/`logger` · `ratelimit`（计数地基）· `loginguard`（登录前防护）· `tokenguard`（登录后会话）· `controlgate`（可信授权门禁 / 防搬走·防盗用)

**🖥️ 带前端的 Feature / 模块**（后端 + 前端源码**共置**在 `feature/<x>/web/` 或 `modules/<x>/web/`;aurora 只存源不构建,消费方从源构建/加载,**别各拷各建**,见各自 README):
`doorman`（门房/风险评估器,schema 驱动配置台前端,`feature/doorman/web/`）· `modules/user`（用户中心,完整 SPA 前端 `modules/user/web/` → 构建一份版本化共享 remote)

**📦 库(非 Feature)**（不走 AddFeature,直接 import 用函数/类型）:
- 后端:`mail`（发信)· `middleware`（JWT 鉴权中间件 + rolefeature）· `encryption`（AES-256-GCM 凭据加解密,密钥调用方注入)· `types`（业务无关通用类型:JSON 列 / 分页 DTO / 状态枚举)
- 前端:[`web/common`](web/common/)（通用 Angular 共享库:工具 date/bytes util、通用 DTO、confirm-dialog 组件;供各模块/特性前端 `@common/*` import,别各自复制)

**🏗️ 脚手架/约定**（照着搭 / fork,不是拿来注入）:
`bootstrap.InitDefaultApp` · 分层结构 controller/service/datalayer/model · 以 `sample/full_showcase` 为骨架 → 见 [doc/building](doc/building/)

**🧩 体系专题**（多个 Feature 组合成的系统）:
**防护体系** = `ratelimit`+`loginguard`+`doorman`+`tokenguard` → 见 [doc/topics/security-suite](doc/topics/security-suite.md)

**🧱 可挂载服务模块**（`modules/`，一整套后端服务：`app.AddFeature(x.NewFeature(cfg))` + `app.RegisterRoutes(x.Routes(app))` 就有整套,项目差异只走 `Config`）:
`modules/user`（后台账号中心:账号/RBAC/登录/2FA/会话/微服务 token/登录防护/被锁管理/gate;**全栈**——配套前端 SPA 在 [`modules/user/web/`](modules/user/web/)）→ 见 [modules/README.md](modules/README.md)

## Documentation

- **[doc/README.md](doc/README.md)** —— 深文档索引(按消费方式分组的全量能力总览)。**要系统了解框架,从这里进。**
- **[doc/architecture.md](doc/architecture.md)** —— App / Feature / DI / 生命周期。**先读这篇。**
- **[doc/topics/security-suite.md](doc/topics/security-suite.md)** —— 防护体系(限流/登录/会话/风险)总设计。
- **[doc/building/](doc/building/)** —— 怎么用框架搭一个服务(脚手架/分层/fork)。
- 单点 Feature 参考:[doc/features/](doc/features/)。

## Installation

```bash
go get github.com/shyandsy/aurora
```

## 消费 aurora:两种方式(别把第二种当默认)

aurora 有**两条**被消费的路径。**默认、通用、绝大多数项目走第一条**;第二条**只为一个特定场景(防逆向)存在**,是 optional,**不是必须**。先认清你属于哪种,别套错模型。

### ① 默认 —— `go get` 直接依赖(普通 Go module)

```bash
go get github.com/shyandsy/aurora
```

直接 `import "github.com/shyandsy/aurora/..."` 用,go module 正常依赖,**不改 import 路径、不 replace**。绝大多数消费项目都是这样。

唯一例外是 `go vendor` 带不走的**非 `.go` 源**(前端 web 源 `modules/*/web`、`feature/*/web`、`web/common`;goose 迁移 `.sql`):这些按需**窄同步**——只把**用到的那几个目录**拷进本项目的 `third_party/aurora/`(**只拷这些目录,不拷 Go、不改任何 import、不 replace**),REF pin 到与 `go.mod` 同一 commit 保证同源。

> ⚠️ **`third_party/aurora/` 文件夹存在 ≠ 你在走 ②。** ① 的窄同步也落在这个文件夹里(web/迁移源)。区分 ①/② **看的是有没有改 import 路径 / `replace`,不是文件夹在不在**。

### ② 身份隐藏 —— 整棵树 vendor + 改 import 路径(仅防逆向场景)

**目的(唯一):** 当构建产物会落到不可信方(对外交付 / 暴露的二进制),需要**在产物里隐藏 aurora / 上游身份**以防逆向时,才把 aurora **整棵树** vendor 进 `third_party/aurora/` 并把 import 路径 `github.com/shyandsy/aurora` 整体**改名**到一个中立 host(如 `bitbucket.com/...`),让产物里认不出上游。(前身是 garble;garble 弃用后改用改名这招。)

**没有这个防逆向 / 身份隐藏需求的项目,一律用 ①。** 别只因为"想在本地持有一份"就上 ②——那是 ① 的窄同步就能满足的事。

判断你在哪种(任一 tell 即可):

- **最直观**:import 写的是 `github.com/shyandsy/...`(→ ①),还是中立 host 如 `bitbucket.com/...`(→ ②)。
- **go.mod**:aurora 是 `require github.com/shyandsy/aurora`(→ ①),还是被 `replace` 成 `third_party/aurora` 的本地改名路径(→ ②)。

### 随仓 skill 分发 —— 和代码消费**正交**(①② 都能用)

aurora 把跨项目复用的 agent skill 放在 `.claude/skills/`。**拿 skill 和"你怎么消费 aurora 的代码"没有关系**,由独立脚本 **`scripts/sync-skills.sh`** 负责,**不碰 `go.mod`、不碰 `third_party/aurora`**。

> ⚠️ **先解决"从哪拿到这个脚本"——这正是容易把人绕回 vendor 的坑。** ① 项目**本地没有 aurora 树**(aurora 只是 go module),所以**不存在 `<aurora>/scripts/sync-skills.sh` 这个本地路径**。**别为了拿这个脚本去 clone / vendor 整个 aurora** —— 那就又掉回 ② 了。正确做法见下,按你是 ① 还是 ② 分叉。

**①(go get)项目 —— 自带一个独立 helper,不碰 aurora 树:**

`scripts/sync-skills.sh` 是**零依赖的单文件**。把它**拷一份进你自己的仓**(如本项目 `scripts/sync-skills.sh`,托管副本,别手改,升级就重拷)——**这不是 vendor aurora**,只是带一个独立小工具。然后在你的 `Makefile` 加**一行**目标:

```makefile
sync-aurora-skills:   ## 同步 aurora 的 agent skill 进项目根 .claude/skills/(与代码消费无关)
	REF=$(AURORA_REF) SKILLS="design-nav" bash scripts/sync-skills.sh
```

脚本会从上游**只** sparse-checkout `.claude/skills`(不下 Go/web 源、**不建 `third_party/aurora`**)拷进项目根 `.claude/skills/`。它和 `sync-aurora-web` 这类**代码**同步目标**平级、互不依赖**。

> 不想带脚本副本?也可以把那几行 sparse-clone 直接内联进 Makefile 目标(效果一样,代价是 clone 逻辑在各项目各一份)。两种都行,**唯独不要为拿 skill 去 vendor 整棵 aurora。**

**②(整树 vendor)项目 —— 本地已有脚本和 skill:**

你的 `third_party/aurora/` 里已有 `scripts/sync-skills.sh` 和 `.claude/skills/`,直接:

```bash
bash third_party/aurora/scripts/sync-skills.sh   # 自动发现本地树,零网络镜像进项目根
```

`scripts/post-sync.sh` 现在只是转发到 `sync-skills.sh` 的**向后兼容 shim**(已写 `post-sync` 调用的项目不受影响;新项目直接用 `sync-skills.sh`)。

> 历史教训:skill 分发曾寄生在 ② 的整树 vendor 上,逼着只想要个 skill 的 ① 项目去上整套 vendoring —— 这正是"把 ② 当默认"的误导源。现已拆开:**要 skill 走 `sync-skills.sh`,和要不要 vendor 整树无关;而拿这个脚本本身也不需要 vendor aurora。**

> 一句话:**改 import 路径 / 整树 vendor 只在"要把上游身份藏进交付产物"时才做,不是消费 aurora 的前提,更不是拿 skill 的前提。** 看到 `make sync-aurora`、中立 host 的 import 路径,先确认目标项目是不是真有防逆向需求(② 类),再照搬。

## Examples

The framework includes sample projects under **[sample](sample/)**:

| Sample | Description |
|--------|-------------|
| **[sample/full_showcase](sample/full_showcase/)** | Full application: Server, GORM, Redis, JWT, i18n, migrations, layered structure (controller / service / datalayer), RBAC, and auth. Use this as a reference for building a complete Aurora app. |
| **[sample/customize_error_handler](sample/customize_error_handler/)** | Minimal app that demonstrates [custom error response format](sample/customize_error_handler/README.md): implement `contracts.ErrorHandler` and pass it via `feature.WithErrorHandler()` so all handler errors use your own JSON shape. |

Run an example from the **Aurora repo root** (e.g. `cd sample` then the run command in that sample’s README). See each sample’s README for required environment variables and run commands.

## Quick Start

### Basic Usage

```go
package main

import (
    "log"
    
    "github.com/shyandsy/aurora/bootstrap"
    "github.com/shyandsy/aurora/contracts"
    "github.com/shyandsy/aurora/bizerr"
)

func main() {
    // Create application with default features (Server, GORM, Redis, JWT)
    app := bootstrap.InitDefaultApp()
    
    // Register routes
    app.RegisterRoutes([]contracts.Route{
        {
            Method:  "GET",
            Path:    "/hello",
            Handler: func(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
                return map[string]string{"message": "Hello, Aurora!"}, nil
            },
        },
    })
    
    // Run application
    if err := app.Run(); err != nil {
        log.Fatalf("Failed to run app: %v", err)
    }
}
```

### Custom App Setup

```go
package main

import (
    "log"
    
    "github.com/shyandsy/aurora/app"
    "github.com/shyandsy/aurora/feature"
    "github.com/shyandsy/aurora/contracts"
    "github.com/shyandsy/aurora/bizerr"
)

func main() {
    // Create application
    a := app.NewApp()
    
    // Add features manually
    a.AddFeature(feature.NewServerFeature())
    a.AddFeature(feature.NewGormFeature())
    a.AddFeature(feature.NewRedisFeature())
    a.AddFeature(feature.NewJWTFeature())
    
    // Register routes
    a.RegisterRoutes([]contracts.Route{
        {
            Method:  "GET",
            Path:    "/api/users",
            Handler: getUserHandler,
        },
    })
    
    // Run application
    if err := a.Run(); err != nil {
        log.Fatalf("Failed to run app: %v", err)
    }
}

func getUserHandler(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
    // Access App instance directly from context
    var userService UserService
    if err := c.App.Find(&userService); err != nil {
        return nil, bizerr.ErrInternalServerError(err)
    }
    
    // Your handler logic
    return map[string][]string{"users": {"user1", "user2"}}, nil
}
```

## Configuration

Aurora uses environment variables for configuration. All configurations are validated on startup.

### Server Configuration

- `HOST`: Server host (default: `0.0.0.0`)
- `PORT`: Server port (default: `8080`)
- `SERVICE_NAME`: Service name (required)
- `SERVICE_VERSION`: Service version (default: `1.0.0`)
- `RUN_LEVEL`: Run level - `local`, `stage`, or `production` (default: `local`)
- `READ_TIMEOUT`: Read timeout (default: `30s`)
- `WRITE_TIMEOUT`: Write timeout (default: `30s`)
- `SHUTDOWN_TIMEOUT`: Graceful shutdown timeout (default: `5s`)
- `TRUSTED_PROXIES`: Comma-separated proxy IPs/CIDRs whose `X-Forwarded-For` is trusted for `c.ClientIP()` (optional). Unset → loopback + RFC1918 private ranges; `none` → trust nobody (client IP = direct peer); `0.0.0.0/0,::/0` → trust all (legacy gin default). See [Server feature docs](doc/features/server.md#可信代理--真实客户端-ip).

**Note**: Gin mode is automatically set based on `RUN_LEVEL`:

- `production` → `release` mode
- `local` or `stage` → `debug` mode

### Database Configuration

- `DB_DRIVER`: Database driver - `mysql` or `sqlite` (required)
- `DB_DSN`: Database connection string (required)
- `DB_MAX_IDLE_CONNS`: Maximum idle connections (required, must be > 0)
- `DB_MAX_OPEN_CONNS`: Maximum open connections (required, must be > 0, must be >= DB_MAX_IDLE_CONNS)

**Example**:

```bash
DB_DRIVER=mysql
DB_DSN=user:password@tcp(localhost:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local
DB_MAX_IDLE_CONNS=10
DB_MAX_OPEN_CONNS=100
```

### Redis Configuration

- `REDIS_ADDR`: Redis address (required, format: `host:port`)
- `REDIS_PASSWORD`: Redis password (required)
- `REDIS_DB`: Redis database number (required, must be >= 0)

**Example**:

```bash
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=yourpassword
REDIS_DB=0
```

### JWT Configuration

- `JWT_SECRET`: JWT secret key (required, must be changed from default in production)
- `JWT_EXPIRE_TIME`: Access token expiry duration (required, e.g., `15m`, `1h`)
- `JWT_ISSUER`: JWT issuer identifier (required)
- `JWT_REFRESH_EXPIRE_TIME`: Refresh token expiry duration (optional, e.g., `30m`, `24h`). When unset it defaults to `JWT_EXPIRE_TIME * 2`.

**Example**:

```bash
JWT_SECRET=your-super-secret-jwt-key-here-change-in-production
JWT_EXPIRE_TIME=15m
JWT_ISSUER=myapp
# Optional: override refresh token lifetime (defaults to JWT_EXPIRE_TIME * 2)
JWT_REFRESH_EXPIRE_TIME=30m
```

**Note**: Refresh tokens expire after `JWT_REFRESH_EXPIRE_TIME` when set, otherwise `JWT_EXPIRE_TIME * 2` (e.g., 15m * 2 = 30 minutes).

### I18N Configuration

Configure internationalization settings:

```bash
# Default language (required)
I18N_DEFAULT_LANG=en

# Supported languages (comma-separated, required)
I18N_SUPPORTED_LANGS=en,zh-CN,ja

# Application locale files directory (relative to working directory, optional)
# Framework locale files are embedded in the binary and loaded automatically
I18N_LOCALE_DIR=locales
```

**Important Notes**:

- **Framework locale files** are embedded in the Aurora binary using `go:embed` and are always loaded automatically. They are located at `feature/i18n/` in the source code.
- **Application locale files** should be placed in the directory specified by `I18N_LOCALE_DIR` (relative to your application's working directory).
- Application locale files can override framework messages with the same message ID.

**Locale File Format**:

Create language files using **flat structure** (not nested). The framework supports multiple formats with the following priority:

1. **YAML** (`.yaml` or `.yml`) - Recommended, most readable
2. **TOML** (`.toml`)
3. **JSON** (`.json`)

**Framework locale file example** (`feature/i18n/en.yaml`):

```yaml
error.not_found:
  id: error.not_found
  other: Resource not found

error.internal_server:
  id: error.internal_server
  other: Internal server error

error.validation:
  id: error.validation
  other: "Validation error: {{.Message}}"

error.bad_request:
  id: error.bad_request
  other: Bad request

error.unauthorized:
  id: error.unauthorized
  other: Unauthorized

error.forbidden:
  id: error.forbidden
  other: Forbidden
```

**Application locale file example** (`locales/en.yaml`):

```yaml
welcome:
  id: welcome
  other: Welcome to Customer Service

user.email_exists:
  id: user.email_exists
  other: Email already exists

user.invalid_email:
  id: user.invalid_email
  other: Invalid email format

auth.register_success:
  id: auth.register_success
  other: Registration successful
```

**Note**: Use flat structure with dot notation (e.g., `user.email_exists:`) instead of nested structure (e.g., `user: email_exists:`). This ensures compatibility with `go-i18n`'s message parsing.

Example TOML file (`locales/en.toml`):

```toml
[welcome]
id = "welcome"
other = "Welcome to Customer Service"

[user.email_exists]
id = "user.email_exists"
other = "Email already exists"

[error.validation]
id = "error.validation"
other = "Validation error: {{.Message}}"
```

### Migration Configuration

- `GOOSE_TABLE_PREFIX`: Optional prefix for goose version table name (optional)
  - If set, goose version table will use this prefix (e.g., `admin_goose_db_version`)
  - If not set or empty, goose default table name `goose_db_version` will be used

**Example**:

```bash
# Use default table name "goose_db_version"
# (no environment variable needed)

# Use custom table name "admin_goose_db_version"
GOOSE_TABLE_PREFIX=admin_
```

### CORS Configuration

- `CORS_ALLOWED_ORIGINS`: Comma-separated list of allowed origins (optional)
- `CORS_ALLOWED_METHODS`: Comma-separated list of allowed HTTP methods (optional)
- `CORS_ALLOWED_HEADERS`: Comma-separated list of allowed headers (optional)
- `CORS_ALLOWED_CREDENTIALS`: Allow credentials (optional, `true` or `false`)

**Note**: CORS is only enabled if at least one CORS configuration is provided.

### Logger Configuration

Aurora provides a built-in structured logger with three log levels:

- `LOG_LEVEL`: Log level - `error`, `info`, or `debug` (optional, highest priority)
- `RUN_LEVEL`: Automatically determines log level if `LOG_LEVEL` is not set

**Log Level Priority**:

1. **`LOG_LEVEL` environment variable** (if set, takes highest priority)
2. **`RUN_LEVEL` environment variable** (if `LOG_LEVEL` is not set):
   - `local` → `debug` (all messages logged)
   - `stage` → `info` (error and info messages logged)
   - `production` → `error` (only error messages logged)
3. **Default**: `error` (if neither `LOG_LEVEL` nor `RUN_LEVEL` is set)

**Log Levels**:

- `error`: Only error messages are logged (suitable for production)
- `info`: Error and info messages are logged
- `debug`: All messages (error, info, and debug) are logged

**Examples**:

```bash
# Option 1: Explicitly set LOG_LEVEL (highest priority)
LOG_LEVEL=debug

# Option 2: Let RUN_LEVEL determine log level automatically
RUN_LEVEL=local      # → debug level
RUN_LEVEL=stage      # → info level
RUN_LEVEL=production # → error level

# Option 3: Override RUN_LEVEL with explicit LOG_LEVEL
RUN_LEVEL=production
LOG_LEVEL=debug      # → debug level (LOG_LEVEL takes priority)
```

**Note**: If neither `LOG_LEVEL` nor `RUN_LEVEL` is set, the logger will use `error` level by default and print a message indicating the default log level being used.

## Architecture

### Core Components

#### App Interface

The `contracts.App` interface provides:

- `AddFeature(feature Features)`: Register a feature
- `RegisterRoutes(routes []contracts.Route)`: Register API routes
- `Run() error`: Start the application
- `Shutdown() error`: Gracefully shutdown the application
- `GetContainer() di.Container`: Get the DI container
- Direct access to `di.Container` methods (Provide, Resolve, Find, etc.)

#### Request Context

Aurora provides `contracts.RequestContext` which extends `gin.Context` with the App instance and Translator:

```go
type RequestContext struct {
    *gin.Context
    App        contracts.App
    Translator contracts.Translator
}
```

This allows handlers to directly access the App instance, DI container, and translation service without global variables or context lookups.

**Language Detection**:

The `RequestContext` automatically detects the language from:

1. Query parameter `lang` (e.g., `?lang=zh-CN`)
2. `Accept-Language` HTTP header
3. Default language from configuration

**Translation in Handlers**:

```go
func myHandler(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
    // Use the built-in T method for translation
    message := c.T("welcome")
    
    // Translation with variables
    errorMsg := c.T("error.validation", map[string]interface{}{
        "Message": "Email is required",
    })
    
    return map[string]string{
        "message": message,
        "error": errorMsg,
    }, nil
}
```

#### Feature System

Features implement the `contracts.Features` interface:

- `Name() string`: Feature identifier
- `Setup(app App) error`: Initialize the feature
- `Close() error`: Cleanup resources

#### Built-in Features

1. **ServerFeature**: HTTP server with routing, health checks, and graceful shutdown
   - Automatically registers `/health` and `/ready` endpoints
   - Supports graceful shutdown with configurable timeout
   - Handles SIGINT and SIGTERM signals
   - Creates `RequestContext` for each request with App instance

2. **GormFeature**: GORM database connection
   - Supports MySQL and SQLite
   - Configurable connection pool
   - Provides both `*gorm.DB` and `*sql.DB` to DI container

3. **RedisFeature**: Redis client with service interface
   - Provides `feature.RedisService` interface to DI container
   - Methods: `Get`, `Set`, `Delete`, `Exists`

4. **JWTFeature**: JWT token management
   - Token generation and validation
   - Refresh token support
   - Token blacklist using Redis
   - Provides `feature.JWTService` interface to DI container

5. **I18NFeature**: Internationalization support
   - Multi-language translation using go-i18n
   - Automatic language detection from HTTP headers and query parameters
   - Supports YAML (recommended), TOML, and JSON locale files
   - Framework locale files are embedded in the binary using `go:embed` (always available)
   - Application locale files loaded from configured directory (can override framework messages)
   - Provides `contracts.Translator` interface to DI container
   - Integrated with `RequestContext` for easy translation in handlers

6. **Logger**: Structured logging support
   - Three log levels: Error (always logged), Info, Debug
   - Environment-based configuration via `LOG_LEVEL` (explicit) or `RUN_LEVEL` (automatic)
   - Automatic log level selection based on `RUN_LEVEL` if `LOG_LEVEL` is not set
   - Error logs go to stderr, Info/Debug logs go to stdout
   - Includes timestamp and file location in log output
   - Global logger functions available without initialization

### Route Handling

Routes use `contracts.CustomizedHandlerFunc` signature:

```go
type CustomizedHandlerFunc func(*RequestContext) (interface{}, bizerr.BizError)
```

The `contracts.Route` struct supports:

- `Method`: HTTP method (GET, POST, PUT, DELETE, PATCH)
- `Path`: Route path
- `Handler`: CustomizedHandlerFunc for business logic
- `Middlewares`: Optional slice of `gin.HandlerFunc` for route-specific middleware

**Middleware Support**:

You can attach Gin middlewares to specific routes. Middlewares are executed in the order they are defined, before the main handler:

```go
app.RegisterRoutes([]contracts.Route{
    {
        Method:  "GET",
        Path:    "/public",
        Handler: publicHandler,
        // No middleware - public endpoint
    },
    {
        Method:      "GET",
        Path:        "/protected",
        Handler:     protectedHandler,
        Middlewares: []gin.HandlerFunc{jwtAuthMiddleware, rateLimitMiddleware},
        // Middlewares execute in order: jwtAuthMiddleware → rateLimitMiddleware → protectedHandler
    },
})
```

Handlers receive `*contracts.RequestContext` which:

- Embeds `*gin.Context` - all Gin methods are available
- Contains `App contracts.App` - direct access to DI container

Handlers return:

- `(data, nil)`: Success response (HTTP 200)
- `(nil, bizErr)`: Error response (HTTP code from `bizErr.HTTPCode()`)

**Example**:

```go
import (
    "errors"
    "github.com/shyandsy/aurora/contracts"
    "github.com/shyandsy/aurora/bizerr"
)

func getUserHandler(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
    userID := c.Param("id")  // Gin method available
    if userID == "" {
        return nil, bizerr.ErrBadRequest(errors.New("user ID is required"))
    }
    
    // Access DI container directly
    var userService UserService
    if err := c.App.Find(&userService); err != nil {
        return nil, bizerr.ErrInternalServerError(err)
    }
    
    // Your business logic
    user := userService.GetUser(userID)
    if user == nil {
        return nil, bizerr.ErrNotFound()
    }
    
    return user, nil
}
```

### Error Handling

Aurora provides unified error handling through `bizerr.BizError`:

```go
// Standard errors
bizerr.ErrBadRequest(err)
bizerr.ErrUnauthorized()
bizerr.ErrForbidden()
bizerr.ErrNotFound()
bizerr.ErrInternalServerError(err)

// Validation errors
bizerr.NewValidationError("message", map[string]string{
    "field1": "error message 1",
    "field2": "error message 2",
})

// Single field validation
bizerr.NewSingleFieldError("email", "invalid email format")

// Multiple field validation
bizerr.NewMultipleFieldErrors(map[string]string{
    "email": "invalid email",
    "password": "password too short",
})
```

**Custom Error JSON Structure**

By default, error responses use the format `{"message": "..."}` with the HTTP status code from `bizerr.BizError`. To use your own error response format (e.g. custom fields, error codes, or i18n), implement the `contracts.ErrorHandler` interface and pass it when creating the server:

```go
package main

import (
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/shyandsy/aurora/app"
    "github.com/shyandsy/aurora/contracts"
    "github.com/shyandsy/aurora/feature"
    "github.com/shyandsy/aurora/bizerr"
)

// MyErrorHandler implements contracts.ErrorHandler to customize error response JSON.
type MyErrorHandler struct{}

func (MyErrorHandler) HandleError(c *gin.Context, err error) {
    code := http.StatusInternalServerError
    msg := err.Error()
    if e, ok := err.(bizerr.BizError); ok {
        code = e.HTTPCode()
        msg = e.Message()
    }
    c.JSON(code, gin.H{
        "code":    code,
        "message": msg,
        "error":   err.Error(),
        // Add any custom fields you need
    })
}

func main() {
    a := app.NewApp()
    a.AddFeature(feature.NewServerFeature(
        feature.WithErrorHandler(MyErrorHandler{}),
    ))
    a.AddFeature(feature.NewGormFeature())
    // ... register routes and run
}
```

- If you do **not** pass `WithErrorHandler`, the default format `{"message": "..."}` is used.
- If you pass `WithErrorHandler(handler)`, all handler errors are sent using your `HandleError(c, err)` implementation, so you control the full JSON body and status code.

### Database Migrations

Migrations are automatically run on startup when using `bootstrap.InitDefaultApp()`.

Migration files should be placed in the `migrations/` directory relative to the working directory.

**Migration Configuration**:

- `GOOSE_TABLE_PREFIX`: Optional prefix for goose version table name (optional)
  - If set, goose version table will use this prefix (e.g., `admin_goose_db_version`)
  - If not set or empty, goose default table name `goose_db_version` will be used

**Example**:

```bash
# Use default table name "goose_db_version"
# (no environment variable needed)

# Use custom table name "admin_goose_db_version"
GOOSE_TABLE_PREFIX=admin_
```

**Migration File Format**:

```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE users (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    email VARCHAR(255) NOT NULL UNIQUE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS users;
-- +goose StatementEnd
```

After migrations complete, the current migration version is logged.

### Dependency Injection

Aurora integrates `github.com/shyandsy/di` for dependency injection:

```go
// Provide dependencies
app.Provide(&myService)

// Resolve dependencies
var service *MyService
app.Resolve(&service)

// Provide with interface
app.ProvideAs(impl, (*MyInterface)(nil))

// Resolve with interface
var service MyInterface
app.Find(&service)
```

**In Handlers**:

```go
func myHandler(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
    // Access DI container directly from RequestContext
    var service MyService
    if err := c.App.Find(&service); err != nil {
        return nil, bizerr.ErrInternalServerError(err)
    }
    
    // Use service
    return service.DoSomething(), nil
}
```

Features can use struct tags for automatic injection:

```go
type MyFeature struct {
    Config   *config.ServerConfig `inject:""`
    DB       *gorm.DB             `inject:""`
    RedisSvc feature.RedisService `inject:""`
}
```

## Custom Features

Implement the `contracts.Features` interface to create custom features:

```go
type MyFeature struct {
    Config *config.ServerConfig `inject:""`
    DB    *gorm.DB             `inject:""`
}

func NewMyFeature() contracts.Features {
    return &MyFeature{}
}

func (f *MyFeature) Name() string {
    return "myfeature"
}

func (f *MyFeature) Setup(app contracts.App) error {
    // Resolve dependencies
    if err := app.Resolve(f); err != nil {
        return err
    }
    
    // Initialize your feature
    // Provide services to DI container
    app.Provide(f)
    
    return nil
}

func (f *MyFeature) Close() error {
    // Cleanup resources
    return nil
}
```

## Logging

Aurora provides a built-in structured logger that can be used throughout your application:

```go
import "github.com/shyandsy/aurora/logger"

func myHandler(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
    // Log error (always logged regardless of log level)
    logger.Error("Failed to process request: %+v", err)
    
    // Log info (logged when LOG_LEVEL is info or debug)
    logger.Info("Processing request for user: %s", userID)
    
    // Log debug (only logged when LOG_LEVEL is debug)
    logger.Debug("Request details: %+v", requestData)
    
    // Alternative format functions
    logger.Errorf("Error: %s", err.Error())
    logger.Infof("Info: %s", message)
    logger.Debugf("Debug: %s", debugInfo)
}
```

**Log Output Format**:

```text
[ERROR] 2024/12/08 14:30:45 service.go:123: Failed to process request: database connection failed
[INFO] 2024/12/08 14:30:45 handler.go:45: Processing request for user: 12345
[DEBUG] 2024/12/08 14:30:45 handler.go:46: Request details: map[method:GET path:/api/users]
```

**Best Practices**:

- Use `logger.Error()` for errors that need attention (always logged)
- Use `logger.Info()` for important application events
- Use `logger.Debug()` for detailed debugging information
- Include context variables in log messages (e.g., `orderNo`, `customerID`, `error=%+v`)
- Use `%+v` format for errors to include stack traces when available

**Example with Context**:

```go
func cancelOrder(ctx *contracts.RequestContext, orderNo string, customerID int64) bizerr.BizError {
    // Log error with context variables
    logger.Error("CancelOrder: failed to get order, orderNo=%s, customerID=%d, error=%+v", 
        orderNo, customerID, err)
    
    // Return generic error to client (don't expose database details)
    msg := ctx.T("error.internal_server")
    return bizerr.ErrInternalServerError(errors.New(msg))
}
```

## Health Checks

Aurora automatically registers two health check endpoints:

- `GET /health`: Returns service status, name, version, and timestamp
- `GET /ready`: Returns service readiness status

## License

MIT
