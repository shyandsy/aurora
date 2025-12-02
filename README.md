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
- 🏥 **Health Checks**: Built-in `/health` and `/ready` endpoints

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
    "github.com/shyandsy/aurora/route"
    "github.com/shyandsy/aurora/bizerr"
    "github.com/gin-gonic/gin"
)

func main() {
    // Create application with default features (Server, GORM, Redis, JWT)
    app := bootstrap.InitDefaultApp()
    
    // Register routes
    app.RegisterRoutes([]route.Route{
        {
            Method:  "GET",
            Path:    "/hello",
            Handler: func(c *gin.Context) (interface{}, bizerr.BizError) {
                return gin.H{"message": "Hello, Aurora!"}, nil
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
    "github.com/shyandsy/aurora/route"
    "github.com/shyandsy/aurora/bizerr"
    "github.com/gin-gonic/gin"
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
    a.RegisterRoutes([]route.Route{
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

func getUserHandler(c *gin.Context) (interface{}, bizerr.BizError) {
    // Your handler logic
    return gin.H{"users": []string{"user1", "user2"}}, nil
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

### CORS Configuration

- `CORS_ALLOWED_ORIGINS`: Comma-separated list of allowed origins (optional)
- `CORS_ALLOWED_METHODS`: Comma-separated list of allowed HTTP methods (optional)
- `CORS_ALLOWED_HEADERS`: Comma-separated list of allowed headers (optional)
- `CORS_ALLOWED_CREDENTIALS`: Allow credentials (optional, `true` or `false`)

**Note**: CORS is only enabled if at least one CORS configuration is provided.

## Architecture

### Core Components

#### App Interface

The `contracts.App` interface provides:

- `AddFeature(feature Features)`: Register a feature
- `RegisterRoutes(routes []route.Route)`: Register API routes
- `Run() error`: Start the application
- `Shutdown() error`: Gracefully shutdown the application
- `GetContainer() di.Container`: Get the DI container
- Direct access to `di.Container` methods (Provide, Resolve, etc.)

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

### Route Handling

Routes use `route.CustomizedHandlerFunc` signature:

```go
type CustomizedHandlerFunc func(*gin.Context) (interface{}, bizerr.BizError)
```

Handlers return:

- `(data, nil)`: Success response (HTTP 200)
- `(nil, bizErr)`: Error response (HTTP code from `bizErr.HTTPCode()`)

**Example**:

```go
func getUserHandler(c *gin.Context) (interface{}, bizerr.BizError) {
    userID := c.Param("id")
    if userID == "" {
        return nil, bizerr.ErrBadRequest(errors.New("user ID is required"))
    }
    
    // Your business logic
    user := getUserByID(userID)
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

## Health Checks

Aurora automatically registers two health check endpoints:

- `GET /health`: Returns service status, name, version, and timestamp
- `GET /ready`: Returns service readiness status

## License

MIT
