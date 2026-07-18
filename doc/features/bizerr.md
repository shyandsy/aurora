# 错误模型(bizerr)与日志(logger)

## 一、bizerr —— 业务错误

Handler 返回 `(interface{}, bizerr.BizError)`,框架据此写出 HTTP 响应。业务里不要自己 `c.JSON` 写错误,而是 `return nil, bizerr.Xxx()`。

源码:[bizerr/bizerr.go](../../bizerr/bizerr.go)

### BizError 接口

[bizerr/bizerr.go:25](../../bizerr/bizerr.go#L25):

```go
type BizError interface {
    HTTPCode() int
    Message() string
    Error() string
    ValidationErrors() map[string]string
    IsValidationError() bool
}
```

> message 来自内部持有的 `err.Error()`,没有独立 message 字段。

### 构造函数

| 构造 | HTTP | 说明 |
|---|---|---|
| `New(httpCode, err)` | 自定义 | 通用底层构造 |
| `ErrBadRequest(err)` | 400 | 变量(函数值),收 error |
| `ErrInternalServerError(err)` | 500 | 变量,收 error |
| `ErrUnauthorized()` | 401 | 无参,固定 msg `please login you account` |
| `ErrForbidden()` | 403 | 无参,固定 msg `http forbidden` |
| `ErrNotFound()` | 404 | 无参,固定 msg `404 Not Found` |
| `NewValidationError(msg, fields)` | 400 | 带字段级错误 map |
| `NewSingleFieldError(field, msg)` | 400 | 单字段校验错误 |
| `NewMultipleFieldErrors(fields)` | 400 | 多字段校验错误 |

> 都是这几个,**没有** `ErrConflict` / `ErrTooManyRequests` 等;需要就用 `New(409, err)`。注意 `ErrBadRequest` / `ErrInternalServerError` 是**包级变量(函数值)**,调用是 `bizerr.ErrInternalServerError(err)`。

### 用法

```go
func Handler(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
    if bad { return nil, bizerr.ErrBadRequest(errors.New("invalid param")) }
    dto, err := svc.Do(...)
    if err != nil { return nil, bizerr.ErrInternalServerError(err) }
    return dto, nil
}
```

### 默认响应格式 & 校验错误的坑

默认 handler([feature/server.go:313](../../feature/server.go#L313)):

```json
{ "message": "<Message()>" }    // 状态码 = HTTPCode()
```

> ⚠️ **默认只输出 `message`,不输出 `ValidationErrors()` 字段明细**。要把字段级校验错误返回给前端,得用 `WithErrorHandler` 传自定义 `ErrorHandler` 读 `ValidationErrors()` 自己拼(见 [sample/customize_error_handler](../../sample/customize_error_handler))。

### 与 i18n 的关系

**bizerr 不感知语言**,只存原始英文串。翻译在业务层做:**先翻译成串、再塞进 bizerr**:

```go
msg := c.T("error.customer.not_found")     // i18n 翻译
return nil, bizerr.NewValidationError(msg, nil)
```

---

## 二、logger

源码:[logger/logger.go](../../logger/logger.go)

### 方法

```go
logger.Debug(format, args...) / logger.Debugf(...)   // 同义
logger.Info(format, args...)  / logger.Infof(...)
logger.Error(format, args...) / logger.Errorf(...)
```

- 级别 `Debug(0) < Info(1) < Error(2)`;
- `Error` **总是输出**;`Info`/`Debug` 按当前级别过滤;
- error→stderr,info/debug→stdout,均带文件行号。

### 级别怎么定(优先级)

[logger/logger.go:33](../../logger/logger.go#L33) `init()`:

1. **`LOG_LEVEL` 最高优先**:设了就用它(`debug`/`info`/`error`,不分大小写),忽略 RUN_LEVEL;
2. 否则看 **`RUN_LEVEL`**:
   - `local` → Debug
   - `eng` / `stage` → Info
   - `production` → Error
   - 未设 / 非法 → **默认 Error**(生产安全)。

也可运行时 `logger.SetLogLevel(...)` / `SetLogLevelFromString(...)` 手动改。
