package controlgate

import (
	"context"
	"crypto/ed25519"
	crand "crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/rand/v2"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/shyandsy/aurora/logger"
	"github.com/shyandsy/sealkit/guard"
)

// verdict 是 control 签名裁决的明文体。策略是**结构化、版本化、命名空间化**的 policy
// (不再是顶层三标量)。命名空间 = 消费方分型:lease 通用租约 / http 应用专属。
// 只读认识的命名空间、json.Unmarshal 忽略其余(control 加键/命名空间对本消费方零改动)。
//
// 本 feature 先**自包含**这份 verdict 契约(与 homeserver common/controlgate 一致);把契约下沉到
// sealkit(与 guard.RenewVerdict 合流)是后续阶段,不在本 PR。crypto/lease 传输仍 delegate 给 sealkit/guard。
type verdict struct {
	Nonce           string          `json:"nonce"`
	DeploymentUuid  string          `json:"deploymentUuid"` // 必须 == 自身部署 UUID,防错配/跨部署 relay
	Now             int64           `json:"now"`
	AuthorizedUntil int64           `json:"authorizedUntil"`
	Revoked         bool            `json:"revoked"`
	Policy          policy          `json:"policy"`
	NtsServers      []ntsServerWire `json:"ntsServers"` // control 随授权下发的 NTS 时间源名单(追加字段,老 control 不发→nil,前向兼容)
}

// ntsServerWire 镜像 control 的 wire 契约(字段名已冻结)。
// pin = leaf 证书 SPKI 的 SHA-256,格式 "sha256/<base64>",与 control 侧计算口径一致:
//
//	"sha256/" + base64.StdEncoding.EncodeToString(sha256.Sum256(leafCert.RawSubjectPublicKeyInfo))
type ntsServerWire struct {
	Host string `json:"host"`
	Port int    `json:"port"` // NTS-KE 端口,默认 4460
	Pin  string `json:"pin"`  // SPKI 指纹,格式 "sha256/<base64>"
}

type policy struct {
	V     int         `json:"v"`     // 策略结构版本
	Lease policyLease `json:"lease"` // 通用租约语义(过期宽限)
	HTTP  policyHTTP  `json:"http"`  // HTTP 应用专属动作
}

type policyLease struct {
	AllowExpiredUse bool `json:"allowExpiredUse"` // 过期后是否允许继续使用(false=过期即停)
	MaxGraceDays    int  `json:"maxGraceDays"`    // 允许续用时过期后最多再用多少天(上限)
}

type policyHTTP struct {
	ErrorRate int `json:"errorRate"` // 异常触发概率 0-100%(HTTP 应用随机注入错误)
}

// defaultStartupGrace 启动期时间门的默认窗口:门禁已配置但**尚无有效授权**(never-seen / 重启 /
// 删 state)时,给这么短的窗口一边服务一边轮询**认证时间**(control verdict / NTS)。
// 首次握手是秒级,故默认砍到 2min(旧 30min 过度):攻击者要维持白嫖就得每 2min 重启一次,噪声大、
// 监控一眼看穿。可经 Config.StartupGrace(CONTROL_STARTUP_GRACE)调,别砍到 1min 以下(给慢冷启动余量)。
//
// ⚠️ 这个短窗口**只在没配 NTS 源时**才是"无条件放行":配了 NTS 源(control 下发过非空名单)时,
// never-seen/重启也必须先拿到 fresh 认证时间才服务(见 faultRate 的 air-gap 门),拿不到即刻 fail-close。
const defaultStartupGrace = 2 * time.Minute

type gate struct {
	name         string
	debug        bool          // 是否输出门禁调试/状态日志(eng=true;prd 由消费方 build 时收敛)
	startupGrace time.Duration // 启动期时间门窗口(来自 Config;0 由构造方回落 defaultStartupGrace)
	clock        *trustedClock // 收口的可信时间(锚点/单调/floor/新鲜度),见 trustedclock.go
	sources      []timeSource  // control 之外的可信时间源(NTS…),control 续约失败时续时钟
	nts          *ntsSource    // NTS 源实例(也在 sources 里);control 下发名单时 apply/applyLoaded 灌进它。可空
	metrics      *gateMetrics  // 观测指标(命名空间可配,见 metrics.go)。可空(未启用观测时)

	exempt map[string]struct{} // 放行路径集合(默认基础设施端点 + Config.ExemptPaths 追加);构造后只读

	// control 认证通道坐标(与 renewer 同一套身份),供 SignViaControl 复用向 control 发代签请求。
	// 构造后只读;门禁关闭(disabledGate)时不走 gate,故这些恒非空(见 feature.Setup)。
	controlBase  string // = RenewBaseURL
	projectUUID  string
	deploymentID string

	mu              sync.RWMutex
	seen            bool // 是否拿到过授权决定(续约成功,或从持久化恢复过)
	authorizedUntil int64
	revoked         bool
	allowExpired    bool
	maxGraceDays    int
	errorRate       int
}

