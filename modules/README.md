# modules/ —— 可挂载服务模块

`modules/` 放的是**一整套后端服务模块**,和框架库(`feature/`、`middleware`、`encryption`、`types` 等)刻意分开。

## 和 `feature/` 的区别

| | `feature/*` | `modules/*` |
|---|---|---|
| 是什么 | 一个**能力库**(redis / jwt / loginguard …) | 一个**完整服务**(controller + service + datalayer + migrations + 路由) |
| 大小 | 一个包 | 一棵子树 |
| 怎么用 | `app.AddFeature(x.NewFeature())` + `inject:""` | `app.AddFeature(x.NewFeature(cfg))` + `app.RegisterRoutes(x.Routes(app))` |
| 目的 | 给服务提供某项能力 | **让多个项目共享同一个服务、不再各写一遍** |

## 为什么要有这一层

有些「后端服务」是**多个项目都要、且本质相同**的(典型:后台账号中心)。与其每个项目 fork 一份改到跑偏,不如做成一份共享代码放这里,各项目 `sync-aurora` 拉到后**换 `Config` 挂载**即可,永不重写。差异全走 `Config`(DB/密钥/品牌/namespace),业务代码零行。

## 通用接入姿势

```go
app := bootstrap.InitDefaultApp()
app.AddFeature(x.NewFeature(x.Config{ /* 本项目的差异配置 */ }))
app.RegisterRoutes(x.Routes(app))
app.Run()
```

## `web/` 约定(带前端的模块/特性)

aurora 里**任何带 UI 的 module 或 feature,前端源码一律放它正下方的 `web/` 子目录**(`modules/user/web/`、`feature/doorman/web/`)——后端 + 前端共置、锁步 sync,扫一眼就知道它是全栈的,不会"前端在别的仓 → 被忘"。aurora 只**存**前端源码、自己不构建;消费方从该源构建成**一份版本化的共享 remote**、各项目后台壳 pin 版本加载,**别各项目各拷各建**(必 drift = "重写"的变种,详见 [设计稿 §4.3](../doc/proposals/shared-user-center.md))。

## 现有模块

| 模块 | 是什么 | 文档 |
|---|---|---|
| [`user/`](user/README.md) | 后台账号中心:账号 / RBAC / 登录 / 2FA / 会话 / 微服务 token / 登录防护 / 被锁管理 / gate | [modules/user/README.md](user/README.md)、[设计稿](../doc/proposals/shared-user-center.md) |
