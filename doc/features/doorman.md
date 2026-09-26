# doorman(门房 / 风险评估器)

对某个业务操作(`scope`,如注册 / 登录 / 找回密码 / 提现),按**可配规则**评估这次请求,输出**一个风险等级**(`none`/`low`/`medium`/`high`/`critical`)。doorman **不认识任何业务动作**——拦截 / 要求邮件激活 / 放行,都由业务侧据风险等级自己映射、自己执行。

包:`github.com/shyandsy/aurora/feature/doorman`
源码分层(见 [feature/doorman/README.md](../../feature/doorman/README.md)):`core/`(领域引擎)、`service/`(Console + DB 存储)、`model/{entity,dto}/`、`controller/`(管理 API)、`migrations/`、`web/components/doorman/`(前端组件)。

> 自动注册的 `contracts.Features`(走 `AddFeature`)。默认自建 DB 存储 + 注入 `Doorman`(业务入口)与 `Console`(配置页后端)。三张 `doorman_*` 表由**宿主 goose** 建(库不自 DDL)。

---

## 这篇重点:怎么复用

doorman 设计成**跨项目可复用**——一个新项目要接反爬风控,不重写,而是复用两块:**后端 feature** + **前端组件**。两块之间靠 **schema 驱动的管理 API** 解耦(后端把"有哪些条件/字段/场景/动作"通过 `/kinds`、`/scopes` 告诉前端),所以**后端加条件插件、加场景,前端零改动**。

### A. 后端 feature 复用(4 步)

```go
// 1) 装 feature,在单一注册点用 WithScope 声明业务场景 + 动作目录
app.AddFeature(doorman.NewFeature(
    doorman.WithScope("register", "注册",
        doorman.Action("allow",        "放行",        doorman.ActionTerminal),
        doorman.Action("email_verify", "要求邮件激活", doorman.ActionFriction), // 减速器 → 进完成漏斗
    ),
))
```

- **2) 建表**:把 `feature/doorman/migrations/doorman_schema.sql` 复制进你**跑 goose 的那个服务**的迁移目录(带该项目时间戳)。多服务共库时一处建即可。
- **3) 守端点**:填 `Attempt` → `d.Assess(ctx)` → 据 `Level` 映射动作 → 执行 → `d.Record(...)`;friction 动作完成后 `d.MarkOutcome(...)` 回填做「判定→完成」漏斗。
- **4) 挂管理 API**(套你后台鉴权):`app.RegisterRoutes(doormanctl.Routes("/api/admin/v1/doorman", myAuth...))`。

以**接口类型** `doorman.Doorman` 注入:

```go
type someService struct {
    Guard doorman.Doorman `inject:""`
}
```

场景名 / 动作名是**与代码的契约**(消费方硬编码 `Assess("register")` + 动作执行),`WithScope` 注册的必须和代码一致——**代码是权威**,只往 DB 加会得到永不生效的"幻影 scope"(保存时会被 Console 拒)。

### B. 前端组件复用(拷贝即用)

配置台是一个 **schema 驱动的 Angular 独立组件**,随包放在 [`feature/doorman/web/components/doorman/`](../../feature/doorman/web/)。复用三步(详见该目录 README):

1. 把 `components/doorman/` 整个拷进你的 Angular 工程。组件**自包含**:自带三语 i18n(启动深合并进 ngx-translate)、删除确认弹窗、时间格式化,不依赖你 app 的共享件。
2. 改 `shared/services/doorman-api.ts` 顶部两处(⚠️ 注释标着):`TOKEN_STORAGE_KEY`(读 token 的 key)、`API_TIMEOUT_MS`(超时)。
3. 挂组件、绑 `apiBase`(= 后端 `Routes` 的 prefix)+ `scopes`(场景 id,和后端 `WithScope` 一致)。

组件不写死任何条件类型/字段/动作——全从 `GET /kinds`、`/scopes` 动态拿。**后端加插件,前端不用动。**

> **权限边界**:管理 API + 前端组件只挂**后台**且套鉴权。customer / 对外端只调后端 `Assess`,**绝不暴露**这套配置 API 或组件。

---

## 中性原则

doorman 严格中性、可跨项目:引擎只读 `Attempt` 上的事实字段,不 import 任何业务包、不出现任何业务名 / scope / 域名字面量;条件插件是唯一扩展点(内置 `ua_match` / `asn_hosting` / `country_in` / `rate_limit`,`rate_limit` 有状态、靠业务注入 `Store` 计数)。业务差异全走注入:`Attempt`(事实)、`scope` / 动作名(字符串)、条件插件、`Subject`(漏斗关联键)。

契约与动作类型、漏斗口径等细节见 [feature/doorman/README.md](../../feature/doorman/README.md)。
