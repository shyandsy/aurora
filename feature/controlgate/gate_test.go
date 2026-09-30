package controlgate

import (
	"context"
	"crypto/ed25519"
	crand "crypto/rand"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

// fakeSource 测试用时间源占位(faultRate 不会真调它,只用它的 Available 控制 air-gap 判定是否启用)。
// avail=true 表示「有可试目标(如 control 已下发非空 NTS 名单)」→ 启用 air-gap;取时始终失败(down)。
type fakeSource struct{ avail bool }

func (fakeSource) Name() string                       { return "fake" }
func (f fakeSource) Available() bool                  { return f.avail }
func (fakeSource) Now(context.Context) (int64, error) { return 0, errors.New("down") }

// newGate 造一个带时钟的门禁(测试用)。startAgo>0 把"启动时刻"往前拨,便于测启动宽限边界。
func newGate(startAgo time.Duration) *gate {
	g := &gate{name: "t", startupGrace: defaultStartupGrace, clock: newTrustedClock()}
	if startAgo > 0 {
		g.clock.started = time.Now().Add(-startAgo)
	}
	return g
}

// TestFaultRatePolicy 降级策略执行 + never-seen fail-closed(从没授权:窗口内放行、超窗口停)。
func TestFaultRatePolicy(t *testing.T) {
	now := time.Now().Unix()
	const day = int64(86400)

	cases := []struct {
		name     string
		v        verdict
		seen     bool
		startAgo time.Duration
		want     int
	}{
		{"从没通话-启动窗口内-放行", verdict{}, false, 0, 0},
		{"从没通话-超启动窗口-failclosed停", verdict{}, false, defaultStartupGrace + time.Minute, 100},
		{"授权有效-满速", verdict{Now: now, AuthorizedUntil: now + 3600}, true, 0, 0},
		{"吊销-立即停", verdict{Now: now, AuthorizedUntil: now + 3600, Revoked: true}, true, 0, 100},
		{"pending从未授权-不服务", verdict{Now: now, AuthorizedUntil: 0}, true, 0, 100},
		{"过期-不允许续用-停", verdict{Now: now, AuthorizedUntil: now - 10}, true, 0, 100},
		{"过期-允许续用-宽限内-按errorRate", verdict{Now: now, AuthorizedUntil: now - 10, Policy: policy{Lease: policyLease{AllowExpiredUse: true, MaxGraceDays: 30}, HTTP: policyHTTP{ErrorRate: 50}}}, true, 0, 50},
		{"过期-允许续用-超宽限上限-停", verdict{Now: now, AuthorizedUntil: now - 31*day, Policy: policy{Lease: policyLease{AllowExpiredUse: true, MaxGraceDays: 30}, HTTP: policyHTTP{ErrorRate: 50}}}, true, 0, 100},
	}
	for _, c := range cases {
		g := newGate(c.startAgo)
		if c.seen {
			g.apply(c.v) // apply 会 observe(v.Now) → fresh=true
		}
		if got := g.faultRate(); got != c.want {
			t.Errorf("%s: faultRate=%d 期望 %d", c.name, got, c.want)
		}
	}
}

// TestAuthGateFreshRequiredWhenSourceAvailable 统一认证时间门:有可试时间源却拿不到 fresh 认证时间
// → 立刻 fail-close(对 loaded 与 never-seen **一视同仁**,不给 grace 豁免)。拿到 fresh → 恢复。
func TestAuthGateFreshRequiredWhenSourceAvailable(t *testing.T) {
	wall := time.Now().Unix()

	// loaded-stale + 有可试源 + 不 fresh → 立刻 100(Isolated),即便仍在 grace 窗口内。
	g := &gate{name: "t", startupGrace: defaultStartupGrace, clock: newTrustedClock(), sources: []timeSource{fakeSource{avail: true}}}
	g.applyLoaded(verdict{Now: wall - 100, AuthorizedUntil: wall + 3600})
	if got := g.faultRate(); got != 100 {
		t.Errorf("loaded + 有源 + 无 fresh 应立刻 fail-close, got %d", got)
	}
	if _, state, _ := g.snapshot(); state != Isolated {
		t.Errorf("有源无 fresh 应判 Isolated, got %s", state)
	}
	// 拿到 fresh 认证时间 → 认证时间门放行 → 授权有效 → 0。
	g.clock.observe(time.Now().Unix())
	if got := g.faultRate(); got != 0 {
		t.Errorf("拿到 fresh 后应放行, got %d", got)
	}

	// never-seen + 有可试源 + 不 fresh → 同样立刻 100(删 state 走 never-seen 也逃不掉)。
	gNS := &gate{name: "t", startupGrace: defaultStartupGrace, clock: newTrustedClock(), sources: []timeSource{fakeSource{avail: true}}}
	if got := gNS.faultRate(); got != 100 {
		t.Errorf("never-seen + 有源 + 无 fresh 应立刻 fail-close(堵删 state 白嫖), got %d", got)
	}
	// never-seen + 有可试源 + 已拿到 fresh 但还没等到 verdict → grace 窗口内仍放行(合法新部署平滑)。
	gNS.clock.observe(time.Now().Unix())
	if got := gNS.faultRate(); got != 0 {
		t.Errorf("never-seen + 有源 + 有 fresh + grace 内应放行(合法新部署), got %d", got)
	}
}

// TestNoSourcesDoesNotBrick 【没配时间源=单独部署】认证时间门不启用:
// control 短挂 + 重启(不 fresh)但授权仍有效 → 照常服务(不误锁);授权过期才停。
func TestNoSourcesDoesNotBrick(t *testing.T) {
	wall := time.Now().Unix()

	// 没配源、超启动窗口、不 fresh、但授权有效 → 应服务(不能因 control 短挂误锁)。
	g := newGate(defaultStartupGrace + time.Minute)
	g.applyLoaded(verdict{Now: wall - 100, AuthorizedUntil: wall + 3600})
	if got := g.faultRate(); got != 0 {
		t.Errorf("没配源 + 授权有效 → 应服务(不因 control 短挂误锁), got %d", got)
	}

	// 没配源,但授权已过期 → 仍应停(过期即停,与是否 fresh 无关)。
	g2 := newGate(defaultStartupGrace + time.Minute)
	g2.applyLoaded(verdict{Now: 1, AuthorizedUntil: wall - 3600})
	if got := g2.faultRate(); got != 100 {
		t.Errorf("没配源但授权过期 → 应停, got %d", got)
	}
}

// TestStartupGraceConfigurable Config.StartupGrace 可配 + 默认 2min。
func TestStartupGraceConfigurable(t *testing.T) {
	if (&Config{}).startupGraceOrDefault() != defaultStartupGrace {
		t.Errorf("StartupGrace 缺省应为 %s", defaultStartupGrace)
	}
	if defaultStartupGrace != 2*time.Minute {
		t.Errorf("默认启动期时间门应为 2min, got %s", defaultStartupGrace)
	}
	if got := (&Config{StartupGrace: 5 * time.Minute}).startupGraceOrDefault(); got != 5*time.Minute {
		t.Errorf("显式 StartupGrace 应生效, got %s", got)
	}
	// gate 用配置窗口:never-seen 在自定义窗口边界的行为。
	g := &gate{name: "t", startupGrace: 5 * time.Minute, clock: newTrustedClock()}
	g.clock.started = time.Now().Add(-4 * time.Minute) // < 5min 窗口
	if got := g.faultRate(); got != 0 {
		t.Errorf("自定义 5min 窗口内(4min)应放行, got %d", got)
	}
	g.clock.started = time.Now().Add(-6 * time.Minute) // > 5min 窗口
	if got := g.faultRate(); got != 100 {
		t.Errorf("自定义 5min 窗口外(6min)应 fail-close, got %d", got)
	}
}

// TestExpiredPersistedStillStops 恢复的授权若按墙钟已过期,即便不 fresh 也应停(不允许续用)。
func TestExpiredPersistedStillStops(t *testing.T) {
	wall := time.Now().Unix()
	g := newGate(0)
	g.applyLoaded(verdict{Now: 1, AuthorizedUntil: wall - 3600}) // 已过期
	if got := g.faultRate(); got != 100 {
		t.Errorf("恢复已过期授权应停, got %d", got)
	}
}

// TestTrustedClockFloorAndFresh 时钟:seedFloor 设不可回拨下界(不置 fresh),observe 置 fresh。
func TestTrustedClockFloorAndFresh(t *testing.T) {
	c := newTrustedClock()
	if _, fresh := c.now(); fresh {
		t.Error("初始应不 fresh")
	}
	future := time.Now().Unix() + 100000
	c.seedFloor(future)
	n, fresh := c.now()
	if fresh {
		t.Error("seedFloor 不该置 fresh")
	}
	if n < future {
		t.Errorf("now 应不低于 floor(防回拨), got %d < %d", n, future)
	}
	c.observe(time.Now().Unix())
	if _, fresh := c.now(); !fresh {
		t.Error("observe 后应 fresh")
	}
}

// TestExemptPath 默认健康探针/指标端点恒放行 + 配置追加项也放行(否则降级会 crashloop pod)。
func TestExemptPath(t *testing.T) {
	// 仅默认集合(exempt 由 newExemptSet 建,extra 为空)。
	g := &gate{exempt: newExemptSet(nil)}
	for _, p := range []string{"/health", "/ready", "/healthz", "/readyz", "/livez", "/metrics"} {
		if !g.exemptPath(p) {
			t.Errorf("%s 应被放行(默认基础设施端点)", p)
		}
	}
	for _, p := range []string{"/api/customer/v1/subscription", "/", "/api/schedule/v1/task", "/ping"} {
		if g.exemptPath(p) {
			t.Errorf("%s 不应被放行(业务路由要能降级)", p)
		}
	}

	// 追加自定义探针:默认集合仍恒放行,且追加项也放行(追加语义,不替换)。
	g2 := &gate{exempt: newExemptSet([]string{"/ping", " /api/v1/health "})}
	for _, p := range []string{"/health", "/metrics", "/ping", "/api/v1/health"} {
		if !g2.exemptPath(p) {
			t.Errorf("%s 应被放行(默认 + 追加,含去空白)", p)
		}
	}
	if g2.exemptPath("/api/customer/v1/subscription") {
		t.Error("追加项不应误放行业务路由")
	}

	// P4:尾斜杠容忍 —— path 或配置项带尾 "/" 都命中(default 与追加皆然)。
	g3 := &gate{exempt: newExemptSet([]string{"/ping/"})} // 配置项带尾斜杠
	for _, p := range []string{"/health/", "/metrics/", "/ping", "/ping/"} {
		if !g3.exemptPath(p) {
			t.Errorf("%s 尾斜杠变体应命中 exempt", p)
		}
	}
	// P4:子路径**不**前缀命中(必须逐条精确列全,否则降级会 500 掉未列的探针子路径)。
	for _, p := range []string{"/metrics/cadvisor", "/health/live", "/ping/x"} {
		if g3.exemptPath(p) {
			t.Errorf("%s 子路径不应被前缀命中", p)
		}
	}
	// nil exempt 回退路径也要有尾斜杠容忍。
	gNil := &gate{}
	if !gNil.exemptPath("/health/") || gNil.exemptPath("/metrics/cadvisor") {
		t.Error("nil exempt 回退:尾斜杠应命中、子路径不应命中")
	}
}

// TestVerifyParseRejectsBadSig 坏签名被拒(不 panic)。
func TestVerifyParseRejectsBadSig(t *testing.T) {
	pub := make([]byte, 32)
	if _, err := verifyParse([]byte(`{"b":"AA==","s":"AA=="}`), pub, "n", "d"); err == nil {
		t.Fatal("坏签名应被拒")
	}
}

func signVerdict(t *testing.T, priv ed25519.PrivateKey, v verdict) []byte {
	t.Helper()
	body, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal verdict: %v", err)
	}
	env, err := json.Marshal(struct {
		B []byte `json:"b"`
		S []byte `json:"s"`
	}{B: body, S: ed25519.Sign(priv, body)})
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	return env
}

