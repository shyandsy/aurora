# controlgate — 可信授权门禁 + control 认证通道(aurora feature)

一个 aurora feature,握住服务实例到 control 的**认证通道**(RenewBaseURL + Project/Deployment 身份 + 授权),
在其上提供能力:

1. **serving 门**(本 feature 主体):回连 control 做续期授权 → 按下发策略降级(随机注入 500 / 过期宽限 /
   吊销立即停),配 NTS 可信时钟 + 启动期时间门。
2. **代签 `SignViaControl`**:请 control 用它持有的项目私钥签一段字节、返回签名——**key 永在 control,
   本 feature 不持不缓存、只转发**(见「代签 SignViaControl」;Phase 1 分发锁的客户端半)。
3. (将来)取材料:同一通道现领节点凭据 / 在线材料(「在线材料依赖」阶段,不在本 PR)。

它**包住(wrap)`github.com/shyandsy/sealkit/guard`,不吞并它**:借 guard 的续约 HTTP 传输(`HTTPRenewer`)
与 ed25519 公钥解码(`DecodePub`),验签裁决 + 执行策略在本包做。依赖方向**单向** `controlgate → sealkit`,
永不反过来;**绝不把 sealkit/guard 搬进 aurora**——它是中立可移植库(转发节点 / C 客户端也在用)。

## 分工

| 角色 | 职责 |
|---|---|
| control(homeserver-deploy) | 策略定义 + 项目私钥签名(随续约裁决下发) |
| `sealkit/guard` | 续约 HTTP 传输(`HTTPRenewer`)+ 公钥解码(`DecodePub`) |
| 本 feature | ① serving 门:验签裁决 → 随机注入 500 / 过期宽限 / 吊销立即停 + NTS 可信时钟 + 启动宽限 + 持久化恢复;② `SignViaControl`:转发关键数据到 control 现签(key 不落本地) |

## 集成(opt-in feature)

不在 bootstrap 默认集——只有 prd 要接 control 授权的服务显式 `AddFeature`。

> **注册顺序铁律:必须在 server feature 之后、且在 `RegisterRoutes` 之前。**
> controlgate 在 `Setup` 里 `Resolve` 已被 server feature `Provide` 的 `*gin.Engine` 来挂中间件——
> gin 中间件只对**之后**注册的路由生效。故:
> - 在 server 之后(否则解析不到 `*gin.Engine` → `Setup` 报错);
> - 在 `RegisterRoutes` 之前(否则已注册的路由会**静默绕过门禁**;`Setup` 有自守卫,检测到 engine 已
>   有路由即 fail-startup,逼你把 `AddFeature(controlgate)` 提前)。
>
> 标准用法 `AddFeature×N → RegisterRoutes → Run` 天然满足,这条只是防误用护栏。

```go
// service providers（在 NewServerFeature 之后）
var cg controlgate.Config
_ = config.ResolveConfig(&cg)             // 灌 CONTROL_* 坐标(见下表)
cg.ServiceName = "customer"               // 日志 / metrics 的 service label
cg.MetricNamespace = "homeserver"         // homeserver 现网须设(留空=不启用观测、不报错;见「观测」)
cg.DebugLogs = app.RunLevel() == "eng"    // eng 大声、prd 收敛

app.AddFeature(controlgate.NewControlgateFeature(cg))

// 需要读授权状态的服务(健康页 / 状态页),注入即用:
type statusHandler struct {
    Gate controlgate.Gate `inject:""`
}
```

`Setup` 会 `ProvideAs` 一个 `Gate` 并把 gin 中间件自动挂到 `*gin.Engine`(等价旧版 `Install(app, name)`)。

## 配置(`Config` / `CONTROL_*` env)

坐标一律**注入**(不再 build-time baked——aurora 是库,没有烘焙值)。env 名与现网冻结的一致,勿缩写。

