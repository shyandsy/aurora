# Proposal:共享「后台 User 中心」——跨项目单一源,永不重写

> 状态:提案(方向已定,进入设计)。面向:aurora 维护者 + 要接入的各项目(homeserver / polaris / deploy / talkwithtong)工程师。

## 0. 决策(本轮已拍板)

| # | 决策 | 定论 |
|---|---|---|
| 落点 | 独立仓(sealkit 模式,依赖 aurora)**vs** 塞进 aurora `usercenter/` | **独立仓** —— 保 aurora 纯框架定位;usercenter 自带 controller/migrations/路由,是「服务模块」不是 feature。 |
| 部署模型 | 各起一份实例、数据各自隔离 **vs** 多项目共用一份 | **各起一份、数据隔离** —— 代码共享、实例/库各自独立。 |
| 前端 remote | 每个 host pin 版本 **vs** 全跟最新 | **每个 host pin 版本** —— 防一次坏部署打挂 4 个后台登录。 |
| gate | 随 usercenter 模块带 **vs** 独立 `feature/gate` | **随模块带**。 |
| 模型扩展点 | 留扩展 hook **vs** 纯配置 | **纯配置、不留 hook** —— 固定模型实测扛住了多份拷贝;真有项目要加字段再说。 |
| Schema 治理 | —— | **唯一真风险,动手前必须先有政策(见 §5.5)。** |

## 1. 背景与目标

我们已经有 **4 个项目**都需要「后台账号管理 + 登录门禁(gate)」:homeserver、polaris、deploy、talkwithtong。现状是**各自一份 user 服务**,任何一处改动,另外三个都得跟着手抄——费劲且迟早跑偏。

**本质上这 4 份是同一个东西**:后台账号 / RBAC / 登录 / 2FA(TOTP) / 会话设备 / 微服务 token / 登录暴力破解防护 / 会话有效性 / 被锁管理 + gate。差异只在**配置**(DB、密钥、品牌、挂哪些路由),**不在代码**。

**目标**:做成**一份共享代码**,4 个项目 `import + 配置` 接入,**永不再重写**;改一次,全项目经现成的 `sync-aurora` 机制同步拿到。

**非目标(明确排除)**:**customer 客户体系**(注册/找回密码/邮箱验证/轰炸防护/会员)**不在此**——那是各项目面向终端客户的业务服务,和「后台 user 中心」是两回事,别混进来。

## 2. 现在 homeserver 里有什么(要搬的清单)

### 2.1 后端 `services/user`(Go,建在 aurora 上)