// TestParseVerdict 验签 + deploymentUuid(不校 nonce,供持久化恢复用)。
func TestParseVerdict(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(crand.Reader)
	blob := signVerdict(t, priv, verdict{DeploymentUuid: "d", Now: 1, AuthorizedUntil: 2})

	if _, err := parseVerdict(blob, pub, "d"); err != nil {
		t.Fatalf("有效裁决应解析通过: %v", err)
	}
	if _, err := parseVerdict(blob, pub, "other"); err == nil {
		t.Fatal("deploymentUuid 不符应被拒(防跨部署 relay)")
	}
	wrongPub, _, _ := ed25519.GenerateKey(crand.Reader)
	if _, err := parseVerdict(blob, wrongPub, "d"); err == nil {
		t.Fatal("错公钥应被拒")
	}
}

// TestVerifyParseNonceReplay 续约路径额外校 nonce:回显不符(重放旧响应)应被拒。
func TestVerifyParseNonceReplay(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(crand.Reader)
	blob := signVerdict(t, priv, verdict{DeploymentUuid: "d", Nonce: "n1", Now: 1, AuthorizedUntil: 2})

	if _, err := verifyParse(blob, pub, "n1", "d"); err != nil {
		t.Fatalf("nonce 相符应通过: %v", err)
	}
	if _, err := verifyParse(blob, pub, "n2", "d"); err == nil {
		t.Fatal("nonce 不符应被拒(重放/过期响应)")
	}
}

