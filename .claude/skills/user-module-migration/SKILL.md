---
name: user-module-migration
description: 把 aurora 共享后台 user 模块(modules/user:账号/RBAC/登录/2FA/会话/微服务 token/gate)接入一个项目(homeserver/polaris/deploy/talkwithtong 等),含存量 user/admin 系统的安全迁移。当需要:新项目挂载 user 模块、把已有 admin/user 服务切到这套共享代码、处理 goose 版本表对接、校验 schema 一致、seed 本项目 feature 权限目录时使用。核心是「表结构归 aurora、权限数据归各项目」不能混。
---

# user-module-migration:把共享 user 模块接进一个项目

`modules/user` 是 4 个项目共用的后台账号中心(见 `doc/proposals/shared-user-center.md`)。这个 skill 是**接入 playbook**:一份代码换 `Config` 挂载,存量系统安全迁移,**永不重写、永不分叉**。

接入前先读三节 → 用「§1 判定」定位你是哪种情况 → 按「§3 分情况执行」走 → **对照「§2 铁律 + 坑」逐条自检**(混了 schema 和 data 一定出事)。

---

## §0 一句话地基(先记住,后面全靠它)

**表结构归 aurora,权限数据归各项目。** 这条决定了所有迁移动作的归属:

| | 内容 | 谁改 | 放哪个 goose 流 |
|---|---|---|---|
| **schema(表结构 DDL)** | `user_users`/`user_roles`/`user_features`/… 怎么建、有哪些列 | **只有 aurora** `modules/user/migrations` | 模块的 `user_` 流(版本表 `user_goose_db_version`) |
| **data(权限/授权/存量)** | 有哪些 feature 行(`ui.page.*`/业务权限)、role↔feature 授权、老数据 adopt | **各项目自己** | host 自己的迁移流(`goose_db_version`)或 host 开机 seed,**绝不进 `user_` 流** |

违反 = 分叉或炸库。两个方向的错都致命:
- 把 schema 改动写进 host → 各项目 schema 漂移,和 aurora 不一致。
- 把 feature/data 写进 aurora 的 `user_` 流 → 非中立(polaris 不想要 homeserver 的 `commission.*`)+ 下次 sync 带来的新迁移号和你塞的号撞车/goose 乱序。

**两道强制闸(不是靠人自觉,是代码写死):**
1. **启动 schema 闸(§4)**:模块每次启动校验 DB 结构满足所需,不满足 **FATAL + os.Exit(1)**。让"本地阶段准备迁移/baseline"变安全——搞错了第一次启动就死,不会带病运行。
2. **模块只查结构、绝不查内容**:闸只管"表在/列在",不管库里有哪些 feature 行(那是业务的事,归 host)。catalog 的正确/防幻影是 host 的开机 seed 负责。

---

## §1 判定:你是哪种情况

```text
目标项目有没有已经在用的 user/admin(账号表 + 数据)?
├─ 没有(全新项目) ─────────────────────────► 情况 ①「全新」
└─ 有
   └─ 它的表结构和模块的 user_* 一致吗?
      ├─ 一致 / 能对齐(如从 homeserver fork 的 deploy) ──► 情况 ②「同构存量」
      └─ 不同(老 admin 结构不一样,如 polaris)────────► 情况 ③「异构存量」
```

- **①「全新」**:空库,最简单。模块迁移建空表 + seed 一个初始管理员。
- **②「同构存量」**:表已在、结构对得上,不搬数据。**校验一致 → 把 goose 号 baseline 进版本表 → 直接沿用现有表**。核心是别让 goose 去重跑 CREATE。
- **③「异构存量」**:模块建新 `user_` 空表 → 自写 host 私有一次性回填(老表→`user_`)→ 老 admin 并行跑 → 最后 cutover。homeserver 自己就是这条路(`20260921140000_user_adopt_admin_data.sql` 就是它的搬家卡车)。

> 注:homeserver 是**源头**,那 5 条迁移号早已记在它的 `user_goose_db_version`,接入 = 把 goose 迁移源指到模块目录 → 按版本号判定(goose 默认无 checksum)→ 全部已应用 → no-op。本质是情况②的特例(结构天然一致,号也已写入)。

---

## §2 铁律 + 坑清单(逐条自检,最值钱的一节)

