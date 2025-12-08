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
- 🔒 **Route Middlewares**: Support for route-specific Gin middlewares (e.g., JWT authentication, rate limiting)
- 🏥 **Health Checks**: Built-in `/health` and `/ready` endpoints
- 📝 **Request Context**: Extended request context with App instance for easy dependency access
- 🌍 **Internationalization (i18n)**: Multi-language support using go-i18n with automatic language detection
- 📊 **Structured Logging**: Built-in logger with log levels (Error, Info, Debug) and environment-based configuration

## Installation

```bash
go get github.com/shyandsy/aurora
```

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

**Example**:

```bash
JWT_SECRET=your-super-secret-jwt-key-here-change-in-production
JWT_EXPIRE_TIME=15m
JWT_ISSUER=myapp
```

**Note**: Refresh tokens expire after `JWT_EXPIRE_TIME * 24` (e.g., 15m * 24 = 6 hours).

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

- **Framework locale files** are embedded in the Aurora binary using `go:embed` and are always loaded automatically. They are located at `api/aurora/feature/i18n/` in the source code.
- **Application locale files** should be placed in the directory specified by `I18N_LOCALE_DIR` (relative to your application's working directory).
- Application locale files can override framework messages with the same message ID.

**Locale File Format**:

Create language files using **flat structure** (not nested). The framework supports multiple formats with the following priority:

1. **YAML** (`.yaml` or `.yml`) - Recommended, most readable
2. **TOML** (`.toml`)
3. **JSON** (`.json`)

**Framework locale file example** (`api/aurora/feature/i18n/en.yaml`):

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

#### Features

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

### Database Migrations

Migrations are automatically run on startup when using `bootstrap.InitDefaultApp()`.

Migration files should be placed in the `migrations/` directory relative to the working directory.

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

```
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
