# user 模块权限:机制归 aurora、数据归项目

> 状态:提议,待评审 ｜ 日期:2026-10-18
> 面向:aurora 维护者,以及已接入或将接入 user 模块的项目(homeserver / polaris / deploy / talkwithtong)工程师。本文不假设读者经历过任何一次具体调试,背景自足。
> 关联:本文细化 [`shared-user-center.md`](./shared-user-center.md) 中「表结构归 aurora、权限数据归各项目」这一条。

## 摘要

aurora 的共享 user 模块应只提供「权限机制」(`user_*` 表结构 + RBAC 引擎 + 后台编辑页),而「有哪些权限数据」(feature、角色、授权、页面清单)应由各消费项目自己提供。现在有两处违反:前端的页面清单写死在 aurora 代码里;后端项目自己的权限数据迁移没被打进镜像、无法生效。本文提议把这两处改成「数据由项目注入」,并给出可落地的机制与顺序。aurora 的后端引擎与表结构已基本中立,改动量不大。

## 背景与问题

homeserver 接入 user 模块后,给后台加一个新页和一个自定义角色时,发现它们的权限数据在部署中不生效。排查指向同一个根因:**项目的业务权限数据,要么被迫写进 aurora,要么没有能生效的落点。** 三个症状:

1. **项目的权限迁移不生效**:homeserver 把自己的权限数据迁移放在 `services/user/migrations/`,但 api-user 镜像构建时只把 aurora 模块自带的迁移打进镜像(见根 [`Dockerfile`](../../Dockerfile) 的 user 打包段)。项目目录里的迁移进不了镜像,goose 读不到,seed 从未执行。
2. **前端页面清单写死在 aurora**:角色权限编辑页的页面清单(代码里叫 PAGE_LABELS)硬编码在 aurora 的 [`modules/user/web`](../../modules/user/web),目前列着 20 多条消费项目的页名。每加一页都要改 aurora;其它项目也会看到不属于自己的页。
3. **aurora 夹着项目专用迁移**:aurora 模块迁移里有一条 `user_adopt_admin_data`,是把某个项目旧表数据搬进来的一次性迁移,对其它项目无意义。

## 术语速览

- **PAGE_LABELS**:aurora 前端「角色权限」编辑页里一个硬编码的「页面清单」数组,决定编辑页能按哪些页授权。
- **goose**:本套项目用的数据库迁移工具,按版本号顺序执行 SQL,用一张版本表记录已执行到哪。
- **adopt 迁移**:指 `user_adopt_admin_data`,把某项目旧 admin 表数据拷进 `user_*` 表的一次性迁移。
- **`ui.page.*`**:一类 feature 命名,代表「某后台页的授权入口」(如 `ui.page.order.operate`);登录发 token 时展开成该页对应的一组接口权限。
- **host 迁移**:消费项目自己仓库里的迁移(相对 aurora 模块自带的迁移而言)。
- **remote**:aurora 前端作为微前端被各项目后台加载的那一份;角色权限编辑页在其中。

## 原则 / 目标

**机制归 aurora(所有项目复用),数据归项目(各自注入)。**

- **机制**(aurora 提供):`user_*` 表结构;RBAC 引擎(feature 展开、鉴权中间件、token 签发);用户 / 角色 / feature / role-feature 的增删改查与后台编辑页。
- **数据**(项目提供):具体有哪些 feature、哪些角色、哪些授权、编辑页要显示哪些页。

**边界要求:aurora 的代码与迁移里,不出现任何一个项目的具体 feature 名、角色名、页名。**

**非目标**:不改 RBAC 鉴权语义;不改 `user_*` 表结构;不动面向终端客户的 customer 体系。

## 现状(本文审计时的 aurora main)

| 位置 | 状况 | 判定 |
| --- | --- | --- |
| 4 个 schema 迁移(rbac_core / ui_feature_grants / session / microservice_token) | 纯建表,无数据 INSERT | 已中立 |
| RBAC 引擎(token 签发 / feature 展开 / CRUD) | 读表运行,无硬编码 feature / role 名 | 已中立 |
| `GET /feature` 接口([`modules/user/routes.go`](../../modules/user/routes.go)) | 通用查询,已存在 | 可供前端数据驱动 |
| adopt 迁移 `user_adopt_admin_data` | 从某项目旧表拷数据 | 项目专用,须搬走 |
| 前端 PAGE_LABELS | 硬编码 20+ 条项目页名 | 数据写死在机制里 |
| 前端 remote i18n(`sidebar.*`) | 硬编码项目页标签 | 同上 |

