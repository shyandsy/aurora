# doorman 前端组件(可复用,拷贝即用)

这个 `web/` 目录放的是 doorman 的**可复用前端组件**——不是某个项目的页面,是**任何用 doorman 后端的项目都能拷走直接用**的东西。
约定:一个组件一个目录,收在 `components/<组件名>/` 下。目前有:

- [`components/doorman/`](components/doorman/) —— **doorman 配置台**(`DoormanConfigComponent`):规则 CRUD / 风险→动作策略 / 统计漏斗 / 决策明细,一个 Angular 独立组件。

## 为什么能复用 / 复用的关键

doorman 后端 `controller` 暴露的管理 API 是 **schema 驱动**的:`GET /kinds` 告诉前端有哪些条件类别、每个类别有哪些字段;
`GET /scopes` 告诉前端有哪些场景、每个场景有哪些动作。所以这个组件**不写死任何条件类型、字段、动作**——
**后端加条件插件、加 scope,前端零改动**,靠 schema 动态渲染表单和下拉。这就是它能跨项目复用的根本。

复用它,你只做三件事(详见组件目录内注释):

1. **拷目录**:把 `components/doorman/` 整个拷进你的 Angular 工程(如 `src/app/features/doorman/`)。组件自包含——
   自带 i18n(三语,启动时深合并进 ngx-translate,不覆盖你的字典)、删除确认弹窗、时间格式化,不依赖你 app 的任何共享件。
2. **改 `shared/services/doorman-api.ts` 顶部两处**(有 ⚠️ 注释):
   - `TOKEN_STORAGE_KEY`:从 `localStorage` 读 access token 的 key(填你 app 的);若你的 HttpClient 已有 auth 拦截器统一加 `Authorization`,把 `getHeaders` 里读 token 那段删掉即可。
   - `API_TIMEOUT_MS`:HTTP 超时(毫秒)。
3. **挂组件**,绑 `apiBase` + `scopes`:

   ```ts
   import { DoormanConfigComponent } from './features/doorman'; // components/doorman/index.ts
   // 页面组件 imports 里加 DoormanConfigComponent(standalone)
   ```

   ```html
   <app-doorman-config
     [apiBase]="'/api/admin/v1/doorman'"
     [scopes]="['register','login','reset_password']">
   </app-doorman-config>
   ```

   输入项:
   - `apiBase`(必填):后端 doorman 管理 API 基路径 = 你把 `doormanctl.Routes(prefix, …)` 挂的 `prefix`。
   - `scopes`:顶部 tab 的场景 id(如 `register`/`login`/`reset_password`),标签走 i18n key `doorman.scope.<id>`;必须和后端 `WithScope(id,…)` 注册的一致(后端权威,配了没注册的 scope 保存会被拒)。
   - `scope`:只配一个场景时用它替代 `scopes`。

## 依赖(你的 Angular app 要有)

- Angular 17+(用了 `input()`/`signal()`/`effect()` 的 standalone 组件)
- `@ngx-translate/core`(文案走它;组件的 i18n 自动深合并进去)
- `provideHttpClient`(组件 `providers` 里 provide 了 `DoormanApi`,内部 `inject(HttpClient)`)

## 权限边界(重要)

这是**管理 API**,只该挂在**后台**、且套你后台的鉴权(后端 `doormanctl.Routes(prefix, 你的鉴权中间件...)`)。
**customer / 对外端只调后端 `Assess`,绝不要暴露这套配置 API 或本组件。**

## 后端契约

字段含义、各接口(`/kinds`、`/scopes`、`/rules`、`/policy`、`/stats`、`/decisions`)见 [../README.md](../README.md) 与 `model/dto`;
组件里 `shared/models/doorman.dto.ts` 就是照后端 DTO 逐字段写的,后端 DTO 变了这里要同步。