| 包 | 干什么 |
|---|---|
| `controller/{auth,user,role,role_feature,feature,microservice,ratelimit}` | 登录/登出/刷新/2FA/**gate**、账号、角色、权限、微服务 token、被锁管理的 HTTP 层 |
| `service/{user,role,role_feature,rolefeaturepublisher,feature,microservice_token,microservice_token_feature,ratelimit}` | 对应业务逻辑;user 含 loginguard/tokenguard 接入、precheck/record、被锁列表/解锁 |
| `datalayer/{user,user_session,role,role_feature,feature,microservice_token_feature}` | 表访问 |
| `migrations/*` | `user_rbac_core` / `user_ui_feature_grants` / `user_session` / `user_microservice_token` / `user_adopt_admin_data`(goose) |
| `model` / `locales` | DTO/实体、三语文案 |

**它已经复用的 aurora feature(不用搬,引用即可)**:`loginguard`、`tokenguard`、`geoip`、`jwt`、`redis`、`gorm`、`i18n`、`mail`。

### 2.2 Gate(登录门禁)

`controller/auth/gate.go` 的 `GateVerify` = Traefik **forwardAuth** 目标:`admin_gate` cookie 镜像 SPA 的 access token(forwardAuth 只看得到 cookie),校验有效→放行下发 SPA,否则→跳登录壳。配套前端是登录壳页面(见下)。这套「登录壳 + forwardAuth + TOTP」4 个项目都要。

### 2.3 前端 `web/user`(Angular,Native Federation remote)

| 部分 | 内容 |
|---|---|
| `features/user-center/pages/*` | user / role / role-feature / microservice / user-permission(壳含 tab)/ locked-logins |
| `pages/{login,two-factor-enroll}` | 登录壳 + 2FA 绑定(gate 的前端) |
| `shell/{contributions.ts,i18n-merge.ts}` | Native Federation 贡献套件(把用户中心路由 + i18n 词典合并进宿主) |

## 3. 搬到哪、怎么放

### 3.1 后端 → 独立仓 `usercenter`(sealkit 模式,依赖 aurora)

**决策(§0):放独立仓,不进 aurora。** 理由:aurora 是纯框架(`feature/*` 是库,没有 `services/`);usercenter 自带 controller / migrations / 路由,是「服务模块」不是 feature,塞进 aurora 会糊掉框架定位。做法照 **sealkit / di 的模式**:一个独立 Go 仓(`github.com/shyandsy/usercenter`),**依赖 aurora**,各项目经 `make sync-usercenter REF=<ref>`(和 sync-aurora / sync-di / sync-sealkit 同一套)拉进 `third_party/usercenter`。

```
usercenter/                   ← 独立仓,依赖 aurora
  controller/  service/  datalayer/  model/  migrations/(embed.FS)  locales/
  feature.go                  ← NewUserCenterFeature(Config) contracts.Features:装配全部 DI
  routes.go                   ← Routes(app) []contracts.Route:供宿主 RegisterRoutes
  gate.go                     ← forwardAuth GateVerify(随模块,§0 决策)
```

- 引用已在 aurora 的 loginguard/tokenguard/geoip/jwt/redis,不重复实现。
- 迁移随模块用 **embed.FS** 带(去掉运行时读目录的部署脆弱),宿主挂载时按自己的 `GOOSE_TABLE_PREFIX` 跑;版本/兼容/回滚政策见 **§5.5**。

### 3.2 Gate → 随模块(或独立 `feature/gate`)

登录壳 + forwardAuth + TOTP 是通用件。倾向随 `usercenter` 模块带;若别的非-usercenter 场景也要 gate,可拆独立 `feature/gate`。

### 3.3 前端 → **不进 aurora**(Go 框架放不了 Angular)

`web/user` 是 Angular Native Federation remote,**单一源靠"共享 remote"实现,不靠进 aurora**:

- 放一个**独立中性前端仓 / 平台前端仓**,构建部署**版本化的** user remote(如 `/user/v4/remoteEntry.json`)。
- 4 个项目的后台壳(host)`federation.manifest.json` **pin 到某个版本的** remote(§0 决策:每个 host pin,防一次坏部署打挂 4 个后台登录)。
- **升级 = 该 host 显式 bump 自己 manifest 指向新版本**——这是改 manifest / 部署配置,**不重构建 host**,但**不自动**(pin 就是用"自动传播"换"各 host 自控、坏了不连累别人")。共享的是"一份代码/一次构建",不是"所有 host 自动跟最新"。
- i18n / 路由经现有的 `shell/contributions.ts` + `i18n-merge.ts` 贡献套件合并进各宿主。

## 4. 别的项目怎么用(接入契约)

### 4.1 后端:AddFeature + 配置 + 挂路由

```go
app.AddFeature(usercenter.NewUserCenterFeature(usercenter.Config{
    TablePrefix:   "user_",              // goose 表前缀,同库多服务隔离
    JWTSecretEnv:  "XXX_JWT_SECRET",     // 各项目自己的密钥来源
    TOTPKeyEnv:    "XXX_TOTP_AUTH_KEY",
    Brand:         "从 values 注入",      // gate/登录页显示名,不进代码
    GateCookie:    "admin_gate",         // forwardAuth cookie 名(可默认)
    // …仅配置,无代码差异
}))
app.RegisterRoutes(usercenter.Routes(app))
```

**差异全走 config**(DB/前缀、密钥、品牌、挂哪些路由),用 aurora feature 现成的 **chart + env** 方式,不发明新配置。

### 4.2 前端:指向共享 remote

后台壳 `federation.manifest.json` 加一条指向共享 user remote;贡献套件自动合并用户中心路由 + i18n。

### 4.3 版本化 / 升级

走现成 `make sync-aurora REF=<ref>`(pin + PR),和 loginguard/tokenguard 同一套——各项目自己控制何时升级,不会一改全崩。

## 5. 中立化要求(硬性)

进 aurora 的这套**必须中立**:品牌 / 域名 / 业务名 / 私有镜像仓**只进 config、不进代码/注释/类名**。搬迁时要把 homeserver 特定的东西(如 `USER_GOOGLE_TOTP_AUTH_KEY` 这类具名 env、admin-gate 的固定 cookie 名、任何 homeserver 字样)改成通用配置键。

## 5.5 Schema / 迁移跨项目治理(唯一真风险,动手前必须先立的政策)

代码越一致,风险越集中在**一处 schema 改动要同时对 N 个项目安全**。这条不定政策,不动手。

**① 单一 owner:schema 只由 `usercenter` 仓改。**
所有表结构变更都进 usercenter 的 migrations(embed.FS),**任何项目不得手改 usercenter 的表**、不得往 usercenter 加迁移。usercenter 迁移版本号单调、由本仓唯一维护。

**② 只加不改、expand-contract:每个迁移必须对"旧代码 + 新代码"都安全。**
- 加列一律 nullable / 带默认;要改/删列走两阶段:先 expand(加新、双写/回填)、跨版本后再 contract(删旧),**绝不在同一个 release 里删/改正在被用的列**。
- 这样任一项目在「拉了新迁移、还没换全代码」的中间态也不崩;回滚 = 退回上一个 usercenter ref(代码),加上去的列留着无害。
- 每个迁移都有可跑的 down;destructive 操作(drop/rename/truncate)默认禁止,确需要走 expand-contract 的 contract 阶段、且单独 release。

**②-bis 跨版本跳跃防护(expand-contract 的缝,必补)。**
expand-contract 只保**相邻版本**安全。但各项目独立 pin、自控节奏(③),某项目可能一次从 v3 跳到 v6——一次部署把 v4/v5/v6 迁移全跑完(列已 contract),而滚动重启期间**旧的 v3 pod(早于 expand、还在用该列)对上已删列的 schema → 部署中崩**。跨多版本会击穿 expand-contract。政策:
- **支持窗口(主)**:usercenter 声明支持最近 K 个版本;**contract 只允许删「支持窗口内所有版本代码都已不用」的列**。这样任何 ≥ current-K 的项目跳版都安全,不限制项目升级节奏,代价是列多留 K 个版本。
- **最低版本地板 + 不可跳过 contract(兜底)**:低于支持窗口的项目,必须先部署 contract 前一步 expand 的**代码**、再跨过 contract release。CI/迁移工具对「drop 的列在支持窗口内仍被引用」报警拦住,不让这种 contract 发布。

**③ 各项目 pin + 自控节奏:升级是一次显式 PR。**
项目经 `make sync-usercenter REF=<ref>` pin 版本,自己决定何时升。升级 = bump ref 的 PR,先上 eng 跑迁移验证(同现有 `feature/* 立即部署 eng 跑 goose` 的约束:迁移号首推前排在最新之后、推过的不改号),绿了再进 prd。**不会"一改全崩"**——是各项目按自己节奏滚。

**④ 表隔离:`GOOSE_TABLE_PREFIX` 按项目/实例。**
各项目各起一份实例、各自的库(§0 部署决策),usercenter 表带项目前缀,同库多服务也不撞;goose 版本表随前缀隔离。

**⑤ 兼容测试是发布门槛。**
usercenter 仓 CI:干净库跑全套 up + down、再 up(验可回滚);迁移**不得假设任何项目特有数据**。跨版本兼容(N-1 代码 + N schema)要有测试或 checklist。

**⑥ 破坏性/不可回滚变更走 RFC。**
真需要不可回滚的大改(极少),单独提 RFC、列各项目影响与迁移窗口,评审通过再做。

> 一句话:**owner 单一 + 只加不改(expand-contract) + 各项目 pin 自控 + 表前缀隔离 + 可回滚测试**。这套定了,共享 schema 才安全。

## 6. 分批迁移计划(每批独立、可回滚)

1. **抽后端**:`services/user` → aurora `usercenter` 模块;**挂回 homeserver 自己,验零回归**(只搬位置 + 把品牌/DB/密钥收成 config,不改行为)。
2. **前端**:`web/user` → 独立共享 remote;homeserver 后台壳改指向它。
3. **接入其余 3 个(接入成本不一样,别默认都轻)**:
   - **deploy**:从 homeserver 骨架 fork、逐字节 ~80% 同款,近乎即插即合——`AddFeature(usercenter)` + 配 values + 指向共享 remote 即可。
   - **polaris**:**可能还在老 admin,是"从老 admin 迁到这套"的更重前置步**(它本就要套 homeserver 同款 gate)。接入前先评估/迁移,不能和 deploy 一视同仁。
   - **talkwithtong**:现状**待核**(本地无仓);接入前先核它后台 user 的实际形态与差距。

## 7. 开放问题

§0 已把大方向拍完(落点=独立仓、部署=各起一份、前端=host pin 版本、gate=随模块、模型=纯配置)。剩下待定:

1. **前端共享仓放哪**:独立 `user-web` 仓 vs 统一「平台前端」仓(两者都满足"部署一份 remote、各 host pin 版本",只是仓的归属)。
2. **`sync-usercenter` 机制细节**:照 sync-aurora/di/sealkit 的 Makefile 目标 + 改名藏 shyandsy(如需)+ `third_party/usercenter` 布局——落地时定。

## 8. 建议(下一步)

方向已定,按 §6 第 1 步动手:**先把 homeserver 的 user 中心抽成独立 `usercenter` 模块并挂回 homeserver 自己验零回归**(只搬位置 + 收 config,不改行为、可回滚),同时把 §5.5 的 schema 治理政策在这套真实代码 + CI 上落实。通了再接入其余 3 个项目。