// graceWindow 取本 gate 的启动期时间门窗口(未设 → 默认)。
func (g *gate) graceWindow() time.Duration {
	if g.startupGrace > 0 {
		return g.startupGrace
	}
	return defaultStartupGrace
}

// apply 应用一张新鲜续约裁决:更新授权状态 + 把权威 now 喂给时钟(新鲜样本)。
func (g *gate) apply(v verdict) {
	g.mu.Lock()
	g.seen = true
	g.authorizedUntil = v.AuthorizedUntil
	g.revoked = v.Revoked
	g.allowExpired = v.Policy.Lease.AllowExpiredUse
	g.maxGraceDays = v.Policy.Lease.MaxGraceDays
	g.errorRate = v.Policy.HTTP.ErrorRate
	g.mu.Unlock()
	g.clock.observe(v.Now) // 新鲜权威时间样本
	if g.nts != nil {
		g.nts.SetServers(v.NtsServers) // 把 control 下发的 NTS 名单灌进源(空则关掉 air-gap,防误锁)
	}
}

// applyLoaded 从本地持久化恢复裁决(重启时):恢复授权状态,但**只给时钟设不可回拨下界(floor),
// 不算新鲜样本** —— 持久化来自过去会话,不能凭它自证"现在几点"。启动后由 control / NTS 续约拿到
// 新鲜样本才 fresh=true。**配了 NTS 源却拿不到 fresh → 立刻 fail-close**(见 faultRate 的认证时间门,
// 与 never-seen 同等对待);没配 NTS 源时靠单调/floor 判过期,授权没过期照常服务(不因 control 短挂误锁)。
func (g *gate) applyLoaded(v verdict) {
	g.mu.Lock()
	g.seen = true
	g.authorizedUntil = v.AuthorizedUntil
	g.revoked = v.Revoked
	g.allowExpired = v.Policy.Lease.AllowExpiredUse
	g.maxGraceDays = v.Policy.Lease.MaxGraceDays
	g.errorRate = v.Policy.HTTP.ErrorRate
	g.mu.Unlock()
	g.clock.seedFloor(v.Now) // 只设签名下界(不可回拨),不新鲜
	if g.nts != nil {
		g.nts.SetServers(v.NtsServers) // 持久化恢复路径也灌名单:重启后 pollForTime 能直接退到 NTS 续时钟
	}
}

// FaultRate 返回本次请求注入 500 的概率(0-100)。它是公开 Gate 契约的实现(见 feature.go)。
func (g *gate) FaultRate() int { return g.faultRate() }

// ServingState 返回当前服务状态(serving/shedding/tripped/isolated),供健康/状态页读授权状态。
func (g *gate) ServingState() ServingState {
	_, state, _ := g.snapshot()
	return state
}

// AuthorizedUntil 返回当前授权到期 unix 秒;0 = 从未授权 / pending / 门禁关闭。
func (g *gate) AuthorizedUntil() int64 {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.authorizedUntil
}

