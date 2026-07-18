# 数据库迁移(Migration)

基于 [goose](https://github.com/pressly/goose)(v3)的 SQL 迁移。不是 feature,而是一个在 `InitDefaultApp` 末尾调用的步骤。

源码:[migration/migration.go](../../migration/migration.go)、[config/migration.go](../../config/migration.go)

---

## 怎么跑

`InitDefaultApp` 在所有 feature 之后调用一次([bootstrap/default.go:25](../../bootstrap/default.go#L25)):

```go
migration.RunMigrations(a)
```

`RunMigrations`([migration/migration.go:17](../../migration/migration.go#L17))流程:
1. 从 DI 容器取 `*sql.DB`(gorm feature 已 Provide);取不到报错;
2. 按 `DB_DRIVER` 定 goose 方言(`postgres/postgresql/pgx`→postgres,`sqlite`→sqlite3,`mysql`→mysql);
3. 读 `GOOSE_TABLE_PREFIX` 定版本表名;
4. `goose.Up(sqlDB, <cwd>/migrations)`。

**迁移文件目录固定为进程工作目录下的 `migrations/`**([migration/migration.go:68](../../migration/migration.go#L68)),不可配。没有迁移文件不算错误(返回 nil)。

## 版本表前缀(多服务共库关键)

多个服务共用一个数据库时,各自的 goose 版本表要隔离,否则版本号互相打架。用 `GOOSE_TABLE_PREFIX`([config/migration.go:10](../../config/migration.go#L10)):

| `GOOSE_TABLE_PREFIX` | 版本表名 |
|---|---|
| (不设) | `goose_db_version` |
| `admin_` | `admin_goose_db_version` |
| `customer_` | `customer_goose_db_version` |

homeserver 里 admin/customer/schedule 共库,就是靠不同前缀各记各的迁移进度。

## worker 进程不该跑迁移

迁移要独占执行,多个进程并发跑 goose 会抢锁 / 冲突。所以:
- **API 进程**用 `InitDefaultApp`(含迁移);
- **worker 进程**不要用 `InitDefaultApp`,自己 `app.NewApp()` + 手动 `AddFeature` 需要的子集,**跳过 `RunMigrations`**。

homeserver 的 `wire.InitWorkerApp()` 就是这个模式。

## 环境变量

| Env | 必填 | 说明 |
|---|---|---|
| `GOOSE_TABLE_PREFIX` | ❌(omitempty) | 版本表前缀;不设 = `goose_db_version` |
| 复用 `DB_DRIVER` | — | 决定 goose 方言 |

> 迁移的 DB 连接直接复用 DI 里的 `*sql.DB`,无独立 DSN;目录固定 `migrations/`,无 env 可改。
