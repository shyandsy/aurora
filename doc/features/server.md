# Server Feature

HTTP 服务器 feature —— 封装 gin engine、路由注册、统一 handler 约定、CORS、健康检查、优雅停机。它是**唯一必须最先注册**的 feature(App 对它有特殊处理,见 [architecture.md §2.3](../architecture.md))。

源码:[feature/server.go](../../feature/server.go)、[config/server.go](../../config/server.go)、[contracts/server.go](../../contracts/server.go)、[contracts/route.go](../../contracts/route.go)、[contracts/request.go](../../contracts/request.go)

---

## 创建

```go
server := feature.NewServerFeature()                       // 默认
server := feature.NewServerFeature(                        // 带自定义错误处理器
    feature.WithErrorHandler(myErrorHandler),
)
app.AddFeature(server)
```

- `NewServerFeature(opts ...ServerOption)` — [feature/server.go:45](../../feature/server.go#L45)
- 目前唯一 option:`WithErrorHandler(contracts.ErrorHandler)` — [feature/server.go:39](../../feature/server.go#L39),替换默认错误响应格式。

`Setup` 里:注入 `ServerConfig` → `Validate` → 构建 gin engine → 把 `*gin.Engine` Provide 进容器([feature/server.go:60](../../feature/server.go#L60))。

---

## 定义路由

路由是 `contracts.Route` 的列表([contracts/route.go:10](../../contracts/route.go#L10)):

```go
type Route struct {
    Method      string                 // "GET" / "POST" / "PUT" / "DELETE" / "PATCH"
    Path        string
    Handler     CustomizedHandlerFunc
    Middlewares []gin.HandlerFunc      // 按序在业务 handler 之前执行
}
```

注册:

```go
app.RegisterRoutes([]contracts.Route{
    {Method: "GET", Path: "/api/customer", Handler: customer.GetCustomer,
     Middlewares: []gin.HandlerFunc{jwtMiddleware, entitlementMiddleware}},
})
```

> `RegisterRoutes` 只是**追加缓存**([feature/server.go:79](../../feature/server.go#L79)),真正挂到 gin 是在 `Start()` 时的 `setupRoutes()`。中间件数组会被拼在业务 handler **前面**(`append(r.Middlewares, handler)`),即中间件先跑。

---

## Handler 约定(重点)

Aurora 不用裸 `gin.HandlerFunc`,而是自定义签名([contracts/route.go:8](../../contracts/route.go#L8)):

```go
type CustomizedHandlerFunc func(*RequestContext) (interface{}, bizerr.BizError)
```

**你只管返回 `(数据, 错误)`,框架负责序列化和错误响应**:

```go
func GetCustomer(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
    id, _ := middleware.GetUserID(c)
    dto, err := svc.Get(c.Context, id)
    if err != nil {
        return nil, bizerr.ErrNotFound()   // → 框架按 BizError 的 HTTPCode/Message 响应
    }
    return dto, nil                        // → 框架 c.JSON(200, dto)
}
```

包装逻辑([feature/server.go:281](../../feature/server.go#L281) `createHandler`):
- `bizErr != nil` → 走错误处理器输出;
- 否则若 handler **没自己写过响应**(`!c.Writer.Written()`)→ `c.JSON(200, data)`;
- 若 handler 自己写了(`c.Data` / `c.String` / 流式)→ 框架不再序列化,避免多写一个 `null`。

### RequestContext

[contracts/request.go:9](../../contracts/request.go#L9) —— 内嵌 `*gin.Context`,额外带 `App` 和 `Translator`:

```go
c.Context          // 原始 *gin.Context(取 param/query/body、写响应)
c.App              // DI 容器,可 Find 依赖
c.GetLang()        // 请求语言:?lang= → Accept-Language 首段 → Translator 默认 → "en"
c.T("error.xxx")   // 按 GetLang() 翻译(见 i18n.md)
```

---

## 错误响应

默认错误格式([feature/server.go:313](../../feature/server.go#L313) `defaultHandleError`):

```json
{ "message": "<bizErr.Message()>" }   // HTTP 状态码 = bizErr.HTTPCode()
```

想改格式 → `WithErrorHandler` 传自定义 `contracts.ErrorHandler`。BizError 模型详见 [bizerr.md](./bizerr.md)。

---

## 健康检查

`setupRoutes` 会**先**注册两个端点([feature/server.go:237](../../feature/server.go#L237)),无需自己写:

| 端点 | 返回 |
|---|---|
| `GET /health` | `{status:"healthy", service, version, timestamp}` 200 |
| `GET /ready` | `{status:"ready", service}` 200 |

swarm / k8s 的存活探针直接打 `/health` 即可。

---

## 优雅停机

启动时起两个 goroutine([feature/server.go:83](../../feature/server.go#L83) `Start`):`startServer()`(`ListenAndServe`)+ `signalListener()`。

停机链路([feature/server.go:154](../../feature/server.go#L154)):

```
signal.Notify(SIGINT, SIGTERM)  →  收到信号
   → server.Shutdown(ctx)        // ctx 超时 = SHUTDOWN_TIMEOUT
   → 排空在途请求;超时则强制关闭并打印 "shutdown timeout ... forcing close"
   → Wait() 返回 → app.Run() 返回
```

- 超时来自 `SHUTDOWN_TIMEOUT`([config/server.go:33](../../config/server.go#L33),默认 5s)。
- ⚠️ **只关 HTTP server**,不会自动关其它 feature、也不排空后台任务(如 asynq)。后台任务的排空要在 `main` 里自己编排 —— 见 [architecture.md §2.5](../architecture.md)。
- ⚠️ 超过 `SHUTDOWN_TIMEOUT` 的长请求(大文件上传等)会被硬切;部署时需把容器 `stop_grace_period` 设得比它大。

---

## 环境变量

> ⚠️ **重要坑**:`ServerConfig`([config/server.go:27](../../config/server.go#L27))每个字段都写了 `envDefault:"..."`,**但 `ResolveConfig` 根本不读 `envDefault`**(见 [config.md](./config.md)),而且这些字段**没有 `omitempty`** —— 所以它们全是**必填**。下表"envDefault 标注值"那列**运行时不会生效**,不设对应环境变量 = 启动直接报 required 错误。部署时这些 env 必须逐个显式设置。

| Env | 类型 | 必填 | envDefault 标注(不生效) | 说明 |
|---|---|---|---|---|
| `HOST` | string | ✅ | `0.0.0.0` | 监听地址,须是合法 IP |
| `PORT` | int | ✅ | `8080` | 1–65535 |
| `READ_TIMEOUT` | duration | ✅ | `30s` | 读超时 |
| `WRITE_TIMEOUT` | duration | ✅ | `30s` | 写超时 |
| `IDLE_TIMEOUT` | duration | ✅ | `60s` | keep-alive 空闲超时 |
| `SHUTDOWN_TIMEOUT` | duration | ✅ | `5s` | 优雅停机排空超时 |
| `SERVICE_NAME` | string | ✅ | `myapp` | 服务名(banner / 健康检查) |
| `SERVICE_VERSION` | string | ✅ | `1.0.0` | 版本 |
| `RUN_LEVEL` | string | ✅ | `local` | `local`/`eng`/`stage`/`production`;=production 时 gin 走 release 模式 |
| `TRUSTED_PROXIES` | `[]string` | ❌(omitempty) | —(默认写在代码里) | 可信代理 IP/CIDR。见下方[「可信代理」](#可信代理--真实客户端-ip)。**这一项是正确写法**:`,omitempty` + 默认值在代码(`DefaultTrustedProxies`),不踩 `envDefault` 的坑 |

> 📌 `TRUSTED_PROXIES` 与上表其余字段不同 —— 它带 `,omitempty`(可选)、默认值写在代码里,是**加配置的正确姿势**;其余字段那套"`envDefault` + 无 omitempty"是历史遗留坑(见 [config.md](./config.md)),别照抄。

CORS 走独立的 `CORSConfig`([config/cors.go](../../config/cors.go)):`CORS_ALLOWED_ORIGINS` / `_METHODS` / `_HEADERS` / `_CREDENTIALS`(前三个是逗号分隔列表)。**三个列表全空 = 不启用 CORS**;任一非空则三者都要填。

---

## 可信代理 · 真实客户端 IP

`c.ClientIP()`(gin)拿到的是不是**真实**客户端 IP,取决于「信任哪些代理」。gin 的默认是**信任所有代理**——即无条件采信请求头里的 `X-Forwarded-For` / `X-Real-IP`。这在反代后能拿到真 IP,但**任何客户端都能伪造 `X-Forwarded-For` 冒充任意 IP**。凡是拿 `c.ClientIP()` 做审计、限流、IP 白名单/黑名单的地方,这就是个洞。

Aurora 用 `SetTrustedProxies` 收敛:**只有请求的直接对端(TCP peer)落在可信网段内时,才解析 XFF;否则 `c.ClientIP()` 用直连对端 IP。** 由 `TRUSTED_PROXIES`(`config.ServerConfig.TrustedProxies`)控制:

| `TRUSTED_PROXIES` 取值 | 行为 |
|---|---|
| **不设**(默认) | 信任 `DefaultTrustedProxies` = loopback + RFC1918 私网([config/server.go](../../config/server.go));反代在私网里→拿真 IP,直连公网→无法伪造 |
| `none` | 谁都不信,`c.ClientIP()` **恒为直连对端** IP |
| `10.0.0.0/8,1.2.3.4` | 精确信任这些 IP/CIDR(如只信你的反代地址),逗号分隔 |
| `0.0.0.0/0,::/0` | 信任所有代理,**恢复 gin 历史默认**(可被伪造,不推荐) |

- 非法 IP/CIDR 在 `ServerConfig.Validate()` 直接报错(fail-fast)。
- ⚠️ **行为变更**:相对 gin 原生默认("信任所有代理"),Aurora 默认改为"只信私网/loopback"。这是安全硬化。若你的应用**直连公网无反代**、或反代不在私网,请显式设 `TRUSTED_PROXIES`(直连无反代场景建议 `none`;反代不在私网则填反代的实际 IP/CIDR)。
- 部署形态:典型是"服务只经私网里的 traefik/nginx 反代可达",默认值即开箱可用,无需配置。
