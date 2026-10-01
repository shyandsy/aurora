# modules/user —— 后台账号中心(可挂载服务模块)

多项目共享的**后台账号中心**:一份代码,各项目换 `Config` 挂载,永不重写。完整设计/迁移/治理见 **[设计稿](../../doc/proposals/shared-user-center.md)**。

## 功能

- **账号 + RBAC**:用户增删改查、角色、权限(feature)、角色→权限展开(读模型)。
- **登录**:密码登录 + 2FA(TOTP) + 会话/设备清单(列出/撤销/滚动续期)。
- **登录防护**:建在 aurora `loginguard`(失败计数→锁)+ `tokenguard`(会话有效性/撤销/IP 绑定)上;后台「被锁列表 + 解锁」。
- **微服务 token**:对外签发「服务间调用」长效 token + 启停。
- **gate**:登录壳 + Traefik forwardAuth + TOTP(cookie 镜像 SPA token),后台 SPA 下载门禁。**部署方式**(谁在 `/gate` 提供登录壳 + host 登录态约定必须对齐)见 [web/README 的「gate 登录壳怎么部署」「登录态约定对齐」](web/README.md)。

## 用法

```go
app := bootstrap.InitDefaultApp()          // aurora 起 redis/jwt/gorm/migrations
app.AddFeature(user.NewFeature(user.Config{
    // 全部字段可选,留空各自回落默认(见 config.go withDefaults / resolveLoginPolicyProvider)。
    TOTPKeyEnv:          "MYPROJ_TOTP_KEY",            // 凭据(TOTP 等)加解密密钥的 env 名;留空→USER_GOOGLE_TOTP_AUTH_KEY
    GateCookie:          "myproj_gate",                 // 下载门禁 forwardAuth cookie 名;留空→admin_gate
    LoginPolicyProvider: loginpolicy.NewDBProvider(..), // 可选:登录限流阈值来源,让阈值运行时可调(如读设置表);
                                                        // 留空→内置 StaticPolicy(编译期硬锁默认),零回归
}))
app.RegisterRoutes(user.Routes(app))
app.Run()
```

- **差异全在 `Config` + chart/values 的 env/secret**,业务代码零行。`Config` 当前只有上面三项。
- **登录限流 namespace 不在 Config**:留空自动取 `SERVICE_NAME`(按服务天然解耦);后台「被锁列表」索引从同一前缀取,单一源。
- **goose 表前缀不在 Config**:走部署期环境变量 `GOOSE_TABLE_PREFIX`(只前缀版本表,业务表名写死在迁移里),与本模块配置无关。
- 拿到这份代码 = 各项目现有的 `make sync-aurora REF=<含本模块的版本>`(和拿 loginguard/tokenguard 一样,无需新 sync 机制)。
- 各项目各起一份实例、各自的库,数据隔离;迁移按各自表前缀跑。

## 对外入口(只这三个)

| 入口 | 作用 |
|---|---|
| `user.Config` | 项目差异配置(上面那几项) |
| `user.NewFeature(cfg) contracts.Features` | `Setup` 里装配全部依赖 + 启动校验(凭据密钥 fail-fast)+ 读模型重建 |
| `user.Routes(app) []contracts.Route` | 全部路由 |

## 结构

```
modules/user/
├ config.go feature.go routes.go   ← 对外入口(上表)
├ controller/   auth(含 gate)/ user / role / role_feature / feature / microservice / ratelimit
├ service/      user / role / role_feature / feature / microservice_token / rolefeaturepublisher / ratelimit
├ datalayer/    user / user_session / role / role_feature / feature / microservice_token
├ model/        dto(API 形状)/ entity(DB 行)—— user 域业务模型
├ migrations/   goose(user_rbac_core / ui_feature_grants / session / microservice_token / adopt_admin_data)
└ locales/      三语文案
```

依赖 aurora 的:`loginguard` / `tokenguard` / `geoip` / `jwt` / `redis` / `gorm`,以及顶层共享库 `middleware`(JWT 鉴权)/ `encryption`(凭据加解密)/ `types`。**不含任何 homeserver/业务代码**,项目差异只走 `Config`。

## 前端

配套前端就在本模块正下方 **[`web/`](web/)**(`modules/user/web/`):完整的 Angular **Native Federation remote** SPA(登录壳 + 账号/角色/权限/2FA/会话/微服务 token/被锁列表页),和后端**同源共置**。aurora 只存源码、不构建;消费方从该源构建**一份版本化共享 remote**,各项目后台壳 pin 版本加载(**别各项目各拷**)。详见 [modules/user/web/README](web/README.md) 与设计稿 §4.3。

## 注意 / 待办(见设计稿 §8)

- **Schema 跨项目治理**:表结构只由本模块 `migrations` 改;只加不改(expand-contract)+ 支持窗口 + 各项目 pin 自控节奏(详见设计稿 §6)。
- 测试待搬回;migrations 待改 embed.FS 由模块自持(现仍靠宿主 goose + `GOOSE_TABLE_PREFIX`)。
