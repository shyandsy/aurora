# 数据库迁移(Migration)

基于 [goose](https://github.com/pressly/goose)(v3)的 SQL 迁移。不是 Feature 对象,而是一个在 `InitDefaultApp` 末尾调用的步骤。

源码:[migration/migration.go](../../migration/migration.go)、[config/migration.go](../../config/migration.go)

---

## 怎么跑

`InitDefaultApp` 在所有 feature 之后调用一次([bootstrap/default.go](../../bootstrap/default.go)):

```go
migration.RunMigrations(a)
```

`RunMigrations`([migration/migration.go](../../migration/migration.go))流程:
1. 从 DI 容器取 `*sql.DB`(gorm feature 已 Provide);取不到报错;
2. 按 `DB_DRIVER` 定 goose 方言(`postgres/postgresql/pgx`→postgres,`sqlite`→sqlite3,`mysql`→mysql);
3. 读 `GOOSE_TABLE_PREFIX` 定版本表名;
4. 跑**两套有序迁移**(见下)。

两套都读进程工作目录下的普通磁盘目录(goose 默认 osFS),没有任何内嵌 / Provider 机制。没有迁移文件不算错误(返回 nil,跳过)。

## 两套有序迁移:schema 套 + data 套

一个服务可以有两套迁移,**schema 套恒先于 data 套**执行,各用各的版本表:

| 套 | 目录 | 版本表 | 归谁维护 | 何时跑 |
|---|---|---|---|---|
| schema 套(可选) | `<cwd>/migrations-schema/` | `{prefix}schema_goose_db_version` | 共享模块(如 user 模块自带的建表 / 改表结构迁移) | 有该目录且含 `.sql` 才跑,**先跑** |
| data 套 | `<cwd>/migrations/` | `{prefix}goose_db_version` | 消费项目(自己的业务数据迁移) | 总是尝试,**后跑** |

**为什么要分成两套两张表**:共享模块(如 user)的表结构后续还可能调整,得能独立演进。两套各自独立编号、各记各的进度,项目的业务迁移号永远不会和模块的 schema 迁移号相撞——模块加新的结构迁移,不需要项目把自己的迁移号往后挪。

**没有 `migrations-schema/` 的服务就是单套**,只跑 data 套,行为与历史完全一致(向后兼容)。homeserver 的 api-user 服务用到两套:aurora user 模块把建表/改表迁移放在模块自带的 `migrations-schema/`、打进镜像;homeserver 把自己的权限业务数据迁移放在 `services/user/migrations/`、打进镜像的 `migrations/`。

## 版本表前缀(多服务共库关键)

多个服务共用一个数据库时,各自的 goose 版本表要隔离,否则版本号互相打架。用 `GOOSE_TABLE_PREFIX`([config/migration.go](../../config/migration.go)):

| `GOOSE_TABLE_PREFIX` | data 套版本表 | schema 套版本表 |
|---|---|---|
| (不设) | `goose_db_version` | `schema_goose_db_version` |
| `admin_` | `admin_goose_db_version` | `admin_schema_goose_db_version` |
| `user_` | `user_goose_db_version` | `user_schema_goose_db_version` |

homeserver 里 admin/customer/schedule/user 共库,就是靠不同前缀各记各的迁移进度。

## worker 进程不该跑迁移

迁移要独占执行,多个进程并发跑 goose 会抢锁 / 冲突。所以:
- **API 进程**用 `InitDefaultApp`(含迁移);
- **worker 进程**不要用 `InitDefaultApp`,自己 `app.NewApp()` + 手动 `AddFeature` 需要的子集,**跳过 `RunMigrations`**。

homeserver 的 `wire.InitWorkerApp()` 就是这个模式。

## 环境变量

| Env | 必填 | 说明 |
|---|---|---|
| `GOOSE_TABLE_PREFIX` | ❌(omitempty) | 版本表前缀;不设 = `goose_db_version` / `schema_goose_db_version` |
| 复用 `DB_DRIVER` | — | 决定 goose 方言 |

> 迁移的 DB 连接直接复用 DI 里的 `*sql.DB`,无独立 DSN;两套目录固定 `migrations/` 与 `migrations-schema/`,无 env 可改。