| 字段 | env | 必填 | 说明 |
|---|---|---|---|
| `RenewBaseURL` | `CONTROL_RENEW_BASE_URL` | 坐标组 | 如 `https://control-eng.deploy789.com` |
| `ProjectUUID` | `CONTROL_PROJECT_UUID` | 坐标组 | 项目标识 |
| `DeploymentID` | `CONTROL_DEPLOYMENT_ID` | 坐标组 | 部署标识(防跨部署 relay,验签时校) |
| `ProjectPubKey` | `CONTROL_PROJECT_PUBKEY` | 坐标组 | base64 ed25519 项目公钥(**只验签,非私钥**) |
| `RenewEvery` | `CONTROL_RENEW_EVERY` | 否 | 续约间隔,默认 60s(eng 联调可调短) |
| `StateDir` | `CONTROL_STATE_DIR` | 否 | 持久化目录(见「持久化」) |
| `StartupGrace` | `CONTROL_STARTUP_GRACE` | 否 | 启动期时间门窗口,默认 **2min**(见「启动期时间门」);别 < 1min |
| `ExemptPaths` | `CONTROL_EXEMPT_PATHS` | 否 | 追加放行路径,逗号分隔(见「降级与放行」) |
| `MetricNamespace` | — | 否 | prometheus 指标前缀;**留空 = 不启用观测(不报错)**;homeserver 现网须设 `"homeserver"`(见「观测」) |
| `DebugLogs` | — | 否 | 运行时日志开关;消费方直接设 |
| `RequireGate` | — | 否 | 逃生阀:true = **任何 RunLevel** 下坐标全空都 fail-startup(见下) |
| `Registerer` | — | 否 | prometheus registerer,空=DefaultRegisterer(测试可注入独立 registry) |

**坐标组(前 4 项)语义**:

- **齐全** → 门禁启用。
- **部分缺失** → `Setup` 直接报错(fail-startup,暴露误配,绝不静默半开)。
- **全空** → 分 `RunLevel` 处置(防 prd 配置回归静默裸奔):
  - `production`(或 `RequireGate=true`,任何 level)→ **fail-startup**:prd 坐标全空视为误配
    (`CONTROL_*` secret 挂坏 / 漏 env),拒绝带病启动。
  - `local` / `eng` / `stage` → 门禁关闭(开发便利,不 brick),`ProvideAs` 一个 `disabledGate`。
    它 `ServingState()` 返回 **`Disabled`(不是 `Serving`)**,让状态页 / 告警能看见「闸关了」,不被伪装成健康。

## 安全模型:时间门这层(软骚扰;每条一位不丢)