### A. goose 两层隔离(为什么模块 schema 是座孤岛)
1. **业务表名写死 `user_*`**(在迁移 SQL 里,不是靠前缀拼)→ 与项目已有的 `users`/`roles` 天然不撞名。
2. **版本表靠 `GOOSE_TABLE_PREFIX` 隔离**:homeserver `user_` → `user_goose_db_version`,和 host 自己的 `goose_db_version` **分开记账**。模块迁移流独立跟踪、永不和 host 的 app 迁移交叉编号。
3. **迁移在 `bootstrap.InitDefaultApp()` 里跑,早于模块 Setup**。所以前缀只能由部署期 env `GOOSE_TABLE_PREFIX` 给(**不是** Config 字段——`Config` 没有 `TablePrefix`,见 §2E)。

### B. 存量接入,顺序绝不能反:先校验、再写号
- 一旦把版本号写进 `user_goose_db_version`,goose **永远不再碰这些表**。若此时存量表少一列/类型不对,不一致被**永久固化** → 代码上线崩在字段不匹配(500)。
- **⚠️ 不能靠 `goose up` + `CREATE TABLE IF NOT EXISTS` 偷懒**:`IF NOT EXISTS` 只保证"表在就不报错",**不保证结构对**。存量表少一列它照样"成功"、照样写号,把不一致藏得更深。→ 必须**显式校验 + 显式写号**,不走这条捷径。
- **⚠️ 迁移别用 MariaDB-only 语法(消费库多是 MySQL)**:`ALTER TABLE ... ADD COLUMN IF NOT EXISTS` / `DROP COLUMN IF EXISTS` **只有 MariaDB 支持,MySQL 报 1064 语法错 → goose 失败 → `bootstrap.InitDefaultApp()` panic(exit 2)→ 服务回滚**。用朴素 `ALTER TABLE ADD COLUMN`(goose 每个迁移只成功跑一次,天然安全);`CREATE/DROP TABLE IF [NOT] EXISTS` 两端都支持可用。deploy 接入时就栽在这(第一版对账迁移用了 `ADD COLUMN IF NOT EXISTS`、api-user 启动即崩回滚)。

### C. feature 权限目录是 data,不是 schema —— host 声明式注册表,模块不掺和
- homeserver 有 **137 条 `services/admin/migrations/feature_*.sql`**,每条 `INSERT INTO features ... ON DUPLICATE KEY UPDATE`(按 name 幂等 upsert)——上个新页/新接口就加一条权限行 + 授权。
- 这些**天生一项目一套**(polaris 要 `funnel.*`、deploy 要 `cluster.*`),**不是分叉、是本该不同**。归 **host 自己**,不进 `user_` 流。
- **推荐机制:host 侧声明式注册表 + 开机幂等 upsert**(不是散在迁移里):
  - host 建 `rbaccatalog/`:`registry.go` 声明本项目**全部** Feature/Role/grants(一份看全、可 diff);`seed.go` 开机把它幂等 upsert 进 `user_features`/`user_roles`/grants + 防幻影(库里有、清单没有的标出来)。就是 doorman `WithScope` + `GET /scopes` 那套。
  - **⚠️ 红线:upsert 循环 + 注册表都在 host,模块压根不 import、不知道 catalog 概念**。否则就成了往模块加 hook,违反设计稿 §0"纯配置、不留 hook"。模块给表 + 只查结构的闸,host 自己填。
- **和 137 条迁移的关系**:不用一次性重写。注册表 upsert 和现存迁移都是"按 name 幂等"、写同一批行、天然收敛 → 可**渐进**(存量迁移不动,新 feature 进注册表;想清爽了再批量搬)。
- **cutover 细节**:homeserver 现存 feature seed 写的是老表 `features`;切到模块后新加的(不论迁移还是注册表)要写 `user_features`。

### D. adopt 迁移是 host 私有,不该在共享模块
- `modules/user/migrations/20260921140000_user_adopt_admin_data.sql` 是 homeserver 专用(引用无前缀老 admin 表名),按铁律它不该在共享 `user_` 流里。它现在还在是历史包袱(homeserver 已应用过该号)。
- **新项目照它的形状写自己的回填,放 host 流,别拷它、别删模块里的号**。剥离手法(homeserver 已应用、直接删文件 goose 报缺失)见设计稿 §8。

