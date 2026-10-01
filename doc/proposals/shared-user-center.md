# 共享后台 User 中心 —— 收口到 aurora,跨项目单一源

> 状态:方向已定、结构已落地(见 aurora `modules/user/` + `middleware/encryption/types`)。homeserver 侧接入待做。
> 面向:aurora 维护者 + 要接入的各项目(homeserver / polaris / deploy / talkwithtong)工程师。

## 0. 决策(已拍板)

| # | 决策 | 定论 |
| --- | --- | --- |
| 落点 | 进 aurora **vs** 独立仓 | **进 aurora**(user 模块 + 通用件都进 aurora)。理由:4 个项目本来就 sync aurora,跟着一起同步、**零新基建**(不建新仓、不写新 sync);代价是 aurora 从纯框架多带一个"服务模块",可接受。 |
| 命名 | usercenter **vs** user | **user**(不叫 usercenter):`package user`,对外 `user.NewFeature` / `user.Routes` / `user.Config`。 |
| 部署模型 | 各起一份实例、数据隔离 **vs** 共用一份 | **各起一份、数据隔离**(代码共享、实例/库各自独立)。 |
| 前端 remote | 每个 host pin 版本 **vs** 全跟最新 | **每个 host pin 版本**(防一次坏部署打挂 N 个后台登录)。 |
| gate | 随 user 模块 **vs** 独立 feature | **随 user 模块**(gate 就在 `user/controller/auth/gate.go`)。 |
| 模型扩展点 | 留 hook **vs** 纯配置 | **纯配置、不留 hook**(固定模型实测扛住多拷贝;真要加字段再说)。 |
| Schema 治理 | —— | **唯一真风险,见 §6 政策。** |

## 1. 目标

homeserver / polaris / deploy / talkwithtong **4 个项目**都要「后台账号管理 + gate」。现状各一份,改一个跟三个、迟早跑偏。本质是同一个东西(账号 / RBAC / 登录 / 2FA / 会话 / 微服务 token / 登录防护 / 被锁管理 + gate),差异只在配置。

**目标**:做成 aurora 里的**一份共享代码**,各项目 `import + 配置 + 挂载`,**永不重写**;改一次,经现成 `sync-aurora` 同步到全项目。

**非目标**:customer 客户体系(注册/找回密码/邮箱验证/会员 tier)不在此——那是各项目面向终端客户的业务。

## 2. 结构:aurora 里怎么组织

```
aurora/
├ modules/                   可挂载的「服务模块」区(和框架库分开,不塞在根)
│  └ user/                   用户中心模块(package user)
│     ├ config.go            Config:项目差异配置(见 §4)
│     ├ feature.go           NewFeature(Config) contracts.Features:Setup 装配全部依赖 + 启动校验
│     ├ routes.go            Routes(app) []contracts.Route:全部路由
│     ├ controller/          auth(含 gate)/ user / role / role_feature / feature / microservice / ratelimit
│     ├ service/             user / role / role_feature / feature / microservice_token(*2) / rolefeaturepublisher / ratelimit
│     ├ datalayer/           user / user_session / role / role_feature / feature / microservice_token(*2)
│     ├ model/               dto(API 形状)/ entity(DB 行)—— user 域业务模型
│     ├ migrations/          goose(user_rbac_core / ui_feature_grants / session / microservice_token / adopt_admin_data)
│     └ locales/             三语文案
│
├ middleware/                共享:JWT 鉴权中间件(JWTAuthMiddleware / GetUserID / AllowEnroll)+ rolefeature/ 子包
├ encryption/               共享:AES-256-GCM 凭据加解密(aes_gcm.go,密钥调用方注入,纯算法库)
└ types/                     共享:业务无关通用类型(JSONObject/JSONArray gorm JSON 列 + 分页 DTO + 状态枚举)
```