> ⚠️ 本节是**时间门 / 降级**这层——抬逆向与离线续命的成本、可远程吊销、可发现克隆,但它终究是
> 二进制里的判断,**光靠它不足以防搬走**(root+AI 能 patch)。真正的锁是下面的
> **[SignViaControl](#signviacontrol请-control-代签关键数据controlgate-的真安全牙齿)**(关键数据签发离不开 control)。
> 以下每条实现一位不丢:

- **fail-closed**:超启动期时间门无有效授权 / pending(`authorizedUntil==0`)/ 吊销 / 过期不允许续用 /
  超宽限上限 → 全停。**never-seen 与 loaded 同等对待**(统一模型,无特例)。
- **NTS 可信时钟 / 认证时间门**:pin-only TLS(**不信系统 CA**,甲方是 root 能污染信任库)取认证时间;
  签名裁决的 `now` 作不可回拨下界(floor)+ 本地单调往前数,防墙钟回拨。**认证时间门(air-gap)**:配了
  可试 NTS 源(control 下发过非空名单)却拿不到 fresh 认证时间 → 判气隙/伪造 → 停;**对 never-seen /
  loaded / 已授权一视同仁**(删 state 走 never-seen 也逃不掉);名单为空则不启用(防误锁拿不到源的合法机器)。
- **启动期时间门**:见下节。
- **降级(处决动作)**:非 exempt 路径按 `faultRate`(0-100)随机注入 500,**不 `os.Exit`、不杀 pod**——
  要的是「业务降级、pod 存活」(更隐蔽、不破坏 k8s)。

## 启动期时间门(bootstrap time gate)

never-seen(全新部署 / 重启 / **删了 state**)时,门禁进入启动期:**短 grace 窗口内一边服务一边轮询认证
时间**(control verdict / NTS)。窗口由 `CONTROL_STARTUP_GRACE` 配,**默认 2min**(首次握手秒级,旧 30min
过度;砍短让偷跑者要每 2min 重启一次,噪声大、监控看穿)。

判定分两种:

- **配了 NTS 源**(control 下发过非空名单):never-seen 也**必须先拿到 fresh 认证时间**才继续服务——
  拿不到(气隙 / 伪造)即刻 fail-close,**不吃 grace 豁免**。**删 state 走 never-seen 同样受此约束**,不再有
  旧版"删 state → 30min 无条件白嫖"的洞。拿到 fresh 后按 verdict 正常判定。
- **没配 NTS 源**:无独立认证时间源,无法区分"气隙偷跑" vs "合法冷启动",只能靠这个**短 grace 窗口**——
  窗口内放行、超窗口 fail-close。这是已知窄残留(见「已知边界」),故 **prd 必须下发 NTS 名单**。

> **净效果**:合法新部署秒级拿到 fresh/verdict → 平滑;偷跑镜像(配了 NTS 却拿不到 fresh)→ 连短 grace 都
> 不给、直接 fail-close;删 state 无用(never-seen 也要 fresh)。**旧版 loaded-stale 特例已删**,never-seen
> 与 loaded 同一套判定,不再有可用性不对称。

## 降级与放行(ExemptPaths)

以下**基础设施端点恒放行**(降级绝不注入 500,否则 500 掉 k8s 探针 → crashloop、或打瞎 `/metrics`):

    /health  /ready  /healthz  /readyz  /livez  /metrics

不同服务探针路径不同,`Config.ExemptPaths`(`CONTROL_EXEMPT_PATHS`,逗号分隔)在其上**追加**,只增不减。

**匹配规则**:精确匹配 + 尾斜杠容忍(`/metrics` 与 `/metrics/` 等价),**不做前缀/子路径匹配**。
所以 `ExemptPaths` **必须逐条精确列全**每个探针 / 指标路径——子路径(如 `/metrics/cadvisor`)**不会**被
`/metrics` 前缀命中;漏列的路径会在降级时被注入 500 → 探针 crashloop。

## 持久化(StateDir,可选)

配了 `StateDir` → 每次续约成功把**签名裁决**加密落盘(`<StateDir>/<ServiceName>.cache`),重启时先加载
恢复(不退回 never-seen,避免「重启时正好 deploy 抖动」被误锁)。加密是**混淆非保密**(甲方 root 能
dump);安全仍靠签名不可伪造 + `now` 不可回拨 + deploymentUuid 不可跨部署 relay + 加载时按墙钟拒绝彻底
过期者。k8s 里挂个可写卷即可;不配 → 退化为纯启动宽限(仍 fail-closed,只是重启不保留剩余授权)。

## 观测(metrics · 中性伪装)

三个 gauge(namespace 由 `Config.MetricNamespace` 决定),label `service`:

    <ns>_http_shed_ratio           0-100 当前丢弃比例(= faultRate)
    <ns>_http_serving_state        0=serving 1=shedding 2=tripped 3=isolated
    <ns>_http_serving_ttl_seconds  距下一次状态跃迁剩余秒数

看 `/metrics` 看不出授权语义,可读性只落在 Grafana 的 value-mapping。

> **`MetricNamespace`:homeserver 现网必设 `"homeserver"`**(留空 = 不启用观测,**不报错**——中立库
> 不自造项目前缀,其它消费方不要 metrics 时可留空)。homeserver 现网 Grafana 依赖 `homeserver_http_*`
> 这套指标名,切换时忘设就会**指标名断**(是运维约定,不是代码强制)。

## `Gate` 契约(注入即用)

```go
type Gate interface {
    FaultRate() int                 // 本次请求注入 500 的概率(0-100);授权有效=0
    ServingState() ServingState     // serving / shedding / tripped / isolated / disabled
    AuthorizedUntil() int64         // 授权到期 unix 秒;0 = 从未授权 / pending / 门禁关闭
    // 请 control 用其持有私钥代签 payload;key 永在 control,本 feature 不持不缓存、只转发。
    // 授权失败/被吊销/后端不可达 → error(fail-closed,绝不返回伪造/占位签名)。
    SignViaControl(ctx context.Context, payload []byte) (sig []byte, err error)
}

// 0 Serving · 1 Shedding · 2 Tripped · 3 Isolated(以上进 metrics gauge)· 4 Disabled(仅 Gate 暴露,
// 门禁关闭时的状态,不进 gauge);.String() 出状态名。
type ServingState int
```

健康 / 状态页据此读授权状态(如 `ServingState()==Serving` + `AuthorizedUntil()` 展示剩余天数)。
中间件已由 feature 自动挂载,消费方无需手动 `Use`。

## SignViaControl:请 control 代签关键数据(controlgate 的真安全牙齿)

### 为什么要有它
上面的时间门(降级 / fail-close)**只是软骚扰**:它是二进制里的判断,root + AI 能定位、一 patch 就废——
**光靠它,防搬走/防劫持的安全性形同虚设**。真正挡得住的,是**让关键数据的签发离不开 control**:
签名私钥**永在 control**,消费方**只能请 control 代签**;未授权 / 被吊销的副本**签不出有效数据 → 产不出
正确输出**。这不是一个可 patch 的判断,而是一个副本**根本没有的能力**(key 不在它手里)——所以逆向
patch 不掉它。

### 如何确保安全
- **私钥永不下发**:control 持有;`SignViaControl` 只把 payload 送过去、取回签名,**本 feature 不持、不缓存、只转发**。
- **抗 capture-once**:签"新数据"这个**动作**每次都要 control;副本抓一次只拿到旧签名(对它无用),
  拿不到"以后自己签"的能力。
- **可吊销**:control 拒签某实例 → 它再也签不出。
- **fail-closed**:授权失败 / 被吊销(401/403)/ control 不可达 / 响应异常 → 返回 error,**绝不返回
  伪造或占位签名**;门禁关闭(`disabledGate`)直接 error。
- 复用 controlgate 已握的 control 认证通道(同 `RenewBaseURL` + Project/Deployment 身份 + 授权),不另造第二条。

### 使用案例:签发 App 引导配置(典型场景)
消费方(如 admin 发布 App `app.json`:API/订阅端点、注册开关)请 control 代签、组装 App 端信封
`{payload, sig}`;已装机 App 用**内置公钥**验签,验过才信。**偷来的副本没私钥、control 又拒签 →
签不出 App 认的配置 → 劫持不了你的用户群。**

```go
type publisher struct {
    Gate controlgate.Gate `inject:""`
}
sig, err := p.Gate.SignViaControl(ctx, payload)   // err 即 fail-closed:别落库、别发布
if err != nil { return err }
envelope := map[string]string{
    "payload": base64.StdEncoding.EncodeToString(payload),
    "sig":     base64.StdEncoding.EncodeToString(sig),
}
```

- **不验 sig**:那是 App 用内置公钥的事,与 controlgate 的 verdict 公钥无关;本 feature 只把 `sig` 取回原样返回。
- **wire 契约**(供 control 服务端对齐):`POST {RenewBaseURL}/projects/{proj}/deployments/{dep}/appconfig/sign`
  (与 renewer 同一身份命名空间,per-deployment 授权中间件原样适用),请求
  `{"payload": base64(bytes), "projectUuid": ..., "deploymentId": ...}`,响应 `{"sig": base64(sig)}`。
- **对端跨仓**:control 服务端(保管私钥 + 授权 + 吊销)不在本 PR;端点上线前本方法"就绪但无对端"。
  消费方**自身**的落地设计(签哪块数据、密钥托管迁移、分阶段)见其项目内文档。

## 已知边界 / prd 须知(诚实交代威胁边界)

这是安全 feature,威胁模型是「小白 + AI」,不防专业逆向。以下边界务必知情:

- **坐标全空 = 门禁静默关**:eng/local/stage 全空 → disabled 放行。**prd 务必 `RequireGate=true`(或依赖
  RunLevel=production 的默认 fail-startup)**,并**监控 `serving_state`**——否则 `CONTROL_*` secret/env 挂坏
  时会裸奔,且靠指标发现不了(disabledGate 不出 gauge,状态页看 `ServingState()==disabled`)。
- **抗回拨 / 续命边界**:
  - **配了 NTS 名单**:never-seen 续命洞**已堵**——删 state 走 never-seen 也必须拿到 fresh 认证时间才服务
    (拿不到即刻 fail-close);floor + NTS 认证时间(pin-only,root 伪造不了)撑起抗回拨。残留仅剩:攻击者
    能持续联网到 control/NTS 拿 fresh、又想离线跑——但那已不是离线续命。
  - **不下发 NTS 名单**(**强烈不建议 prd 这么配**):无独立认证时间源,只剩**短 grace 窗口**这个残留——
    root 回拨墙钟 + 还原 state,每 `< StartupGrace`(默认 2min)重启一次可维持服务(比旧 30min 小一个量级,
    噪声大、监控看穿;见 `TestNoNtsResidueShortGraceWindow`)。故 **prd 必须给 deployment 下发 NTS 名单**。
- **exempt 是精确匹配(非前缀)**:配 `/api/health` **不**覆盖其子路径;每条探针/指标路径必须逐条列全,
  配错 = 降级时 500 掉未列的探针 → pod crashloop。
- **依赖 NTS 客户端拒收未认证响应**:整条抗回拨/air-gap 防线假设 `github.com/beevik/nts` 只接受
  AES-SIV 认证过的 NTP 响应(时间在认证载荷内);pin-only TLS 只认服务器 pin、绝不回退系统 CA。
- **天花板**:controlgate 抗回拨/时间门到位,也只控"能否 serve HTTP",**不阻止复制二进制本身**。真正防复制 /
  根治「root-on-box 逆向 / 离线续命」靠后续「**在线材料依赖**」阶段(服务离了 control 现领的节点凭据就跑不了),
  设计见 homeserver `doc/design/controlgate-aurora.md`。方案「必须拿认证时间才服务」是往那个方向的第一步。
  本 feature 定位:**提高逆向 / 离线续命的成本 + 可远程吊销 + 可发现克隆**,不是绝对防死。

## 范围 · 后续阶段 · prd 构建卫生

- **本包范围**:wrap `sealkit/guard` + 在其上提供 **serving 门** 和 **`SignViaControl` 代签**(**不只是 wrap**)。
- verdict 契约本包先**自包含**(与 homeserver 一致);把它下沉 sealkit(与 `guard.RenewVerdict` 合流)、
  信封装运营材料、principal+type 是**后续阶段**,不在此。
- **prd 的 garble / `-literals` / DCE 特征卫生是消费方 build 时的事**:中立库不带 `controlprod` 之类
  build tag,debug 日志由 `Config.DebugLogs` 运行时控制。homeserver prd build 需自行对本包路径叠 scoped
  garble(抹字符串 / 函数名 / 包路径),中立库不掺和。