### E. 挂载配置别硬编码(照 Config 走)
- 差异只经 `user.Config`,**当前就三个字段**(以 `modules/user/config.go` 为准):
  - `TOTPKeyEnv`(凭据加密密钥的 env 名;留空 → `USER_GOOGLE_TOTP_AUTH_KEY`)
  - `GateCookie`(下载门禁 forwardAuth cookie 名;留空 → `admin_gate`)
  - `LoginPolicyProvider`(可选:登录限流阈值来源,让阈值运行时可调,如读设置表;留空 → 内置 StaticPolicy 硬锁默认,零回归)
- ⚠️ **不存在 `RateLimitNamespace` / `TablePrefix` 这两个 Config 字段**(老文档写过、是错的):
  - **限流 namespace 不在 Config**:留空自动取 `SERVICE_NAME`;后台「被锁列表」索引从 `loginguard.Guard.Namespace()` 取同一前缀(单一源,别再按老说法手改索引常量)。
  - **goose 表前缀不在 Config**:走部署期 env `GOOSE_TABLE_PREFIX`(见 §2A.3)。

### F. 凭据密钥 fail-fast
- 模块 Setup 会强制校验 `TOTPKeyEnv` 指向的密钥(空/无效 → FATAL + os.Exit(1)),**绝不回落占位密钥**。接入前先把该 env 配进 chart/secret(base64 32 字节或任意口令)。多个共享同一份凭据的服务必须同一把密钥。

### G. 别只接后端——前端 federation + gate + 登录态对齐(deploy 在这栽过死循环)
user 模块是**全栈**的:只挂后端,用户进不了用户中心。前端 = host 懒加载 user remote(Native Federation),完整说明见 [`modules/user/web/README.md`](../../../modules/user/web/README.md),这里只给接入时**必踩的三条**:

1. **消费 remote,别拷源**:host 把 `web/user` 改成 Federation host,懒加载 `loadRemoteModule('user','./Routes')`;从 `modules/user/web` 源构建一份 **web-user remote 镜像**(base-href `/user/`)独立部署,host pin manifest 加载。别把 remote 的 `src/app` 拷进 host(= drift)。
2. **gate 由 remote 镜像提供,别注入 host**:gate 是 remote 自带静态壳(`public/gate` → 镜像 dist 根 `/gate/`);Traefik 把 `/gate` 路由到 **web-user 镜像**(公开、不挂 forwardAuth、不 stripprefix),host 零 gate。详见 web/README「gate 登录壳怎么部署」。(homeserver 用"注入 host"反模式;deploy 已改推荐做法。)
3. **⚠️ 登录态约定必须全栈对齐(不对齐必死循环)**:remote/gate **硬编码** `admin_access_token`/`admin_refresh_token`/`admin_user`/`admin_2fa_enrollment_pending`(localStorage)+ `admin_gate`(cookie)。host 壳必须全部用这套:
   - host SPA 的 storage 层写同样的 key(remote 与 host 同源共享 localStorage,remote 的 guard 硬读 `admin_access_token`);
   - 门禁 forwardAuth 的 verify 指 **api-user `/api/<user>/v1/auth/gate/verify`**(读 `admin_gate`);
   - 任何旁路读该 cookie 的(如 SSR 控制台)也改成 `admin_gate`。
   - host 原本用别的 key(deploy 曾用 `deploy_*` / `access_token`)**必须一并全改**,只换一半 → remote 读不到 token / gate 与 forwardAuth 对不上 → 疯狂 `/auth/refresh` 死循环。key 名用 `admin_*` 不碍各 host 独立(不同 host 独立 origin,localStorage 天然隔离)。

---

## §3 分情况执行

### 情况 ①「全新项目」
1. `make sync-aurora REF=<含 user 模块的版本>` 拉进 `third_party/aurora`。
2. main 挂载:
   ```go
   app := bootstrap.InitDefaultApp()
   app.AddFeature(user.NewFeature(user.Config{TOTPKeyEnv:"...", GateCookie:"..." /*, LoginPolicyProvider: ... 可选 */}))
   app.RegisterRoutes(user.Routes(app))
   // 注:namespace 自动取 SERVICE_NAME、表前缀走 env GOOSE_TABLE_PREFIX,都不是 Config 字段(见 §2E)。
   ```
3. 部署期 export `GOOSE_TABLE_PREFIX=user_`;goose 迁移源指向模块 migrations。
4. 起服务 → 迁移建空 `user_*` 表(**不跑 adopt**,删/占位那条)。
5. seed 一个初始管理员(host 侧一次性)+ seed 本项目 feature 目录(host 流,见 §2C)。