// faultRate 执行 control 下发策略,返回本次请求注入 500 的概率(0-100)。
//
// 判定顺序(统一模型:never-seen 与 loaded 同等对待,都受「认证时间门」约束):
//  1. revoked → 100(吊销 kill switch,最高优先级);
//  2. **认证时间门 / air-gap**:配了可试时间源(control 下发过非空 NTS 名单 → Available)却拿不到 fresh
//     认证时间 → 100。对 never-seen / loaded / 已授权**一视同仁**:删 state 走 never-seen 也逃不掉;
//  3. 已有授权决定(seen && au>0)→ 走正常 verdict 语义(有效/过期即停/宽限内 errorRate);
//     seen 但 au==0(control 明说 pending/未授权)→ 100(不服务,防被 de-authorize 的在线镜像分段续命);
//  4. **启动期时间门**(never-seen:重启 / 删 state / 首次握手):到这说明没配 NTS 源(否则第 2 步已拦)
//     或已拿到 fresh 但还没等到首个 verdict → 短 grace 窗口内放行、超窗口 fail-close。
func (g *gate) faultRate() int {
	g.mu.RLock()
	seen := g.seen
	revoked := g.revoked
	au := g.authorizedUntil
	allowExpired := g.allowExpired
	maxGraceDays := g.maxGraceDays
	errorRate := g.errorRate
	g.mu.RUnlock()

	now, fresh := g.clock.now()

	if revoked {
		return 100 // 吊销 = 立即停(最高优先级)
	}
	// 认证时间门(air-gap):配了可试时间源却拿不到 fresh 认证时间 = 气隙/伪造 → fail-close。
	// 对 never-seen 也生效:删 state 后走 never-seen 仍要 fresh,堵掉"删 state → 无条件白嫖"。
	// 名单为空(没配 NTS / 首个裁决之前)不启用——没有独立认证源就无法区分"气隙" vs "control 短挂",
	// 只能退到下面的短 grace。
	if g.hasAvailableSource() && !fresh {
		return 100
	}
	if seen && au > 0 {
		// 已有授权决定:正常 verdict 语义(离线也按单调/floor 判时间)。
		if now <= au {
			return 0 // 授权有效
		}
		if !allowExpired {
			return 100 // 过期即停
		}
		if now > au+int64(maxGraceDays)*86400 {
			return 100 // 超宽限上限
		}
		return errorRate // 宽限期内:按 control 下发的 errorRate 随机 500
	}
	if seen { // au == 0:control 明确 pending / 未授权 → 不服务
		return 100
	}
	// never-seen:启动期时间门(短 grace 内一边服务一边轮询认证时间;超窗口 fail-close)。
	if g.clock.sinceStart() < g.graceWindow() {
		return 0
	}
	return 100
}

// snapshot 从当前门禁状态导出三个观测量(与 faultRate 同一套判定):
// shed = 丢弃比例(0-100)、state = 服务状态、ttl = 距下一次状态跃迁的剩余秒数。
func (g *gate) snapshot() (shed float64, state ServingState, ttl float64) {
	g.mu.RLock()
	seen := g.seen
	revoked := g.revoked
	au := g.authorizedUntil
	allowExpired := g.allowExpired
	maxGraceDays := g.maxGraceDays
	errorRate := g.errorRate
	g.mu.RUnlock()

	now, fresh := g.clock.now()

	if revoked {
		return 100, Isolated, 0 // isolated(已吊销)
	}
	// 认证时间门 / air-gap:有可试时间源却拿不到 fresh(对 never-seen/loaded/已授权一视同仁)→ 判被隔离。
	if g.hasAvailableSource() && !fresh {
		return 100, Isolated, 0
	}
	if seen && au > 0 {
		if now <= au {
			return 0, Serving, float64(au - now) // serving,ttl=距过期
		}
		if !allowExpired {
			return 100, Tripped, 0 // tripped(过期即停)
		}
		graceEnd := au + int64(maxGraceDays)*86400
		if now > graceEnd {
			return 100, Tripped, 0 // tripped(超宽限)
		}
		return float64(errorRate), Shedding, float64(graceEnd - now) // shedding,ttl=距彻底停
	}
	if seen { // au == 0:pending / 未授权
		return 100, Tripped, 0
	}
	// never-seen:启动期时间门。
	if grace := g.graceWindow(); g.clock.sinceStart() < grace {
		return 0, Serving, grace.Seconds() - g.clock.sinceStart().Seconds() // serving(启动窗口内)
	}
	return 100, Tripped, 0 // tripped(超启动窗口仍无授权)
}

