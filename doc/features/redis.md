# Redis Feature

封装 go-redis(v8),把 `RedisService` 接口注入 DI 容器。除常规 KV/Hash 操作外,还提供带自动续租的分布式锁 `WithLock`。

源码:[feature/redis.go](../../feature/redis.go)、[config/redis.go](../../config/redis.go)

---

## 创建 & 用法

```go
app.AddFeature(feature.NewRedisFeature())
```

以**接口类型**注入([feature/redis.go:69](../../feature/redis.go#L69)):

```go
type someService struct {
    Redis feature.RedisService `inject:""`
}
```

---

## RedisService 方法表

[feature/redis.go:15](../../feature/redis.go#L15) 接口,逐个:

| 方法 | 签名 | 备注 |
|---|---|---|
| Get | `Get(ctx, key) (string, error)` | **key 不存在返回 `("", nil)`**(吞掉 `redis.Nil`) |
| Set | `Set(ctx, key, value, expiration) error` | `expiration=0` = 永不过期 |
| Delete | `Delete(ctx, keys...) (int64, error)` | 注意方法名是 `Delete` 不是 `Del` |
| Exists | `Exists(ctx, key) (bool, error)` | |
| Incr | `Incr(ctx, key) (int64, error)` | |
| SetNX | `SetNX(ctx, key, value, expiration) (bool, error)` | 成功抢到返回 true |
| HSet | `HSet(ctx, key, field, value) error` | 单 field |
| HGet | `HGet(ctx, key, field) (string, error)` | 不存在返回 `("", nil)` |
| HDel | `HDel(ctx, key, fields...) (int64, error)` | |
| HGetAll | `HGetAll(ctx, key) (map[string]string, error)` | |
| HExists | `HExists(ctx, key, field) (bool, error)` | |
| HKeys | `HKeys(ctx, key) ([]string, error)` | |
| Expire | `Expire(ctx, key, expiration) error` | |
| WithLock | `WithLock(ctx, key, value, ttl, fn) error` | 见下 |

> ⚠️ `Get` / `HGet` 把 "不存在" 和 "空值" 都返回成 `("", nil)` —— 判断存在性要用 `Exists`,别靠 `Get` 的错误。

---

## WithLock —— 分布式锁(重点)

[feature/redis.go:180](../../feature/redis.go#L180):

```go
err := redis.WithLock(ctx, "lock:daily-settle", instanceID, 30*time.Second, func() error {
    return doHeavyJob()   // 只有抢到锁的实例会执行
})
if errors.Is(err, feature.ErrLockNotAcquired) {
    // 别的实例正在跑,本次跳过(不是错误)
}
```

**语义 —— 是 "skip-if-running",不是排队等待**:

- **获取**:`SetNX(key, value, ttl)`。抢不到锁 → **立即返回 `ErrLockNotAcquired`,`fn` 不执行**([feature/redis.go:186](../../feature/redis.go#L186))。不阻塞、不重试。
- **自动续租**:后台 goroutine 每 `ttl/2`(最小 1s)续一次([feature/redis.go:197](../../feature/redis.go#L197))。续租前先 `GET` 校验锁值仍是自己写的 `value` 才 `EXPIRE`,防止续别人的锁。→ **`fn` 跑再久也不会因 ttl 到期被别人抢走**,同时进程崩了锁按 ttl 自动释放。
- **释放**:`fn` 返回后 `defer` 里停续租 goroutine,再用独立 5s 超时 `DEL key`。释放失败只告警(靠 ttl 兜底)。
- **返回值**:直接透传 `fn()` 的返回。

用途:周期性后台任务防重复执行(多副本 worker 里,同一任务只有一个实例真跑)。

> ⚠️ 释放时直接 `DEL`,**没做 value 比对(无 Lua CAS)**。理论上若 `fn` 执行超过 ttl 且续租失败、锁被他人抢占,释放会误删他人锁。实践中 ttl 给足 + 续租正常时不触发,但要清楚这个边界。

导出错误:`var ErrLockNotAcquired`([feature/redis.go:172](../../feature/redis.go#L172))。

---

## 连接池 & 版本

- 用的是 **go-redis v8**(`github.com/go-redis/redis/v8`)。
- `provideRedis` 只设了 `Addr/Password/DB`,**没设 `PoolSize`**([feature/redis.go:80](../../feature/redis.go#L80))→ 走 go-redis 默认池大小(`10 × GOMAXPROCS`)。高并发 + 无 CPU 限制时每容器 `GOMAXPROCS` 可能偏大,注意评估。

---

## 环境变量

| Env | 字段 | 必填 | 说明 |
|---|---|---|---|
| `REDIS_ADDR` | Addr | 是 | `host:port` |
| `REDIS_PASSWORD` | Password | **是(非空)** | 无密码 Redis 也得填非空占位(如 `none`) |
| `REDIS_DB` | DB | 是(≥0) | DB 编号 |

> ⚠️ `REDIS_PASSWORD` **强制非空**([config/redis.go:18](../../config/redis.go#L18))—— 这是常见的启动失败原因。无密码环境填任意非空串。无 `PoolSize` 等池相关 env。