// TestPersistedAlive 彻底过期者不恢复(防存旧裁决刷新有效期)。
func TestPersistedAlive(t *testing.T) {
	now := time.Now().Unix()
	if !persistedAlive(verdict{AuthorizedUntil: now + 3600}) {
		t.Error("未过期应判定 alive")
	}
	if persistedAlive(verdict{AuthorizedUntil: now - 10}) {
		t.Error("过期且不允许续用应判定 dead")
	}
	if !persistedAlive(verdict{AuthorizedUntil: now - 10, Policy: policy{Lease: policyLease{AllowExpiredUse: true, MaxGraceDays: 1}}}) {
		t.Error("过期但在宽限内应判定 alive")
	}
	if persistedAlive(verdict{AuthorizedUntil: now - 2*86400, Policy: policy{Lease: policyLease{AllowExpiredUse: true, MaxGraceDays: 1}}}) {
		t.Error("超宽限上限应判定 dead")
	}
}

// TestConfigDefaults 续约间隔/门禁名的缺省 + 坐标齐全/全空判定。
func TestConfigDefaults(t *testing.T) {
	c := Config{}
	if c.renewEveryOrDefault() != defaultRenewEvery {
		t.Errorf("续约间隔缺省应为 %s", defaultRenewEvery)
	}
	if c.serviceName() != "controlgate" {
		t.Errorf("门禁名缺省应为 controlgate, got %q", c.serviceName())
	}
	if !c.coordsAllEmpty() {
		t.Error("空 Config 坐标应判定全空")
	}
	full := Config{RenewBaseURL: "u", ProjectUUID: "p", DeploymentID: "d", ProjectPubKey: "k"}
	if !full.coordsComplete() || full.coordsAllEmpty() {
		t.Error("齐全坐标应判定 complete、非全空")
	}
	partial := Config{RenewBaseURL: "u"}
	if partial.coordsComplete() || partial.coordsAllEmpty() {
		t.Error("部分坐标应判定非 complete、非全空(→ Setup fail-startup)")
	}
}