// exportMetrics 每 30s 把门禁状态刷进 gauge(独立于续约间隔,让状态/倒计时在大盘上及时)。
// metrics 为空(未启用观测)时直接返回。
func (g *gate) exportMetrics(ctx context.Context) {
	if g.metrics == nil {
		return
	}
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		shed, state, ttl := g.snapshot()
		g.metrics.set(g.name, shed, float64(state), ttl)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// run 后台续约循环(复用 sealkit 的 HTTP 传输,验签+解析+执行策略在本包)。
// statePath != "" 时,每次续约成功把签名裁决原文落盘,供重启恢复(见 loadPersisted)。
func (g *gate) run(ctx context.Context, base, proj, dep string, pub ed25519.PublicKey, every time.Duration, statePath string, sealK []byte) {
	renew := guard.HTTPRenewer(base, proj, dep) // 只借它的传输;验签/策略本包做
	for {
		nonce, err := newNonce()
		gotControl := false
		if err == nil {
			blob, rerr := renew(ctx, nonce)
			switch {
			case rerr != nil:
				if g.debug {
					logger.Debugf("controlgate[%s]: 续约请求失败(离线宽限期内仍按上次裁决): %v", g.name, rerr)
				}
			default:
				if v, verr := verifyParse(blob, pub, nonce, dep); verr != nil {
					if g.debug {
						logger.Debugf("controlgate[%s]: 裁决验签/解析失败: %v", g.name, verr)
					}
				} else {
					g.apply(v)                            // 含 clock.observe(v.Now):新鲜权威时间
					savePersisted(statePath, sealK, blob) // 续约成功:加密落盘签名裁决,供重启恢复(best-effort)
					gotControl = true
					if g.debug {
						logger.Debugf("controlgate[%s]: 续约成功 authorizedUntil=%d revoked=%v policy.v=%d errorRate=%d%% allowExpired=%v maxGraceDays=%d ntsServers=%d → 当前 faultRate=%d%%",
							g.name, v.AuthorizedUntil, v.Revoked, v.Policy.V, v.Policy.HTTP.ErrorRate, v.Policy.Lease.AllowExpiredUse, v.Policy.Lease.MaxGraceDays, len(v.NtsServers), g.faultRate())
					}
				}
			}
		}
		// control 没拿到新鲜裁决 → 退到签名时间源(NTS…)续时钟:拿不到授权,但保住 fresh、
		// 让"离线但授权有效"能正常判过期,也避免重启后无谓 fail-closed。全部源都失败即不 fresh。
		if !gotControl {
			g.pollForTime(ctx)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(every):
		}
	}
}

// parseVerdict 验证 {b:base64(裁决),s:base64(签名)} 信封:项目公钥验签 + 解析 + deploymentUuid 校验
// (**不校验 nonce**)。续约路径用 verifyParse(额外校 nonce 防重放);从本地持久化恢复走本函数——
// 持久化本就是"复用旧裁决",无 nonce 语义,其安全性靠:签名不可伪造 + trustedNow 不可回拨 +
// deploymentUuid 不可跨部署 relay + 加载时按墙钟拒绝彻底过期者(见 loadPersisted)。
func parseVerdict(blob []byte, pub ed25519.PublicKey, expectDep string) (verdict, error) {
	var w struct {
		B []byte `json:"b"`
		S []byte `json:"s"`
	}
	if err := json.Unmarshal(blob, &w); err != nil {
		return verdict{}, err
	}
	if len(pub) != ed25519.PublicKeySize || !ed25519.Verify(pub, w.B, w.S) {
		return verdict{}, errors.New("签名验证失败")
	}
	var v verdict
	if err := json.Unmarshal(w.B, &v); err != nil {
		return verdict{}, err
	}
	// 防跨部署 relay:eng/prd 共用同一项目公钥,不校验 deploymentUuid 的话别的部署的裁决能被冒用。
	if v.DeploymentUuid != expectDep {
		return verdict{}, errors.New("deploymentUuid 不符(错配/跨部署 relay)")
	}
	return v, nil
}

// verifyParse 续约路径:parseVerdict + nonce 回显防重放(防甲方 replay 旧续约响应)。
func verifyParse(blob []byte, pub ed25519.PublicKey, nonce, expectDep string) (verdict, error) {
	v, err := parseVerdict(blob, pub, expectDep)
	if err != nil {
		return verdict{}, err
	}
	if v.Nonce == "" || v.Nonce != nonce {
		return verdict{}, errors.New("nonce 不符(重放/过期响应)")
	}
	return v, nil
}

// loadPersisted 启动时从本地加载上次的**签名裁决**(验签 + deploymentUuid),并据墙钟判定是否还"活着"
// (未超 authorizedUntil + 宽限上限)。活着才恢复,避免"重启时正好 deploy 抖动"被误锁;彻底过期的直接不认
// (甲方存旧裁决刷新有效期无效)。best-effort:文件缺失/损坏/过期/未启用均静默返回 false → 走 never-seen 启动窗口。
func loadPersisted(path string, key []byte, pub ed25519.PublicKey, expectDep string) (verdict, bool) {
	if path == "" {
		return verdict{}, false
	}
	enc, err := os.ReadFile(path)
	if err != nil {
		return verdict{}, false
	}
	blob, err := sealDecrypt(key, enc)
	if err != nil {
		return verdict{}, false // 解密失败(被改 / 换 key / 非本格式)→ 当无有效持久化,走启动宽限
	}
	v, err := parseVerdict(blob, pub, expectDep)
	if err != nil {
		return verdict{}, false
	}
	if !persistedAlive(v) {
		return verdict{}, false // 彻底过期(墙钟判定),不恢复
	}
	return v, true
}

// persistedAlive 据墙钟判定持久化裁决是否还"活着"(未超 authorizedUntil + 宽限上限)。
// 彻底过期者不恢复 → 甲方"存旧裁决、等它过期、重启刷新有效期"无效。
func persistedAlive(v verdict) bool {
	deadline := v.AuthorizedUntil
	if v.Policy.Lease.AllowExpiredUse {
		deadline += int64(v.Policy.Lease.MaxGraceDays) * 86400
	}
	return time.Now().Unix() <= deadline
}

// savePersisted 把当前有效的签名裁决落本地(供重启恢复),**加密落盘**(混淆:让磁盘上看不出是授权令牌)。
// best-effort,写失败静默(顶多退化成 never-seen)。
// 安全仍靠签名:加载时先解密再验签,甲方改不了(伪造不出签名)、只能删(删=回到 fail-closed);
// 加密只是遮内容(见 seal.go 的"混淆非保密"说明)。
func savePersisted(path string, key, blob []byte) {
	if path == "" {
		return
	}
	enc, err := sealEncrypt(key, blob)
	if err != nil {
		return // best-effort
	}
	_ = os.WriteFile(path, enc, 0o600)
}

func newNonce() (string, error) {
	b := make([]byte, 16)
	if _, err := crand.Read(b); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// defaultExemptPaths 恒放行的基础设施端点,降级绝不能碰它们:
//   - k8s 健康/就绪探针(/health /ready /healthz /readyz /livez)—— 碰了会 500 探针 → pod crashloop;
//   - /metrics —— 碰了我们自己的观测就瞎了。
//
// 目的是「业务降级、pod 存活」,不是把 pod 杀掉。不同服务的探针路径可能不同,
// 消费方经 Config.ExemptPaths(CONTROL_EXEMPT_PATHS,逗号分隔)**追加**(不替换默认)。
var defaultExemptPaths = []string{"/health", "/ready", "/healthz", "/readyz", "/livez", "/metrics"}

// normPath 归一化路径用于 exempt 比对:去掉尾部 "/"(容忍 "/metrics" vs "/metrics/"),但保留根 "/"。
// 只做尾斜杠容忍,**不做前缀/子路径匹配**:子路径(如 /metrics/cadvisor)必须逐条精确列全,否则不命中。
func normPath(p string) string {
	for len(p) > 1 && strings.HasSuffix(p, "/") {
		p = p[:len(p)-1]
	}
	return p
}

// newExemptSet 建「默认基础设施端点 + 追加项」的放行集合(空白项忽略;键按 normPath 归一)。
func newExemptSet(extra []string) map[string]struct{} {
	set := make(map[string]struct{}, len(defaultExemptPaths)+len(extra))
	for _, p := range defaultExemptPaths {
		set[normPath(p)] = struct{}{}
	}
	for _, p := range extra {
		if p = strings.TrimSpace(p); p != "" {
			set[normPath(p)] = struct{}{}
		}
	}
	return set
}

// exemptPath 该路径是否放行(默认基础设施端点 + 配置追加;尾斜杠容忍,子路径不前缀命中)。
// exempt 为空时退回仅默认集合。
func (g *gate) exemptPath(p string) bool {
	p = normPath(p)
	if g.exempt == nil {
		for _, dp := range defaultExemptPaths {
			if normPath(dp) == p {
				return true
			}
		}
		return false
	}
	_, ok := g.exempt[p]
	return ok
}

// middleware 执行策略 —— 按 faultRate 随机注入 500。平常(授权有效)faultRate=0,零影响。
// 门禁**永远强制**(不设运行时开关,免得甲方 env 一翻就关掉整个闸)。边界靠:
//   - 启动期时间门(never-seen 短 grace 内放行,配了 NTS 源则须先拿 fresh):首次握手/deploy 短抖动
//     放行不 brick 新部署;超窗口或(有 NTS 源却)拿不到 fresh 认证时间 → fail-close;
//   - exemptPath 放行健康探针/就绪/指标:否则降级会 500 掉 k8s 探针 → pod crashloop。
//
// 要的是「业务降级、pod 仍存活」(更隐蔽、不破坏 k8s),不是杀 pod。
func (g *gate) middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !g.exemptPath(c.Request.URL.Path) {
			if r := g.faultRate(); r > 0 && rand.IntN(100) < r {
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
		}
		c.Next()
	}
}
