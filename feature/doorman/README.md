# doorman —— 通用「门房 / 风险评估器」feature

对某个业务操作(`scope`,如注册 / 登录 / 提现 / 某项敏感操作),按**可配规则**评估这次请求,输出**一个风险等级**(`none`/`low`/`medium`/`high`/`critical`)。doorman **不认识任何业务动作**——拦截 / 要求邮件激活 / 放行,都由**业务侧**据风险等级自己映射、自己执行。

- **严格中性、可跨项目复用**:引擎只读 `Attempt` 上的事实字段,不碰采集、不认业务名/scope、**不含任何动作**。业务负责填 `Attempt`、传 `scope`、注册 scope/动作、(可选)加业务专属条件。
- **电池全含**:自带存储 + 规则/策略管理 API + schema 驱动的配置页(前端组件另发)+ 自带 goose 迁移 + 决策流水自动保留清理。
- **可扩展**:加一类条件 = 实现接口 + 注册一行;引擎、存储表、DTO、配置页**全不动**。

---

## 一、核心概念

- **Attempt**:一次请求的事实快照(UA/IP/国家/运营商/ASN/是否机房 + `Ext` 业务扩展袋)。业务填,doorman 只读、不采集。
- **Condition**(条件插件,唯一扩展点):`Type / Compile(自解析+校验参数) / Fields(配置 schema)`。内置 `ua_match` / `asn_hosting` / `country_in` / `rate_limit`。
  - `rate_limit` 是**有状态**条件(某维度在时间窗内次数超阈值即命中),靠业务注入的 `Context.Store`(如 Redis)计数;没注入时它恒不命中(best-effort,不误伤)。配置:`by`(ip/asn/subject)+ `window`(如 `1h`)+ `max`(窗内允许次数)。
- **Rule**:`{scope, 多个条件, combine(and/or), riskLevel, enabled}`;命中多条取**最高**等级。
- **Doorman**:门面,`Assess(*Context) Assessment{Level, Matched}`。业务据 `Level` 自己决定处置。
- **动作**:doorman 只存动作名字符串 + 类型(`friction` 减速器 / `terminal` 硬卡),不解释、不执行。

---

## 二、接入(五步)

1. **注册 feature**,在**单一注册点**用 `WithScope` 声明业务的 scope + 动作目录:

   ```go
   app.AddFeature(doorman.NewFeature(
       doorman.WithScope("register", "注册",
           doorman.Action("allow",        "放行",        doorman.ActionTerminal),
           doorman.Action("block",        "直接拦截",    doorman.ActionTerminal),
           doorman.Action("email_verify", "要求邮件激活", doorman.ActionFriction), // 减速器 → 进完成漏斗
       ),
       // 业务专属条件(可选):doorman.WithCondition(yourCondition),
   ))
   ```

   > 需宿主已装 GormFeature(有 `*gorm.DB`)。想换存储:`doorman.WithRuleSource(...)` override 默认 DB store。
   > scope/动作**名字是与代码的契约**(消费方硬编码 `Assess(scope)` + 动作执行),必须一致——名字是代码权威,别只往 DB 加(防"幻影 scope")。

2. **建表**:doorman 是库不是服务、不自己 DDL。把 `migrations/doorman_schema.sql` **复制进你跑 goose 的那个服务**的迁移目录(带该项目的时间戳命名)。多服务共库时,一个服务建表即可,其余只用表。

3. **在要守的端点**填 `Attempt` → `Assess` → 据 `Level` 映射动作 → 执行:

   ```go
   gctx := &doorman.Context{Ctx: ctx, Scope: "register", Attempt: buildAttempt(req)}
   a := d.Assess(gctx)
   action := myMapRiskToAction(d, gctx.Scope, a.Level) // 业务侧:ActionFor 覆盖 + 内置默认
   d.Record(gctx, a, action)                            // 记流水(统计/明细)
   // friction 动作(如 email_verify)完成后,回填结果做「判定→完成」漏斗:
   //   进入挑战时 gctx.Subject = 关联键;完成时 d.MarkOutcome(scope, subject, "done")
   ```

4. **挂管理 API**(给配置页用),套自己的鉴权中间件:

   ```go
   import doormanctl "github.com/shyandsy/aurora/feature/doorman/controller"
   app.RegisterRoutes(doormanctl.Routes("/api/admin/v1/doorman", myAuthChain...))
   ```

5. **挂配置页前端**(schema 驱动的 Angular 组件,参考实现在 [`web/`](web/README.md),拷进你后台工程即用):
   宿主传后端 `apiBase` + `scopes`,组件从 `GET /kinds`、`/scopes`、`/rules`、`/policy` 自发现,渲染 tab / 条件表单 / 「风险→动作」下拉 / 统计漏斗 / 决策明细。接线与「要改的两处」见 web/README.md。

---

## 三、动作类型与漏斗

- **friction(减速器)**:施加一道人工阻力(邮件激活 / 2FA / OTP…),有后续(完成 or 放弃)→ 进**完成漏斗**(判要减速 → 已完成 / 未完成)。业务在进入挑战时设 `Context.Subject`(不透明关联键),完成时 `MarkOutcome` 回填(回填时清 PII,只留最小计数行)。
- **terminal(硬卡)**:当场终结(拦截 / 放行),无后续,只记一笔。

漏斗口径通用(`challenged` / `resolved`),doorman 不认识"激活"这类业务语义,由动作 label 承载。

---

## 四、扩展:加一个条件类型

实现 `Condition` 接口(`Type` / `Compile` / `Fields`)+ `WithCondition` 注入即可;引擎、存储表、DTO、配置页全不动。内置中性条件见 `core/builtin.go`。

---

## 五、边界(通用 vs 业务)

| 缝 | doorman(通用) | 业务侧 |
|---|---|---|
| scope | 只当字符串 key | 定义 scope 集、在哪个接口拦、注册(`WithScope`) |
| Attempt | 只读固定字段 + `Ext` | 采集(geoip/IP/…) |
| 动作 | 只存名 + 类型,不执行 | 动作目录、风险→动作映射、真正执行 |
| Condition / Subject | 内置中性条件、存 Subject | 业务专属条件、Subject 语义 |

**doorman 严格中性**:不 import 任何业务包、不出现任何业务名/scope/域名字面量。

---

## 六、自持存储与保留

- 表:`doorman_rule`(规则)/ `doorman_action_policy`(风险→动作策略)/ `doorman_decision`(决策流水);由宿主 goose 建(见第二步),feature 不 AutoMigrate。
- `doorman_decision` 是只增流水,feature 自起后台 goroutine 定期清老数据(默认留 90 天,`WithDecisionRetention` 可改/关),`Close` 时停。