**为什么分两层**:
- `modules/user/` 是**一个服务**的完整实现(controller→service→datalayer→migrations),对外只暴露 `NewFeature / Routes / Config` 三个入口,内部子包不对外。放 `modules/` 下,和框架库(`feature/`、`middleware`、`encryption`、`types`)区分——将来别的可复用服务也进 `modules/`。
- `middleware / encryption / types` 是**所有服务都用**的共享框架库,放 aurora 顶层,admin/customer/deploy 也能直接用。

> 待统一:`types` 里 `status.go`(int Enable/Disable)与 `enabled_disabled_status.go`(string ENABLED/DISABLED)是同概念不同类型的历史冗余;统一 = 改 DB 列/DTO 类型的数据变更,列为独立 cleanup,不塞进本次零行为搬移。

## 3. 功能:这套提供什么

- **账号 + RBAC**:用户增删改查、角色、权限(feature)、角色→权限展开(rolefeaturepublisher 写读模型)。
- **登录**:密码登录 + 2FA(TOTP) + 会话/设备清单(列出/撤销/滚动续期)。
- **登录防护**:建在 aurora `loginguard`(IP/账号失败计数→锁)+ `tokenguard`(会话有效性/撤销/IP 绑定)上;后台「被锁列表 + 解锁」。
- **微服务 token**:对外签发「服务间调用」长效 token + 启停。
- **gate**:登录壳 + Traefik forwardAuth + TOTP(`admin_gate` cookie 镜像 SPA token),后台 SPA 下载门禁。
- **前端**:web/user(用户中心 SPA + 登录壳),做成共享 Native Federation remote(见 §4)。

## 4. 用法:项目怎么接入

### 4.1 后端(换 Config + 挂载,代码不写)

```go
app := bootstrap.InitDefaultApp()                       // aurora 起 redis/jwt/gorm/migrations
app.AddFeature(user.NewFeature(user.Config{
    // 字段以 config.go 为准,当前就三个(都可选,留空回落默认):
    TOTPKeyEnv:          "MYPROJ_TOTP_KEY",            // 凭据加密密钥 env 名;留空→USER_GOOGLE_TOTP_AUTH_KEY
    GateCookie:          "myproj_gate",                 // 下载门禁 forwardAuth cookie 名;留空→admin_gate
    LoginPolicyProvider: loginpolicy.NewDBProvider(..), // 可选:登录限流阈值运行时可调;留空→内置 StaticPolicy 硬锁
}))
app.RegisterRoutes(user.Routes(app))
app.Run()
```
- **差异全在这个 Config + chart/values 的 env/secret**,业务代码零行。
- 限流 **namespace 不在 Config**(自动取 `SERVICE_NAME`);**表前缀不在 Config**(走部署期 env `GOOSE_TABLE_PREFIX`)——二者都不是字段。
- 迁移随模块按各项目表前缀跑;各项目各自的库、数据隔离。

### 4.2 拿到这份代码 = `make sync-aurora`

user 模块在 aurora 里,所以**不需要新 sync 机制**——各项目现有的 `make sync-aurora REF=<含 user 的版本>` 拉进来就有了(和拿 loginguard/tokenguard 完全一样)。

### 4.3 前端(源码进 aurora,消费一份共享 remote)

