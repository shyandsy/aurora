# 配置说明

本文档说明 Aurora 框架的环境变量配置。

## 配置方式

Aurora 框架通过环境变量进行配置。项目启动时会自动加载根目录下的 `.env` 文件。

**配置优先级：**

1. 系统环境变量（优先级最高）
2. `.env` 文件中的值（如果环境变量未设置）

## 必需配置项

### Server 配置

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `HOST` | 服务器监听地址 | `0.0.0.0` |
| `PORT` | 服务器监听端口 | `8080` |
| `SERVICE_NAME` | 服务名称 | `myapp` |
| `SERVICE_VERSION` | 服务版本 | `1.0.0` |
| `RUN_LEVEL` | 运行级别: `local`, `stage`, `production` | `local` |

### Database 配置

| 变量名 | 说明 | 示例值 |
|--------|------|--------|
| `DB_DRIVER` | 数据库驱动: `mysql` 或 `sqlite` | `mysql` |
| `DB_DSN` | 数据库连接字符串 | `root:password@tcp(localhost:3306)/sample_db?charset=utf8mb4&parseTime=True&loc=Local` |
| `DB_MAX_IDLE_CONNS` | 最大空闲连接数 | `10` |
| `DB_MAX_OPEN_CONNS` | 最大打开连接数 | `100` |

**MySQL DSN 格式：**

```
user:password@tcp(host:port)/dbname?charset=utf8mb4&parseTime=True&loc=Local
```

**SQLite DSN 格式：**

```
file:database.db?cache=shared&mode=rwc
```

### Redis 配置

| 变量名 | 说明 | 示例值 |
|--------|------|--------|
| `REDIS_ADDR` | Redis 服务器地址 | `localhost:6379` |
| `REDIS_PASSWORD` | Redis 密码（无密码可设置为 "none"） | `your_redis_password` |
| `REDIS_DB` | Redis 数据库编号 | `0` |

**注意：** JWT 功能需要 Redis 来管理 token 黑名单，所以 Redis 是必需的。

### JWT 配置

| 变量名 | 说明 | 示例值 |
|--------|------|--------|
| `JWT_SECRET` | JWT 密钥（生产环境必须修改） | `your-super-secret-jwt-key-change-in-production` |
| `JWT_EXPIRE_TIME` | Token 过期时间 | `24h` |
| `JWT_ISSUER` | JWT 签发者标识 | `sample-service` |

**JWT_EXPIRE_TIME 格式：**

- `24h` - 24小时
- `720h` - 30天
- `168h` - 7天

## 可选配置项

### I18N 配置

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `I18N_DEFAULT_LANG` | 默认语言 | `en` |
| `I18N_SUPPORTED_LANGS` | 支持的语言列表（逗号分隔） | `en,zh-CN` |
| `I18N_LOCALE_DIR` | 国际化文件目录 | `locales` |

### Mail 配置（可选）

如果不需要邮件功能，可以不配置。

## 配置示例

### 开发环境

在项目根目录创建 `.env` 文件：

```bash
# Server
HOST=0.0.0.0
PORT=8080

# Database
DB_DRIVER=mysql
DB_DSN=root:password@tcp(localhost:3306)/sample_db?charset=utf8mb4&parseTime=True&loc=Local

# Redis
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=none
REDIS_DB=0

# JWT
JWT_SECRET=your-secret-key
JWT_EXPIRE_TIME=24h
JWT_ISSUER=sample-service
```

### 生产环境

通过环境变量传入配置：

```bash
export DB_DRIVER=mysql
export DB_DSN=user:pass@tcp(host:3306)/db?charset=utf8mb4&parseTime=True&loc=Local
export REDIS_ADDR=redis-server:6379
export JWT_SECRET=production-secret-key
# ... 其他环境变量
```

## 默认数据

项目启动后，数据库会自动创建以下默认数据：

- **默认用户：**
  - Email: `shyandsy@gmail.com`
  - Password: `123456`
  - Role: `admin`

- **默认角色：**
  - `admin` - 拥有所有权限
  - `user` - 普通用户

- **默认功能权限：**
  - `user.get`, `user.create`, `user.update`, `user.delete`
  - `role.get`, `role.create`, `role.update`, `role.delete`
  - `feature.get`, `feature.create`, `feature.update`, `feature.delete`
  - `rolefeature.get`, `rolefeature.create`, `rolefeature.delete`
  - `customer.get`, `customer.create`, `customer.update`, `customer.delete`

## 注意事项

1. **生产环境：**
   - 必须修改 `JWT_SECRET`，不能使用默认值
   - 设置 `RUN_LEVEL=production`
   - 使用强密码和安全的数据库配置

2. **Redis 密码：**
   - 如果 Redis 没有设置密码，`REDIS_PASSWORD` 不能为空字符串
   - 可以设置为 `"none"` 或其他非空值

3. **端口冲突：**
   - 如果 8080 端口被占用，修改 `PORT` 环境变量
