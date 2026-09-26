# doorman 配置页(前端参考实现,拷贝即用)

doorman 后端 `controller` 暴露的管理 API 是 **schema 驱动** 的(`GET /kinds`、`/scopes` 把条件类别/字段/风险等级/动作目录全告诉前端),
所以这套配置页**不写死任何条件类型或字段**——后端加条件插件、加 scope,前端零改动。

这是一份 **Angular 独立组件** 参考实现:一个 `DoormanConfigComponent`,挂进你后台任意位置,绑定 `apiBase` + `scope` 即可用。
它不是 npm 包——**把 `web/` 整个目录拷进你的 Angular 工程**(比如 `src/app/features/doorman/`),按下面接线。

## 里面有什么

```
web/
  doorman-config.component.ts/html/css   ← 主组件(schema 驱动:规则 CRUD / 风险→动作策略 / 统计漏斗 / 决策明细)
  index.ts                               ← 出口 barrel(export DoormanConfigComponent)
  i18n/{index.ts,en,zh-CN,zh-TW}.json    ← 模块自带三语文案(启动时深合并进 ngx-translate,不覆盖宿主字典)
  shared/
    models/doorman.dto.ts                ← 与后端管理 API 逐字段对齐的数据契约(直接可用)
    services/doorman-api.ts              ← 瘦 HTTP 访问层(见下「要改的两处」)
    components/confirm-dialog/…          ← 删除确认弹窗(小组件,随包带,免得依赖你 app 的)
    utils/date.util.ts                   ← 时间格式化(默认 UTC+8,改时区改这里)
```

## 依赖(你的 Angular app 要有)

- Angular(standalone components;本组件用了 `input()`/`signal()`/`effect()`,需 **Angular 17+**)
- `@ngx-translate/core`(文案走 ngx-translate;模块 i18n 会自动深合并进去)
- `provideHttpClient`(组件级 `providers` 里 provide 了 `DoormanApi`,它 `inject(HttpClient)`)

## 接线

1. **拷目录**:把 `web/` 拷成你工程里的一个 feature 目录,如 `src/app/features/doorman/`。
2. **改 `shared/services/doorman-api.ts` 两处**(文件顶部有 ⚠️ 注释标着):
   - `TOKEN_STORAGE_KEY`:从 `localStorage` 读 access token 的 key,填你 app 的。
     **若你的 HttpClient 已有 auth 拦截器统一加 `Authorization`**,直接把 `getHeaders` 里读 token 那段删掉。
   - `API_TIMEOUT_MS`:HTTP 超时(毫秒),按你 app 的统一值改。
3. **挂组件**,绑定输入:

   ```html
   <app-doorman-config
     [apiBase]="'/api/admin/v1/doorman'"
     [scopes]="['register','login','reset_password']">
   </app-doorman-config>
   ```

   ```ts
   import { DoormanConfigComponent } from './features/doorman';
   // 在你的页面组件 imports 里加 DoormanConfigComponent(standalone)
   ```

   输入项:
   - `apiBase`(必填):后端 doorman 管理 API 基路径,对应你把 `doormanctl.Routes(prefix, …)` 挂的 `prefix`。
   - `scopes`(顶部 tab 列表):要配的场景 id,如 `register`/`login`/`reset_password`;标签走 i18n key `doorman.scope.<id>`,没配就显示 id 原文。
   - `scope`(单场景,可选):只配一个场景时用它替代 `scopes`。

   > 场景 id 必须和后端 `WithScope(id, …)` 注册的一致(后端是权威);配了后端没注册的 scope,保存会被拒。

## 权限边界(重要)

这套是**管理 API**,只该挂在**后台**、且套你后台的鉴权(后端 `doormanctl.Routes(prefix, 你的鉴权中间件...)`)。
**customer/对外端只调后端 `Assess`,绝不要暴露这套配置 API 或页面。**

## i18n

组件初始化时把 `i18n/` 里的三语文案**深合并**进 ngx-translate 现有字典(不整体替换,不覆盖你宿主的 key),
并在语言切换后再合并一次。你只需保证 app 里有 ngx-translate。要加语言,往 `i18n/` 加一份 json 并在 `i18n/index.ts` 登记。

## 后端契约

字段含义、各接口(`/kinds`、`/scopes`、`/rules`、`/policy`、`/stats`、`/decisions`)见上一层 [../README.md](../README.md) 与 `model/dto`。
`shared/models/doorman.dto.ts` 就是照它逐字段写的,后端 DTO 变了这里要同步。