> **✅ 已落地(aurora #71)**:前端源码已进 `modules/user/web/`(完整 SPA)。**待做**:从该源构建版本化共享 remote 的 CI + 各项目壳 pin 加载 + 各项目现有 `web/user` 切成消费这份。

**源码放 `modules/user/web/`**(和 doorman 的 `feature/doorman/web/` 一致——aurora 存前端源码、自己不构建;见 §2 的 `web/` 约定)。这样 user 模块的**后端 + 前端共置、锁步 sync、扫一眼就知道是全栈的**(避免"前端在别的仓 → 被忘"的坑)。

**消费 = 一份共享 remote,别各项目各拷。** 前车之鉴:doorman 现在是"各项目把 `feature/doorman/web` 组件拷进自己的 remote app"(homeserver `web/doorman`),实测已和 aurora 源**逐字节漂移**——各拷各建必 drift,就是"重写"的变种。web/user 一整个 SPA 更要避开:从 `modules/user/web/` 源**构建出一份版本化的共享 remote**(`/user/vN/remoteEntry.json`),4 个项目后台壳 `federation.manifest.json` **pin 到某版本**加载;升级 = 该壳显式 bump manifest(不重构建 host、不自动传播,坏了不连累别人)。i18n/路由经贡献套件合并。

#### 4.3.1 gate 登录壳怎么部署(之前漏写,homeserver / deploy 各自 ad-hoc,在此收口)

gate(`public/gate/index.html`)是**后台 SPA 下载门禁的静态登录壳**,**不是** `./Routes` 那棵 Angular 路由:自包含页、POST 绝对 `/api/<user>/v1/auth/login`、写 `admin_gate` cookie、不依赖 base-href。它随 `ng build` 落到**本 remote 产物的 dist 根 `/gate/`**。运行时:host 的 SPA catch-all 挂 forwardAuth(verify = `GET /api/<user>/v1/auth/gate/verify`,读 `admin_gate`),未过 → 302 到 `/gate/`(`gate.go: gateShellPath`);故 `/gate/` **必须有人提供、且公开不挂门禁**。

- **✅ 推荐:`/gate` 路由到 user remote 的镜像**(它 dist 根就有 `/gate/`)——Traefik 一条 `PathPrefix(/gate)` 指向 remote 的 nginx,公开、不挂 forwardAuth、不 stripprefix、priority 高于 host catch-all;host 完全不碰 gate。gate **只有 aurora 一份源、运行时只 remote 一份拷贝**,升级随 remote 走。
- **⚠️ 反模式:注入 host 镜像**(host 构建时 `cp aurora gate → public/gate`)——运行时 host 和 remote 各一份拷贝、host Dockerfile 硬编码 aurora 路径、升级要重建 host。仅当 host 不单独部署 remote 镜像时退而求其次。homeserver admin 现用此法;**deploy 已改用推荐做法**。

#### 4.3.2 登录态约定必须全栈对齐(接入硬约束,否则死循环)

remote 与 gate **硬编码**一套 key/cookie,host 壳必须全部对齐:access/refresh/user/enroll 的 localStorage key = `admin_access_token` / `admin_refresh_token` / `admin_user` / `admin_2fa_enrollment_pending`,门禁镜像 cookie = `admin_gate`。原因:remote 被懒加载进 host、**同源共享 localStorage**,remote 的 guard 硬读 `admin_access_token`;gate/forwardAuth 认 `admin_gate`。host 原本用别的 key(deploy 曾用 `deploy_*` / `access_token`)**必须一并改**(storage 层 + forwardAuth verify 的 cookie 名 + 任何读该 cookie 的旁路如 SSR 控制台)。key 名用 `admin_*` 不碍各 host 独立——不同 host 本就独立 origin、localStorage 天然隔离。deploy 真实踩坑:只换 gate 没对齐 storage → remote 读不到 token、gate 与 forwardAuth cookie 对不上 → 疯狂 `/auth/refresh` 死循环;全栈对齐后解决。

### 4.4 版本化 / 升级

`make sync-aurora REF=<新版本>` 一个 PR,先上 eng 跑迁移验证再进 prd。各项目自控节奏,不会"一改全崩"。

## 5. 中立化(硬性)

- 进 aurora 的代码里**不含 homeserver/customer 业务名、域名、tier 等**;项目差异只经 Config。
- **拆薄 `common/middleware`**:只搬干净的 JWT 鉴权(`jwt_middleware.go`);拖 customer `tier` 的限流中间件(`rate_limit.go → model/setting → tier`)**留在 homeserver、不进 aurora**。
- `common/model/setting` 是一袋**业务配置**(alipay/commission/tier/…),**不进 aurora**;通用的「设置存取框架 + 控制台」值得单独做成 aurora `feature/setting` + `web/setting`(doorman 那套:中立框架 + 前端 + 业务配置留消费侧),但那是**独立 initiative**,不在本模块内。

## 6. Schema 跨项目治理(唯一真风险,必须先立的政策)

代码越一致,风险越集中在「一处 schema 改动要同时对 N 个项目安全」。不定政策不动手。

1. **单一 owner + 编号归 aurora**:模块的表结构只由 aurora `modules/user/migrations` 改;**接入项目一律不得往这个 `user_` 迁移流里加迁移/改号**——那套版本号空间归 aurora,项目自己塞一条(哪怕是数据回填)迟早和下次 sync 带来的新号撞车或触发 goose 乱序/缺失校验。版本号单调、唯一,由 aurora 维护。
2. **只加不改 expand-contract**:加列 nullable/带默认;改/删走两阶段(expand 加新回填 → 跨版本后 contract 删旧),**绝不同 release 删/改在用列**;每个迁移有 down;destructive 默认禁。
3. **跨版本跳跃防护**:expand-contract 只保相邻版本;各项目独立 pin 可能一次跳多版(v3→v6),滚动重启期旧 pod 会对上已 contract 的 schema 而崩。政策:**支持窗口**(contract 只删「支持窗口内所有版本都不用」的列)+ **最低版本地板**(低于窗口须先部署 expand 代码再跨 contract)+ CI 拦「drop 的列仍被引用」。
4. **各项目 pin 自控节奏**:升级 = sync ref 的 PR,先 eng 跑迁移。
5. **两层隔离,让模块 schema 成为孤岛**:
   - 业务表名写死 `user_*`(在迁移 SQL 里,非靠前缀拼),与接入项目已有的 `users`/`roles` 天然不撞名;
   - goose **版本表**靠 `GOOSE_TABLE_PREFIX`(homeserver=`user_` → `user_goose_db_version`)与 host 自己的 `goose_db_version` **分开记账**,模块迁移流独立跟踪、永不和 host 的 app 迁移交叉编号。
6. **CI 跑 up/down/up 验可回滚**;迁移不得假设项目特有数据;破坏性变更走 RFC。

## 6b. 存量系统迁移方案(接入时怎么和老 user/admin 对接)

已有 user/admin 服务的系统接入本模块,分三种情况。核心两条铁律:**(A)** 模块只跑它那 4 条**中立 schema 迁移**(建 `user_*` 空表),**(B)** 「把老数据搬进 `user_` 表」是**每项目私有的一次性活,归 host 自己的 goose 流(`goose_db_version`),绝不进模块的 `user_` 流**。

| 情况 | 做法 |
|---|---|
| **① 源头项目(homeserver)** | 这 4 条版本号早已记在 `user_goose_db_version`(本地 user 服务先跑过)。接入 = 把 goose 迁移源**指到** `third_party/aurora/modules/user/migrations` → goose 按版本号判定(默认无 checksum)→ 全部已应用 → **no-op,纯换代码**。前提:确认 host 的迁移发现路径已重指过去(见 §8 embed.FS 待办)。 |
| **② 全新项目(无老 user)** | 空库,模块迁移建 `user_*` 空表,**不需要任何 adopt**,seed 一个初始管理员即可。 |
| **③ 有*不同*结构的老 admin(polaris/talkwithtong)** | 模块 4 条 schema 迁移在自己的 `user_` 孤岛/版本流里干净跑,建好空表;**数据采纳由该项目自己写一次性回填**(老表→`user_` 表),放进 **host 自己的迁移流**,编号在 host 空间、不碰 `user_` 流。老 admin 表保持不动、并行运行,最后 cutover。回填用 `INSERT IGNORE` + 显式列名并保持 id,可安全重跑追增量(参考下方 homeserver 样例)。 |

**关于随模块附带的 `20260921140000_user_adopt_admin_data.sql`**:它是 homeserver 专用样例(引用无前缀老 admin 表名),按上面铁律 **B,它不该待在共享模块的 `user_` 流里**。它现在还在,是历史包袱(homeserver 已把它作为该流的一员应用过)。方向:**把「采纳存量」从模块剥出、改为 host 私有一次性迁移**;新接入项目照它的形状写自己的版本,别拷它、别删模块里的号。具体剥离手法(homeserver 已应用该号、直接删文件会让 goose 报缺失)见 §8 待办。

## 7. 迁移计划(每批独立、可回滚)

> 每批的 goose/存量数据怎么对接,见 **§6b 存量系统迁移方案**。

1. **抽成可挂载模块 + 挂回 homeserver 验零回归**(已做:homeserver 侧先在本地证明形状;本 aurora PR 把 user + 通用件搬进 aurora)。
2. **homeserver 接入**(§6b 情况①):sync aurora → 删本地 `services/user` → main 改 `user.NewFeature(cfg)` + `user.Routes` → goose 迁移源重指到模块目录(版本号已应用 → no-op)→ 验零回归、eng 跑一遍。
3. **前端**:web/user → 共享 remote;homeserver 后台壳指过去。
4. **接入其余 3 个(成本不一)**:
   - **deploy**:从 homeserver 骨架 fork、~80% 同款,近乎即插即合;有没有独立老 user 决定它是 §6b 情况②还是③。
   - **polaris**:可能还在老 admin(§6b 情况③),是「从老 admin 迁到这套」的更重前置步——模块建空表 + 自写 host 私有一次性回填 + 并行跑到 cutover;先评估/迁移。
   - **talkwithtong**:现状待核(本地无仓),先核后台 user 形态与差距,再定 §6b 情况②/③。

## 8. 待做 / 开放

- **测试搬回**(本次搬移先移除测试聚焦结构):`modules/user/*` 各层测试、`middleware` 的 `jwt_middleware_test`/`dualmode_test`。(`encryption` 已补齐 AES-GCM 测试 ✅。)
- **`middleware` 去业务化(独立重构)**:通用 JWT 鉴权中间件里目前焊着 ①2FA 绑定域逻辑(`ScopeEnroll2FA`/`mustEnrollTwoFactor`)②写死的 `gin.H` 响应壳 ③硬编码中文。应:2FA 逻辑泛化成"受限作用域 token"或注入钩子(回到 user 模块)、响应走 `bizerr`、文案走 i18n。单独 PR,别塞进本次搬移。
- **前端**:源码**已进** `modules/user/web/`(aurora #71 ✅)。**待做**:从该源构建版本化共享 remote 的 CI + 各项目后台壳 pin 加载 + 各项目现有 `web/user` 切成消费这份(去本地副本)。
- **migrations 改 embed.FS 由模块自持**(现仍靠宿主 `InitDefaultApp` 的 goose + `GOOSE_TABLE_PREFIX`;下沉需连 App 初始化一起理)。这也是 homeserver 接入(§6b①)"goose 迁移源重指到模块目录"的落点。
- **把 `20260921140000_user_adopt_admin_data.sql` 从模块剥离**(见 §6b):它是 homeserver 专用数据采纳,按治理铁律 B 不该在共享 `user_` 流里。剥离手法注意——homeserver 已把该号应用进 `user_goose_db_version`,直接删文件 goose 会报"缺失迁移"。可选:①保留一个 no-op 占位(空 up/down)守住版本号连续、真正的采纳逻辑挪进 homeserver 私有一次性迁移;②或按 goose `allow-missing` 策略处理。定夺前别动,先确认 homeserver 现网 goose 行为。
- 限流 namespace **不是 Config 字段**(自动取 `SERVICE_NAME`);后台「被锁列表」索引已从 `loginguard.Guard.Namespace()` 取同一前缀(单一源,已非硬编码)。此前「`RateLimitNamespace` 待参数化」的说法作废。
- `types`:`status.go`(int)与 `enabled_disabled_status.go`(string)同概念不同类型的历史冗余,统一 = 改 DB 列/DTO 的数据变更,独立 cleanup。
- 注释里 "homeserver" 泛化为"宿主"。
- 通用「设置」框架(aurora `feature/setting` + `web/setting`)—— 独立 initiative,单独排(doorman 那套:中立框架 + 前端 + 业务配置留消费侧)。
