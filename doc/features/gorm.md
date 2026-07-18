# Gorm Feature(数据库)

封装 GORM,负责建连接、配连接池、把 `*gorm.DB` 和 `*sql.DB` 注入 DI 容器。

源码:[feature/gorm.go](../../feature/gorm.go)、[config/database.go](../../config/database.go)

---

## 创建 & 用法

```go
app.AddFeature(feature.NewGormFeature())
```

`Setup` 后,容器里有两个可注入对象([feature/gorm.go:47](../../feature/gorm.go#L47)):

```go
type customerDatalayer struct {
    DB *gorm.DB `inject:""`   // ORM,业务查询用这个
}

// 或需要底层连接池句柄时:
var sqlDB *sql.DB
app.Find(&sqlDB)             // migration 就是拿它跑 goose
```

`Close()` 关闭底层 `sql.DB`([feature/gorm.go:53](../../feature/gorm.go#L53))。

---

## 支持的 driver

`DB_DRIVER` 取值([feature/gorm.go:72](../../feature/gorm.go#L72)):

| 值 | 驱动 |
|---|---|
| `mysql` | gorm.io/driver/mysql |
| `sqlite` | gorm.io/driver/sqlite |
| `postgres` / `postgresql` / `pgx` | gorm.io/driver/postgres(三个别名等价) |

其它值 → 启动即 `log.Fatalf`。gorm 日志固定 Info 级([feature/gorm.go:65](../../feature/gorm.go#L65),不可配)。

---

## 连接池

[feature/gorm.go:92](../../feature/gorm.go#L92):

```go
sqlDB.SetMaxIdleConns(MaxIdleConns)      // 无条件设
sqlDB.SetMaxOpenConns(MaxOpenConns)      // 无条件设
if ConnMaxLifetime > 0 { sqlDB.SetConnMaxLifetime(ConnMaxLifetime) }  // 可选
if ConnMaxIdleTime > 0 { sqlDB.SetConnMaxIdleTime(ConnMaxIdleTime) }  // 可选
```

四个旋钮的语义:

| 参数 | 作用 | 不设时 |
|---|---|---|
| `MaxOpenConns` | 连接总数硬上限 | 必填 |
| `MaxIdleConns` | 保留的空闲连接数(高峰后池回落到这个数) | 必填,须 ≤ MaxOpenConns |
| `ConnMaxLifetime` | 单连接最长可复用时长,到点回收 | 0 = 永不因年龄回收(可能被 MySQL `wait_timeout` 关掉后留下死连接) |
| `ConnMaxIdleTime` | 空闲连接最长保留时长,到点关闭 | 0 = 空闲连接不因时长关闭,只受 MaxIdleConns 数量约束 |

> **多服务共库要算总账**:所有连同一 MySQL 的服务,`MaxOpenConns × 副本数` 的**总和**必须 < MySQL `max_connections`,否则高并发下 `Error 1040: Too many connections`。
>
> `ConnMaxLifetime` 建议设为小于 DB `wait_timeout`(如 `1h`),规避低峰期陈旧连接;`ConnMaxIdleTime`(如 `5m`)能让闲置服务在低峰释放连接、给 DB 腾余量。二者都是**可选、向后兼容**(不设 = 保持原行为)。

---

## 环境变量

| Env | 字段 | 必填 | 说明 |
|---|---|---|---|
| `DB_DRIVER` | Driver | 是 | 见上表 |
| `DB_DSN` | DSN | 是 | 数据源连接串 |
| `DB_MAX_IDLE_CONNS` | MaxIdleConns | 是(>0,≤MaxOpen) | 空闲连接数 |
| `DB_MAX_OPEN_CONNS` | MaxOpenConns | 是(>0) | 连接总上限 |
| `DB_CONN_MAX_LIFETIME` | ConnMaxLifetime | 否(omitempty) | duration;0/不设=永不回收 |
| `DB_CONN_MAX_IDLE_TIME` | ConnMaxIdleTime | 否(omitempty) | duration;0/不设=不按空闲时长关 |

校验规则见 [config/database.go:30](../../config/database.go#L30)。迁移用同一份 `DatabaseConfig` 决定 goose 方言,见 [migration.md](./migration.md)。
