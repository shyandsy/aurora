# Aurora full_showcase

Full Aurora sample: Server, GORM, Redis, JWT, i18n, migrations, layered structure (controller / service / datalayer), RBAC, auth.

## Directory structure

```
full_showcase/
├── cmd/              # Entry
│   ├── main.go       # App bootstrap
│   └── providers.go  # DI registration
├── controller/       # HTTP layer
│   ├── routes.go
│   └── [module]/
├── service/          # Business logic
│   └── [module]/
├── datalayer/        # Data access
│   └── [module].go
├── model/
│   ├── entity/
│   └── dto/
├── migrations/
├── locales/
├── common/
│   ├── middleware/
│   └── model/
└── .env
```

## Core concepts

### App bootstrap

```go
// cmd/main.go
func main() {
    godotenv.Load()
    app := app.NewApp()
    app.AddFeature(auroraFeature.NewServerFeature())
    app.AddFeature(auroraFeature.NewGormFeature())
    app.AddFeature(auroraFeature.NewRedisFeature())
    app.AddFeature(auroraFeature.NewJWTFeature())
    app.AddFeature(auroraFeature.NewI18NFeature())
    migration.RunMigrations(app)
    registerProviders(app)
    app.RegisterRoutes(controller.GetRoutes(app))
    app.Run()
}
```

You can use `bootstrap.InitDefaultApp()` instead; it includes the Mail feature.

### Dependency injection

Register in `cmd/providers.go`. Register datalayers before services that depend on them.

### Routes

Define in `controller/routes.go`; use `middleware.JWTMiddleware(app, "user.get")` for protected routes.

### Layers

- **Controller** — HTTP request/response, calls services via `app.Find(&service)`.
- **Service** — Business logic, calls datalayers via `app.Find(&datalayer)`.
- **Datalayer** — DB access with `DB.WithContext(ctx)`.

### Migrations

SQL files in `migrations/` run on startup.

## Quick start

### 1. Configuration

Create a `.env` in this directory (or set env vars). Priority: system env > `.env`.

**Required (see Configuration below for full reference):** Server (HOST, PORT, SERVICE_NAME, RUN_LEVEL), Database (DB_DRIVER, DB_DSN, DB_MAX_IDLE_CONNS, DB_MAX_OPEN_CONNS), Redis (REDIS_ADDR, REDIS_PASSWORD, REDIS_DB), JWT (JWT_SECRET, JWT_EXPIRE_TIME, JWT_ISSUER).

### 2. Database

For MySQL, create the DB:

```sql
CREATE DATABASE sample_db CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

### 3. Run

```bash
# From Aurora repo root: go into the sample, then full_showcase
cd sample/full_showcase

GOWORK=off go run ./cmd

# Or build
GOWORK=off go build -o sample ./cmd
./sample
```

Use `go run ./cmd` (not `go run cmd/main.go`) so both `main.go` and `providers.go` are included.

---

## Configuration

### Server

| Variable | Description | Default |
|----------|-------------|---------|
| `HOST` | Listen address | `0.0.0.0` |
| `PORT` | Listen port | `8080` |
| `SERVICE_NAME` | Service name | `myapp` |
| `SERVICE_VERSION` | Service version | `1.0.0` |
| `RUN_LEVEL` | `local`, `stage`, `production` | `local` |

### Database

| Variable | Description | Example |
|----------|-------------|---------|
| `DB_DRIVER` | `mysql` or `sqlite` | `mysql` |
| `DB_DSN` | Connection string | See below |
| `DB_MAX_IDLE_CONNS` | Max idle connections | `10` |
| `DB_MAX_OPEN_CONNS` | Max open connections | `100` |

- **MySQL DSN:** `user:password@tcp(host:port)/dbname?charset=utf8mb4&parseTime=True&loc=Local`
- **SQLite DSN:** `file:database.db?cache=shared&mode=rwc`

### Redis

| Variable | Description | Example |
|----------|-------------|---------|
| `REDIS_ADDR` | Redis address | `localhost:6379` |
| `REDIS_PASSWORD` | Password (use `none` if no auth) | — |
| `REDIS_DB` | DB index | `0` |

Redis is required for JWT token blacklist.

### JWT

| Variable | Description | Example |
|----------|-------------|---------|
| `JWT_SECRET` | Secret (change in production) | — |
| `JWT_EXPIRE_TIME` | Token TTL, e.g. `24h`, `168h` | `24h` |
| `JWT_ISSUER` | Issuer | `sample-service` |

### I18N (optional)

| Variable | Description | Default |
|----------|-------------|---------|
| `I18N_DEFAULT_LANG` | Default language | `en` |
| `I18N_SUPPORTED_LANGS` | Comma-separated langs | `en,zh-CN` |
| `I18N_LOCALE_DIR` | Locale files dir | `locales` |

### Example .env (development)

```bash
HOST=0.0.0.0
PORT=8080
SERVICE_NAME=full_showcase
RUN_LEVEL=local

DB_DRIVER=mysql
DB_DSN=root:password@tcp(localhost:3306)/sample_db?charset=utf8mb4&parseTime=True&loc=Local
DB_MAX_IDLE_CONNS=10
DB_MAX_OPEN_CONNS=100

REDIS_ADDR=localhost:6379
REDIS_PASSWORD=none
REDIS_DB=0

JWT_SECRET=your-secret-key
JWT_EXPIRE_TIME=24h
JWT_ISSUER=sample-service
```

---

## Default data

After first run, the app may seed:

- **User:** `shyandsy@gmail.com` / `123456`, role `admin`
- **Roles:** `admin`, `user`
- **Features:** e.g. `user.get`, `user.create`, `role.get`, `customer.get`, etc.

---

## Notes

- **Production:** Set `RUN_LEVEL=production`, change `JWT_SECRET`, use strong DB/Redis config.
- **Redis:** If no password, set `REDIS_PASSWORD=none` (non-empty).
- **Port:** Change `PORT` if 8080 is in use.

## Framework

- **DI:** `app.ProvideAs` / `app.Find`
- **Migrations:** Automatic from `migrations/`
- **JWT:** Middleware + RBAC
- **i18n:** Multi-language support

More: [github.com/shyandsy/aurora](https://github.com/shyandsy/aurora)