### 情况 ②「同构存量」(先对齐、再 baseline,启动闸兜底)
1. sync + 挂载 + 配 env(同①的 1–3)。
2. **本地阶段对齐 schema**:比对存量表和模块所需(§4 辅助手段:scratch 库 dump-diff),有 drift 走 expand-contract 补齐。
3. **baseline goose 号(§5)**:把模块这批版本号写进 `user_goose_db_version`,标记已应用。
4. 起服务 → **启动 schema 闸(§4)自动校验**:对齐漏了 / baseline 写漏号 → 直接 FATAL,不会带病跑;通过则 goose 跳过已应用号、沿用现有表。
5. 后续 feature/role 目录走 host 注册表(§2C)。

### 情况 ③「异构存量」(建新表 + 回填 + cutover)
1. sync + 挂载 + 配 env。
2. 模块 4 条 schema 迁移在自己的 `user_` 流干净跑,建**空** `user_*` 表。
3. **自写 host 私有一次性回填**(老表→`user_`,参考 adopt 的形状:显式列名 + 保持 id + `INSERT IGNORE` 可重跑),放 **host 的 `goose_db_version` 流,编号在 host 空间**。
4. 老 admin 表不动、并行运行;回填可反复跑追增量。
5. 验证稳 → **cutover**:切流量到新服务、停老 admin、删老代码/表(见记忆 user-service-migration)。
6. feature seed 走 host 流。

---

## §4 verify:启动期 schema 一致性闸(承重机制,代码写死)

**这是保证安全的核心,不是可选的子命令——是模块每次启动都跑的 FATAL 闸。**

- **在哪**:`modules/user/schema_guard.go`,`feature.go` 的 `Setup` 里调(紧跟 `ValidateCredentialKey` 的同款模式:不过就 `logger.Errorf("[FATAL] ...")` + `os.Exit(1)`)。绝不带病启动。
- **查什么**:模块所需的每张表在、每个必需列在(**只查结构,不查内容**——库里有哪些 feature 行是 host 的事)。
- **怎么查:从 entity 自省,不手写描述文件**。entity 是代码对 schema 的唯一契约(`TableName()` + `gorm:"column:..."`),自省 = 永不和迁移漂移:
  ```go
  m := db.Migrator()
  for _, e := range []any{&entity.User{}, &entity.Role{}, &entity.Feature{}, /* … */} {
      if !m.HasTable(e) { /* 记:表缺失 */ ; continue }
      stmt := &gorm.Statement{DB: db}; _ = stmt.Parse(e)
      for _, f := range stmt.Schema.Fields {
          if f.DBName == "" { continue }              // 跳过关联字段(无列)
          if !m.HasColumn(e, f.DBName) { /* 记:缺列 */ }
      }
  }
  // 有 problem → Setup 里 FATAL + os.Exit(1)
  ```
- **抓得住什么**:baseline 写漏号、存量表 drift、goose 源指错 → **第一次启动就死**,不会静默固化成运行时 500。
- **类型精确校验**(`Migrator().ColumnTypes`)可作后续增强;先做"表在+列在"已覆盖所有真会崩的 drift。
- **辅助(非承重)**:开发/CI 里可拿模块迁移在干净 scratch 库 `goose up` + `mysqldump --no-data` 导规范 DDL 和存量库 diff,做更细的对齐检查。但**运行时的承重闸是上面这道 boot 校验**。

## §5 baseline:把 goose 号标记为已应用(不重跑)

goose 版本表:`user_goose_db_version(id, version_id, is_applied, tstamp)`。baseline = 对每个"已存在且已校验一致"的号 INSERT 一行:

```sql
INSERT INTO user_goose_db_version (version_id, is_applied, tstamp) VALUES
  (20260921100000, 1, now()),
  (20260921110000, 1, now()),
  (20260921120000, 1, now()),
  (20260921130000, 1, now());
  -- 140000(adopt)对同构存量也标记跳过:它是 homeserver 专用搬数据,这里不该跑
```

之后 goose `up` 看到已应用 → 跳过;将来 aurora 出 `150000`,号更大 → 正常往下跑。**必须先 §4 校验一致、再执行 §5**(见铁律 B)。同样应做成模块自带的确定性 `baseline` 子命令。