// TestDisabledGate 门禁关闭时的空 Gate:恒放行、状态 Disabled(不伪装 Serving)、无到期。
func TestDisabledGate(t *testing.T) {
	var g Gate = disabledGate{}
	if g.FaultRate() != 0 {
		t.Error("disabledGate FaultRate 应恒 0")
	}
	if g.ServingState() != Disabled {
		t.Errorf("disabledGate ServingState 应为 Disabled(让状态页看见闸关了), got %s", g.ServingState())
	}
	if g.AuthorizedUntil() != 0 {
		t.Errorf("disabledGate AuthorizedUntil 应为 0, got %d", g.AuthorizedUntil())
	}
}

// TestFailStartupOnEmptyCoords P2:坐标全空的处置分 level + RequireGate 逃生阀。
func TestFailStartupOnEmptyCoords(t *testing.T) {
	cases := []struct {
		name     string
		cfg      Config
		runLevel string
		wantFail bool
	}{
		{"eng+空坐标 → disabled", Config{}, "eng", false},
		{"local+空坐标 → disabled", Config{}, "local", false},
		{"stage+空坐标 → disabled", Config{}, "stage", false},
		{"production+空坐标 → fail-startup", Config{}, "production", true},
		{"RequireGate+eng+空坐标 → fail-startup", Config{RequireGate: true}, "eng", true},
		{"RequireGate+production → fail-startup", Config{RequireGate: true}, "production", true},
	}
	for _, c := range cases {
		if got := c.cfg.failStartupOnEmptyCoords(c.runLevel); got != c.wantFail {
			t.Errorf("%s: failStartupOnEmptyCoords=%v 期望 %v", c.name, got, c.wantFail)
		}
	}
}
