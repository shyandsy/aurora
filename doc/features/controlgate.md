# controlgate — 可信授权门禁(🔌 直接用 Feature,防搬走 / 防盗用)

把「这个服务实例现在还获授权运行吗」收口成门禁:control 随续约下发**项目私钥签名的裁决**,本 feature 执行——验签 → 过期宽限(`allowExpiredUse`+`maxGraceDays`)/ 吊销立即停 / 按 `errorRate` 随机注入 500 降级,配 NTS 可信时钟(防墙钟回拨 + air-gap 判定)+ 启动宽限 + 落盘恢复。**包住 `sealkit/guard`(续约传输 + 公钥解码),不吞并它**——sealkit 是中立可移植库(节点 / C 也在用),依赖单向 `controlgate → sealkit`。

> 与「防护体系」正交:那套(`ratelimit`/`loginguard`/`doorman`/`tokenguard`)防的是**每请求滥用**;controlgate 防的是**整个部署被搬走 / 盗用**(licensing / attestation),是另一类。

## 消费方式

```go
app.AddFeature(controlgate.NewControlgateFeature(controlgate.Config{
    ServiceName:     "admin",
    MetricNamespace: "homeserver",   // homeserver 现网须设(留空=不启用观测、不报错;其它消费方可不要 metrics)
    // 坐标经 CONTROL_* env 注入(ResolveConfig),或直接填:
    // RenewBaseURL / ProjectUUID / DeploymentID / ProjectPubKey / RenewEvery / StateDir / StartupGrace / ExemptPaths
}))
// 须在 server feature 之后、且在 RegisterRoutes 之前注册(要 Resolve *gin.Engine;中间件只对之后注册
// 的路由生效,Setup 有 assertNoRoutes 自守卫,顺序反了直接 fail-startup)。中间件自动挂,无需手动 Use。
// 健康/状态页:  Gate controlgate.Gate `inject:""`  → FaultRate() / ServingState() / AuthorizedUntil() / SignViaControl()
```

对外只有 `Gate` 契约(`FaultRate`/`ServingState`/`AuthorizedUntil`/`SignViaControl`)+ `Config`;verdict/时钟/持久化等实现全部私有。

## 关键点

- **fail-close**:超启动期时间门无授权 / pending / 吊销 / 过期不续用 → 全停(faultRate=100)。与限流类(ratelimit/loginguard)的 fail-open 刻意相反。
- **坐标注入**:coords 不再 build-time baked,走 `CONTROL_*` env / Config。**部分缺 = Setup fail-startup**(暴露误配);**全空**按 RunLevel 分:`production`(或 `RequireGate=true`)→ **fail-startup**(prd 全空=误配,拒绝静默裸奔),`local/eng/stage` → 门禁关(`disabledGate`,`ServingState()=Disabled` 让状态页看见闸关了,不伪装健康)。
- **exempt 路径可配**(`CONTROL_EXEMPT_PATHS`):默认 6 个探针/指标端点(`/health /ready /healthz /readyz /livez /metrics`)恒放行——否则降级会 500 掉 k8s 探针 → pod crashloop;配置项**追加不替换**。**精确匹配 + 尾斜杠容忍,不前缀/子路径命中**——每条探针路径必须逐条列全(`/metrics/cadvisor` 不被 `/metrics` 命中)。
- **启动期时间门(bootstrap time gate)**:never-seen / 重启 / **删 state** → 短 grace 窗口(`CONTROL_STARTUP_GRACE`,默认 **2min**,旧 30min 已砍)内一边服务一边轮询认证时间。**配了 NTS 源时 never-seen 也必须先拿 fresh 认证时间才服务**(拿不到即刻 fail-close)——统一模型,never-seen 与 loaded 同等对待(旧 loaded-stale 特例已删),**删 state 白嫖洞已堵**;没配 NTS 源时只能靠短 grace 窗口(窄残留)。
- **NTS 可信时钟(抗过期承重墙)**:防墙钟回拨;control 下发名单后启用认证时间门 / air-gap 判定(有可试时间源却拿不到 fresh → 判被气隙攻击 → Isolated,对 never-seen/loaded 一视同仁)。**prd 必须给 deployment 下发 NTS 名单**;不下发时残留仅剩"短 grace 窗口"(root 回拨墙钟 + 还原 state 每 <2min 重启续命),root-on-box 固有边界,真正根治靠后续「在线材料依赖」阶段。
- **代签 `SignViaControl`(Phase 1 分发锁客户端半)**:复用同一 control 认证通道,请 control 用其持有的项目私钥代签一段字节(如 admin 发布 app.json)——**key 永在 control,本 feature 不持不缓存、只转发**;授权失败/吊销/不可达 → error(fail-closed,绝不占位)。wire:`POST {RenewBaseURL}/projects/{proj}/deployments/{dep}/appconfig/sign` `{payload,projectUuid,deploymentId}` → `{sig}`。control 服务端 `/appconfig/sign` 跨仓、不在本 PR。
- **prd 特征卫生**(garble / `-literals` / DCE):是**消费方 build 时**的事,中立库不带 build tag。

详见 `feature/controlgate/README.md`;设计与后续阶段(信封装运营材料 / principal+type / 契约下沉 sealkit)见 homeserver `doc/design/controlgate-aurora.md`。
