# Aurora 框架使用指南

这是 Aurora 框架的示例项目，展示如何使用 Aurora 框架构建完整的应用。

## 目录结构规范

Aurora 框架推荐的项目结构：

```
sample/
├── cmd/              # 主程序入口
│   ├── main.go       # 应用启动入口
│   └── providers.go  # 依赖注入配置
├── controller/       # HTTP 控制器层
│   ├── routes.go     # 路由定义
│   └── [module]/     # 各业务模块的控制器
├── service/          # 业务逻辑层
│   └── [module]/     # 各业务模块的服务
├── datalayer/        # 数据访问层
│   └── [module].go   # 各业务模块的数据访问
├── model/            # 数据模型
│   ├── entity/       # 数据库实体（对应数据库表）
│   └── dto/          # 数据传输对象（API 请求/响应）
├── migrations/       # 数据库迁移文件
├── locales/          # 国际化文件
├── common/           # 公共模块
│   ├── middleware/   # 中间件
│   └── model/        # 公共模型
└── .env              # 环境变量配置
```

## 核心概念

### 1. 应用初始化

```go
// cmd/main.go
func main() {
    // 加载 .env 文件
    godotenv.Load()
    
    // 创建应用实例
    app := app.NewApp()
    
    // 添加必需的 Feature
    app.AddFeature(auroraFeature.NewServerFeature())
    app.AddFeature(auroraFeature.NewGormFeature())
    app.AddFeature(auroraFeature.NewRedisFeature())
    app.AddFeature(auroraFeature.NewJWTFeature())
    app.AddFeature(auroraFeature.NewI18NFeature())
    
    // 运行数据库迁移
    migration.RunMigrations(app)
    
    // 注册依赖注入
    registerProviders(app)
    
    // 注册路由
    app.RegisterRoutes(controller.GetRoutes(app))
    
    // 启动服务
    app.Run()
}
```

**注意：** 也可以使用 `bootstrap.InitDefaultApp()` 快速初始化，但它会包含 Mail 功能。如果不需要邮件功能，建议手动初始化。

### 2. 依赖注入

在 `cmd/providers.go` 中注册所有依赖：

```go
func registerProviders(app contracts.App) {
    // 注册 Datalayer
    userDatalayer := datalayer.NewUserDatalayer(app)
    app.ProvideAs(userDatalayer, (*datalayer.UserDatalayer)(nil))
    
    // 注册 Service（依赖 Datalayer）
    userService := service.NewUserService(app)
    app.ProvideAs(userService, (*service.UserService)(nil))
}
```

**注册顺序很重要：** 先注册被依赖的组件（如 Datalayer），再注册依赖它的组件（如 Service）。

### 3. 路由定义

在 `controller/routes.go` 中定义所有路由：

```go
func GetRoutes(app contracts.App) []contracts.Route {
    return []contracts.Route{
        {
            Method:  http.MethodPost,
            Path:    "/auth/login",
            Handler: controller.Login(app),
        },
        {
            Method:  http.MethodGet,
            Path:    "/users",
            Handler: middleware.JWTMiddleware(app, "user.get")(controller.GetUsers(app)),
        },
    }
}
```

### 4. 控制器层

控制器负责处理 HTTP 请求和响应：

```go
func GetUsers(app contracts.App) gin.HandlerFunc {
    return func(c *gin.Context) {
        var userService service.UserService
        app.Find(&userService)
        
        users, err := userService.GetUsers(c.Request.Context())
        // ... 处理响应
    }
}
```

### 5. 服务层

服务层包含业务逻辑：

```go
type UserService interface {
    GetUsers(ctx context.Context) ([]*dto.User, error)
}

func (s *userService) GetUsers(ctx context.Context) ([]*dto.User, error) {
    var userDatalayer datalayer.UserDatalayer
    s.app.Find(&userDatalayer)
    
    users, err := userDatalayer.GetAll(ctx)
    // ... 业务逻辑处理
    return dtos, nil
}
```

### 6. 数据访问层

数据访问层负责数据库操作：

```go
type UserDatalayer interface {
    GetAll(ctx context.Context) ([]*entity.User, error)
}

func (d *userDatalayer) GetAll(ctx context.Context) ([]*entity.User, error) {
    var users []*entity.User
    err := d.DB.WithContext(ctx).Find(&users).Error
    return users, err
}
```

### 7. 数据库迁移

在 `migrations/` 目录下创建 SQL 文件，应用启动时会自动执行：

```sql
-- migrations/20251204035059_feature_39_user.sql
CREATE TABLE IF NOT EXISTS users (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    email VARCHAR(255) NOT NULL UNIQUE,
    -- ...
);
```

## 快速开始

### 1. 配置环境变量

在项目根目录创建 `.env` 文件，配置数据库、Redis、JWT 等参数。详细配置说明请参考 [配置说明](./README_CONFIG.md)。

### 2. 准备数据库

创建数据库（如果使用 MySQL）：

```sql
CREATE DATABASE sample_db CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

### 3. 启动应用

```bash
# 开发环境（推荐方式）
go run ./cmd

# 生产环境
go build -o sample ./cmd
./sample
```

**注意：** 必须使用 `go run ./cmd` 而不是 `go run cmd/main.go`，因为项目包含多个文件（`main.go` 和 `providers.go`）。

应用启动后会：

- 自动执行数据库迁移
- 启动 HTTP 服务器（默认端口：`8080`）

## 框架特性

- **依赖注入：** 使用 DI 容器管理依赖，支持接口注入
- **自动迁移：** 启动时自动执行 migrations 目录下的 SQL 文件
- **JWT 认证：** 内置 JWT 中间件和权限检查
- **国际化：** 支持多语言（i18n）
- **配置管理：** 通过环境变量配置，支持 `.env` 文件

## 依赖管理

```bash
go mod tidy    # 整理依赖
go mod vendor  # 更新 vendor 目录
```

## 更多信息

- **配置说明：** [README_CONFIG.md](./README_CONFIG.md) - 详细的环境变量配置说明
- **框架源码：** `github.com/shyandsy/aurora`