**结论:aurora 后端引擎与 schema 已基本中立;写死业务数据的只有前端 PAGE_LABELS(含 i18n)与后端 adopt 迁移两处。**

## 方案

### 1. 后端:让项目 host 迁移生效(两套分离的迁移)

aurora 的 `RunMigrations` 改为跑**两套有序迁移**,各用各的版本表(机制详见 [`doc/features/migration.md`](../features/migration.md)):

- **schema 套**:aurora 模块自带的建表 / 改表结构迁移,放模块的 `migrations-schema/`,用版本表 `{prefix}schema_goose_db_version`,**先跑**;
- **data 套**:项目自己的业务数据迁移,放 host 目录(homeserver 为 `services/user/migrations/`),用版本表 `{prefix}goose_db_version`,**后跑**。

镜像构建时两个目录都打进去(schema → `migrations-schema/`,host → `migrations/`)。**两套分开成两张版本表,不是共用一张**:aurora 模块以后还会调整自己的表结构,两套各自独立编号才能互不干扰——项目的业务迁移号永远不会和模块的 schema 迁移号相撞,模块加新结构迁移也不逼项目改号。host 目录只放项目自己新增的,不保留 aurora 迁移的拷贝。

### 2. 后端:adopt 迁移搬回 host

`user_adopt_admin_data` 从 aurora 删除,内容移到对应项目的 host 迁移。它已在各环境执行过,删文件对已执行环境无影响;新项目天然不带它。

### 3. 前端:PAGE_LABELS 改为数据驱动

编辑页不再用写死的页面清单,改为调用 `GET /feature`,筛出 `ui.page.*` 的 feature、按其 module 去重,得到「本项目实际有哪些页」,据此渲染。页标签来自项目的 i18n(编辑页与宿主共享翻译服务,解析 `sidebar.<module>`),或在 feature 行上带标签字段由项目提供。删除 aurora remote i18n 中的项目页标签。

## Alternatives Considered

| 备选做法 | 为什么没选 |
| --- | --- |
| 把项目权限迁移也放进 aurora 模块迁移目录 | 能生效,但让共享模块带上各项目的业务名,不中立;其它项目会拉到无关迁移。 |
| 保留 PAGE_LABELS 硬编码,每加一页给 aurora 提一个 PR | 维护成本高、天然不中立;逼着每个项目的页改动都经过 aurora。 |
| 前端页面清单由宿主通过依赖注入传入(而非从后端 feature 目录派生) | 可行,但清单仍需前端手工维护一份;从 `GET /feature` 派生以数据库为单一来源,更省、更不易漂移。 |

## 落地顺序 / 影响 / 回滚

顺序(风险从小到大):

1. **消费项目本仓**(以 homeserver 为例):让 host 迁移生效(改镜像打包)+ 去重 + adopt 搬入本仓。做完该项目的权限迁移即生效,零 aurora 改动。
2. **aurora 前端**:PAGE_LABELS 改数据驱动 + 删项目 i18n。影响 4 个项目,需在各宿主验证登录与编辑页回归。
3. **aurora 后端**:从 aurora 删 adopt 迁移。

**对已部署环境**:项目 host 迁移是新版本号,各环境首次部署视为未执行、自动补跑;迁移为幂等 upsert,与运维手动补过的 SQL 叠加安全。adopt 搬走不影响已执行过它的环境。

**回滚**:均为数据 / 构建层改动,回退对应镜像即可;删或搬迁移不改表结构。

## 开放问题

- ~~迁移版本号:给项目迁移约定一个与 aurora 不重叠的编号区间,还是依赖时间戳自然不撞。~~ 已定:schema 套与 data 套分成两张版本表,各自独立编号,从机制上不会相撞,无需约定区间(见方案 ①)。
- 前端页标签来源:走宿主 i18n(零 schema),还是在 feature 行加标签字段(自带标签、不依赖宿主 i18n)。
- adopt 从 aurora 物理删除的时机:是否等所有消费项目都确认已执行后再删。

## 落地后

本提议定稿后,应更新 `user-module-migration` skill:补「host 迁移如何打包生效」、新增「前端页面清单数据驱动」,使接入者不再把迁移写进不生效的目录。不新建 skill。
