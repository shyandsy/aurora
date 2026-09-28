# 搭建指南(🏗️ 脚手架 / 怎么搭一个服务)

这里讲的不是"某个 Feature 怎么用",而是**怎么用 aurora 搭起一个服务**——启动、分层、以现成结构为骨架。这些是**约定和模板**,照着搭 / fork,不是拿来 `inject` 的运行时能力。

> 先读 [架构与核心机制](../architecture.md)(App / Feature / DI / 生命周期)。完整可运行范例:[sample/full_showcase](../../sample/full_showcase/)。

## 一、起一个 App

```go
func main() {
    app := bootstrap.InitDefaultApp()   // 装默认 Feature:Server / GORM / Redis / JWT / i18n / migration 等
    registerProviders(app)              // 注册本服务的 datalayer / service / 额外 Feature(opt-in 的如 loginguard/doorman)
    app.RegisterRoutes(routes(app))
    app.Run()
}
```
- `InitDefaultApp` 给一套开箱即用的默认 Feature;opt-in 的(loginguard / doorman / ratelimit / tokenguard…)在 `registerProviders` 里按需 `AddFeature`。
- 依赖顺序:被别人 `inject` 的东西要先注册(如 redis 在 loginguard 之前;provider 在其 feature 之前)。

## 二、分层结构(约定)

一个服务按 `controller / service / datalayer / model` 分层(见 `sample/full_showcase`):

```
services/<svc>/
  cmd/            # main + registerProviders(装配)
  controller/     # HTTP handler:返回 (data, bizerr),薄;鉴权/限流等中间件挂这里
  service/        # 业务逻辑;inject datalayer 与各 Feature(如 loginguard.Guard)
  datalayer/      # 存储访问(gorm),接口 + 实现,可替身单测
  model/          # entity(持久化)/ dto(对外)
  migrations/     # goose 迁移(只在跑迁移的那个服务)
```
- controller 薄、service 厚;跨层靠接口 + DI 注入,便于单测替身。
- Handler 统一返回 `(data, bizerr.BizError)`,响应/错误由框架统一渲染。

## 三、以现有服务为骨架 fork

新服务/新项目**别从零手搓**:以一个结构最全的现有服务(如 `sample/full_showcase`,或你已有的某个服务)为骨架,复制其分层 + 装配 + CI/部署骨架,再裁剪替换业务。这样天然继承框架约定、避免重造基础设施。

## 四、带前端的 Feature 怎么落地

`doorman` 这类 🖥️ 带前端 Feature:后端 `AddFeature` 之外,把 `feature/doorman/web/` 组件**拷进你的前端工程**、配 `apiBase` 挂上即用(schema 驱动,后端加插件前端零改动)。详见该 feature 文档。
