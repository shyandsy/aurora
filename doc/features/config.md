# 配置系统(ResolveConfig)

Aurora 所有配置都是「结构体 + `env` tag + `ResolveConfig` 从环境变量填充」。这是框架的核心机制之一,理解它才能正确加配置、避开几个致命坑。

源码:[config/resolve.go](../../config/resolve.go)、[config/error.go](../../config/error.go)

---

## 基本用法

```go
type MyConfig struct {
    APIKey  string        `env:"MY_API_KEY"`            // 必填
    Region  string        `env:"MY_REGION,omitempty"`   // 可选
    Timeout time.Duration `env:"MY_TIMEOUT,omitempty"`  // duration
    Hosts   []string      `env:"MY_HOSTS,omitempty"`    // 逗号分隔列表
}

cfg := &MyConfig{}
if err := config.ResolveConfig(cfg); err != nil { log.Fatal(err) }
if err := cfg.Validate(); err != nil { log.Fatal(err) }   // Validate 要自己调!
```

## `omitempty` 语义(必须理解)

[config/resolve.go:41](../../config/resolve.go#L41):环境变量取到**空串**时:

| tag | 行为 |
|---|---|
| `env:"X"`(无 omitempty) | **直接报 required 错误**,进程起不来 |
| `env:"X,omitempty"` | 跳过,字段保持 Go 零值 |

所以规则很简单:**可选字段必须加 `,omitempty`,否则缺值即启动失败**。

## ⚠️ 致命坑:`envDefault` 完全无效

`ResolveConfig` **只读 `env` tag,从不读 `envDefault`**([config/resolve.go:32](../../config/resolve.go#L32) 只调 `Tag.Get("env")`)。

但 [config/server.go:27](../../config/server.go#L27) 的 `ServerConfig` 却写了一堆 `envDefault:"..."`(`HOST` `envDefault:"0.0.0.0"` 等),**这些标签运行时一律不生效**,纯误导。更糟的是这些字段**没有 `omitempty`** → 它们实际上是**必填**,不设对应 env 直接报错。

**要默认值的正确做法**(二选一):

```go
// 做法 A:构造 config 时用结构体字面量预填默认值(推荐)
cfg := &MyConfig{ Timeout: 30 * time.Second }
config.ResolveConfig(cfg)   // env 有值则覆盖,没值则保留默认

// 做法 B:Validate 里兜底
func (c *MyConfig) Validate() error {
    if c.Timeout == 0 { c.Timeout = 30 * time.Second }
    return nil
}
```

**永远不要用 `envDefault`**,它是死标签。

## 支持的字段类型

[config/resolve.go:57](../../config/resolve.go#L57) `setFieldValue`:

| 类型 | 说明 |
|---|---|
| `string` | 直接赋值 |
| `int` 系列 | `strconv.ParseInt` + 溢出检查 |
| `time.Duration` | `time.ParseDuration`,值写 `"30s"` `"1h"` `"720h"` |
| `uint` / `float` 系列 | 对应 Parse + 溢出检查 |
| `bool` | `strconv.ParseBool`(`1/t/true/0/f/false`…) |
| `[]string` | 按 `,` 分割、逐段 TrimSpace、丢空段 |
| 其它 slice(如 `[]int`) | **不支持**,报错 |
| 其它类型 | **不支持**,报错 |

## config 结构体的隐式契约

每个 config 结构体约定实现两个方法:

- `Key() string` —— 稳定标识(`"server"` / `"redis"` / …),供按 key 注册查找。
- `Validate() error` —— 业务校验,返回 `*ConfigError`([config/error.go](../../config/error.go),`NewConfigError(msg)`)。

> ⚠️ **`ResolveConfig` 不会自动调 `Validate`**。固定模式是:`ResolveConfig(cfg)` → `cfg.Validate()`,两步都要自己写。框架各 feature 的 `Setup` 里就是这么做的(如 [feature/gorm.go:36](../../feature/gorm.go#L36))。

## 加自定义配置的完整范例

```go
package config

import "time"

type OSSConfig struct {
    Provider  string `env:"OSS_PROVIDER,omitempty"`   // 可选,空 = 关闭 OSS
    Endpoint  string `env:"OSS_ENDPOINT,omitempty"`
    Bucket    string `env:"OSS_BUCKET,omitempty"`
    KeyID     string `env:"OSS_ACCESS_KEY_ID,omitempty"`
    KeySecret string `env:"OSS_ACCESS_KEY_SECRET,omitempty"`
}

func (c *OSSConfig) Key() string { return "oss" }

func (c *OSSConfig) Validate() error {
    if c.Provider == "" {
        return nil   // 未配 = 功能关闭,不报错
    }
    if c.Endpoint == "" || c.Bucket == "" {
        return NewConfigError("OSS_ENDPOINT and OSS_BUCKET are required when OSS_PROVIDER is set")
    }
    return nil
}
```

三个要点:① 可选字段全加 `omitempty`;② 要默认值靠代码不靠 envDefault;③ `Validate` 自己实现、由 feature 的 `Setup` 显式调用。