---

## §6 执行顺序清单(接入一个项目)
1. **判定情况**(§1):有没有存量 user/admin、结构一致否。
2. **sync + 挂载 + 配 env**(`user.Config` + `GOOSE_TABLE_PREFIX` + 凭据密钥 secret · 坑 E/F)。
3. **存量项目先 verify(§4)**:有 drift 停,expand-contract 对齐。
4. **按情况处理表/数据**:①建空表+seed / ②baseline / ③建空表+host 回填(§3)。
5. **feature 目录 seed 走 host 流**(§2C),别碰 `user_` 流。
6. **前端接入(§2G)**:host 改 Federation host 懒加载 user remote + 构建 web-user remote 镜像部署 + `/gate` 路由到该镜像 + **host 登录态全栈对齐 `admin_*`/`admin_gate`**(只接后端用户进不去 / 只换 gate 必死循环)。
7. **起服务验证**:迁移无重跑、`/health` 通、登录+2FA+gate 通、`/user-center` remote 加载出、后台被锁列表读得到(namespace 对)。
8. **③ 额外**:并行验证稳 → cutover 停老 admin。

---

## §7 落地状态 / 开放问题
- **启动 schema 闸(§4)**:设计已定(entity 自省 + Setup FATAL),**代码待写**(`modules/user/schema_guard.go`)——建议独立小 PR,是模块自带的承重安全件。落地前存量对齐靠 scratch 库 dump-diff 人工兜。
- **host feature/role 注册表(§2C)**:设计已定(host `rbaccatalog/` 声明 + 开机 upsert + 防幻影,模块不掺和),**代码待写**,属各项目接入 PR(homeserver 先做)。渐进,不必一次搬完 137 条。
- **baseline(§5)**:落地前靠手工 SQL;可后续做成模块自带确定性子命令。
- **这个 skill 怎么到达消费项目的 agent**:它在 aurora(单一源)。**只有把 aurora 整棵树 vendor 进 `third_party/aurora` 的项目**(README「消费 aurora:两种方式」第 ② 类 —— 因防逆向需隐藏上游身份才整树 vendor + 改 import)才能靠 `scripts/post-sync.sh` 把它镜像进项目根 `.claude/skills/`。走 `go get` 直接依赖的项目(第 ① 类)其 `third_party/aurora` 里只有 web/迁移源(甚至没这文件夹),不含 `.claude/skills/`,不经此获得本 skill——**别为了拿它去引入整树 vendor**;要不要、怎么拿由该项目自己决定(直接放一份 / 不放)。其余方案(薄指针 / 共享 skill 仓)仍待产品化拍板。

---

## §8 维护:改了模块「对外面」就同步这些文档(别只改一处)

user.Config / 路由 / gate / 前端约定这些**对消费方可见的契约**被抄在多处文档里——改一处必漏几处(实测:Config 字段曾在模块 README + 本 skill + 设计稿三处抄错,漏 `LoginPolicyProvider`、多 `RateLimitNamespace`/`TablePrefix`)。改了下列任一就**全量同步**对应文档:

| 改了什么 | 要同步的文档 |
|---|---|
| `user.Config` 字段(config.go) | `modules/user/README.md`(用法 + 对外入口)、`modules/user/web/README.md`(涉前端时)、设计稿 `§4.1`、本 skill §2E/§3/§6 |
| 路由(routes.go) | `modules/user/README.md`(对外入口)、设计稿 §3 功能 |
| gate 部署 / 登录态 cookie·localStorage 约定 | `modules/user/web/README.md`(gate 部署 / 登录态对齐节)、设计稿 §4.3.1/4.3.2、本 skill §2G |

**自动闸(别只靠自觉)**:`make doc-check`(= `go test ./modules/user/ -run TestDocs_`,已含在 `make test` / CI)——①文档里 `user.Config{}` 示例字段都真实存在(反射 config.go)②每个真实字段都在模块 README 出现。漏同步/抄错字段,CI 直接红。新增要盯的文档就加进 `modules/user/docs_config_test.go` 的 `configDocs`。

**通法**:评估「改这个要同步哪些文档」时**对整仓全量扫**(`git ls-tree -r --name-only HEAD | grep -iE '\.md$'` 排 `vendor/` 再逐个 grep),别只盯脑子里那个子目录——漏文档基本都是 scope 太窄。
