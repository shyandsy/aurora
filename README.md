# Aurora Framework

A lightweight, modular web framework for Go, built on top of Gin.

## Features

- 🚀 **Modular Architecture**: Feature-based design for easy extensibility
- 🔌 **Dependency Injection**: Built-in DI container for clean dependency management
- 🗄️ **Database Support**: GORM integration with MySQL and SQLite
- 🔐 **JWT Authentication**: Built-in JWT token management
- 📦 **Redis Support**: Redis integration for caching and session management
- 🔄 **Database Migrations**: Goose-based migration system
- ⚙️ **Configuration Management**: Environment-based configuration loading
- 🛡️ **Error Handling**: Unified business error handling
- 🌐 **CORS Support**: Configurable CORS middleware

## Installation

```bash
go get github.com/shyandsy/aurora
```

## Quick Start

```go
package main

import (
    "log"
    
    "github.com/gin-gonic/gin"
    auroraApp "github.com/shyandsy/aurora/app"
    "github.com/shyandsy/aurora/bizerr"
    "github.com/shyandsy/aurora/route"
)

func main() {
    // Create application
    app := auroraApp.NewApp()
    
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

## Configuration

Aurora uses environment variables for configuration. See the `config` package for available options.

### Server Configuration

- `HOST`: Server host (default: `0.0.0.0`)
- `PORT`: Server port (default: `8080`)
- `SERVICE_NAME`: Service name
- `SERVICE_VERSION`: Service version
- `RUN_LEVEL`: Run level (`local`, `stage`, `production`)

### Database Configuration

- `DB_DRIVER`: Database driver (`mysql`, `sqlite`)
- `DB_DSN`: Database connection string

### Redis Configuration

- `REDIS_ADDR`: Redis address
- `REDIS_PASSWORD`: Redis password (optional)
- `REDIS_DB`: Redis database number

### JWT Configuration

- `JWT_SECRET`: JWT secret key
- `JWT_ACCESS_TOKEN_EXPIRY`: Access token expiry (e.g., `15m`)
- `JWT_REFRESH_TOKEN_EXPIRY`: Refresh token expiry (e.g., `7d`)

## Features

### Built-in Features

- **GormFeature**: Database connection management
- **RedisFeature**: Redis client management
- **JWTFeature**: JWT token generation and validation
- **ServerFeature**: HTTP server with routing and health checks

### Custom Features

Implement the `contracts.Features` interface to create custom features:

```go
type MyFeature struct {
    // Your dependencies
}

func (f *MyFeature) Name() string {
    return "myfeature"
}

func (f *MyFeature) Setup(app contracts.App) error {
    // Initialize your feature
    return nil
}

func (f *MyFeature) Close() error {
    // Cleanup
    return nil
}
```

## License

MIT
