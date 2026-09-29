package controlgate

import (
	"context"
	"crypto/ed25519"
	crand "crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// tempStateDir 建一个**项目内**临时目录(cwd = 包目录,位于项目 temp/ 的 clone 内),用完清理。
// 不用 t.TempDir()(那会落到系统 /var/folders,违反本仓"临时只在项目内"的铁律)。
func tempStateDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp(".", "cgstate")
	if err != nil {
		t.Fatalf("建临时目录失败: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// TestMiddleware500Injection 中间件端到端:faultRate=100 时非 exempt 路径真返回 500;
// exempt 路径(默认 + 配置追加)永远放行;faultRate=0 时全放行。
func TestMiddleware500Injection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now().Unix()

	g := &gate{name: "t", clock: newTrustedClock(), exempt: newExemptSet([]string{"/custom-health"})}
	// 吊销 → faultRate 恒 100(确定性,不靠随机 errorRate)。
	g.apply(verdict{Now: now, AuthorizedUntil: now + 3600, Revoked: true})
	if g.faultRate() != 100 {
		t.Fatalf("前置:吊销应 faultRate=100, got %d", g.faultRate())
	}

	eng := gin.New()
	eng.Use(g.middleware())
	ok := func(c *gin.Context) { c.String(http.StatusOK, "ok") }
	eng.GET("/biz", ok)
	eng.GET("/health", ok)        // 默认 exempt
	eng.GET("/metrics", ok)       // 默认 exempt
	eng.GET("/custom-health", ok) // 配置追加 exempt

	do := func(path string) int {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		eng.ServeHTTP(w, req)
		return w.Code
	}

	// faultRate=100:业务路由被注入 500。
	if code := do("/biz"); code != http.StatusInternalServerError {
		t.Errorf("faultRate=100 业务路由应 500, got %d", code)
	}
	// exempt 路径永不降级(否则会 crashloop 探针 / 打瞎观测)。
	for _, p := range []string{"/health", "/metrics", "/custom-health"} {
		if code := do(p); code != http.StatusOK {
			t.Errorf("exempt 路径 %s 应始终 200, got %d", p, code)
		}
	}

	// 恢复有效授权 → faultRate=0 → 业务路由放行。
	g.apply(verdict{Now: now, AuthorizedUntil: now + 3600})
	if g.faultRate() != 0 {
		t.Fatalf("前置:有效授权应 faultRate=0, got %d", g.faultRate())
	}
	if code := do("/biz"); code != http.StatusOK {
		t.Errorf("faultRate=0 业务路由应 200, got %d", code)
	}
}

// fakeControl 起一个 httptest 假 control:用测试私钥签名 verdict 回给续约请求。
// calls 计每次被调,便于断言续约循环真的打了 control。
func fakeControl(t *testing.T, priv ed25519.PrivateKey, dep string, au int64, calls *int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Nonce string `json:"nonce"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		*calls++
		env := signVerdict(t, priv, verdict{
			Nonce:           req.Nonce, // 回显 nonce(过 verifyParse 的防重放)
			DeploymentUuid:  dep,
			Now:             time.Now().Unix(),
			AuthorizedUntil: au,
		})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(env)
	}))
}

// TestRunRenewLoop 续约循环:假 control 返回签名 verdict → gate apply 之 + 落盘持久化。
func TestRunRenewLoop(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(crand.Reader)
	const dep = "dep-1"
	au := time.Now().Unix() + 3600

	calls := 0
	srv := fakeControl(t, priv, dep, au, &calls)
	defer srv.Close()

	statePath := filepath.Join(tempStateDir(t), "t.cache")
	sealK := sealKey(pub)

	g := &gate{name: "t", clock: newTrustedClock()}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go g.run(ctx, srv.URL, "proj-1", dep, pub, 20*time.Millisecond, statePath, sealK)

	// 等续约成功:seen 且 faultRate 归 0。
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if g.faultRate() == 0 && g.AuthorizedUntil() == au {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()

	if calls == 0 {
		t.Fatal("续约循环没打到假 control")
	}
	if g.AuthorizedUntil() != au {
		t.Errorf("gate 未 apply 裁决 authorizedUntil,期望 %d got %d", au, g.AuthorizedUntil())
	}
	if g.faultRate() != 0 {
		t.Errorf("apply 有效授权后应 faultRate=0, got %d", g.faultRate())
	}

	// 落盘:能从持久化 loadPersisted 回来且驱动正确 faultRate。
	v, ok := loadPersisted(statePath, sealK, pub, dep)
	if !ok {
		t.Fatal("续约成功应 savePersisted 落盘,loadPersisted 却拿不到")
	}
	if v.AuthorizedUntil != au {
		t.Errorf("持久化 verdict authorizedUntil 期望 %d got %d", au, v.AuthorizedUntil)
	}
}

// TestPersistRoundTrip 持久化全周期:savePersisted → loadPersisted → applyLoaded 驱动正确 faultRate。
func TestPersistRoundTrip(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(crand.Reader)
	const dep = "dep-x"
	sealK := sealKey(pub)
	statePath := filepath.Join(tempStateDir(t), "rt.cache")
	now := time.Now().Unix()

	// 有效授权:落盘 → 加载 → applyLoaded → faultRate 0。
	blob := signVerdict(t, priv, verdict{DeploymentUuid: dep, Now: now, AuthorizedUntil: now + 3600})
	savePersisted(statePath, sealK, blob)
	v, ok := loadPersisted(statePath, sealK, pub, dep)
	if !ok {
		t.Fatal("有效授权应能 loadPersisted 回来")
	}
	g := newGate(0)
	g.applyLoaded(v)
	if g.faultRate() != 0 {
		t.Errorf("恢复有效授权应 faultRate=0, got %d", g.faultRate())
	}
	if g.AuthorizedUntil() != now+3600 {
		t.Errorf("恢复 authorizedUntil 期望 %d got %d", now+3600, g.AuthorizedUntil())
	}

	// 吊销授权:落盘 → 加载 → applyLoaded → faultRate 100(isolated)。
	blob2 := signVerdict(t, priv, verdict{DeploymentUuid: dep, Now: now, AuthorizedUntil: now + 3600, Revoked: true})
	savePersisted(statePath, sealK, blob2)
	v2, ok := loadPersisted(statePath, sealK, pub, dep)
	if !ok {
		t.Fatal("吊销但未过期的裁决应能 loadPersisted 回来")
	}
	g2 := newGate(0)
	g2.applyLoaded(v2)
	if g2.faultRate() != 100 {
		t.Errorf("恢复吊销授权应 faultRate=100, got %d", g2.faultRate())
	}
	if _, state, _ := g2.snapshot(); state != Isolated {
		t.Errorf("吊销应判 Isolated, got %s", state)
	}
}

// TestSnapshotStates snapshot 各状态下 (shed, state, ttl) 三值对。
func TestSnapshotStates(t *testing.T) {
	now := time.Now().Unix()
	const day = int64(86400)

	// 启动窗口内(从未通话):serving,shed=0,ttl>0(距窗口结束)。
	g := newGate(0)
	shed, state, ttl := g.snapshot()
	if state != Serving || shed != 0 || ttl <= 0 {
		t.Errorf("启动窗口内应 serving/0/ttl>0, got %v/%v/%v", state, shed, ttl)
	}

	// 超启动窗口仍无授权:tripped,shed=100,ttl=0。
	g2 := newGate(defaultStartupGrace + time.Minute)
	shed, state, ttl = g2.snapshot()
	if state != Tripped || shed != 100 || ttl != 0 {
		t.Errorf("超窗口无授权应 tripped/100/0, got %v/%v/%v", state, shed, ttl)
	}

	// 有效授权:serving,shed=0,ttl≈距过期。
	g3 := newGate(0)
	g3.apply(verdict{Now: now, AuthorizedUntil: now + 3600})
	shed, state, ttl = g3.snapshot()
	if state != Serving || shed != 0 || ttl <= 0 || ttl > 3601 {
		t.Errorf("有效授权应 serving/0/ttl≈3600, got %v/%v/%v", state, shed, ttl)
	}

	// 吊销:isolated,shed=100。
	g4 := newGate(0)
	g4.apply(verdict{Now: now, AuthorizedUntil: now + 3600, Revoked: true})
	if shed, state, _ := g4.snapshot(); state != Isolated || shed != 100 {
		t.Errorf("吊销应 isolated/100, got %v/%v", state, shed)
	}

	// pending(au=0):tripped,shed=100。
	g5 := newGate(0)
	g5.apply(verdict{Now: now, AuthorizedUntil: 0})
	if shed, state, _ := g5.snapshot(); state != Tripped || shed != 100 {
		t.Errorf("pending 应 tripped/100, got %v/%v", state, shed)
	}

	// 过期宽限内:shedding,shed=errorRate,ttl>0(距彻底停)。
	g6 := newGate(0)
	g6.apply(verdict{Now: now, AuthorizedUntil: now - 10, Policy: policy{
		Lease: policyLease{AllowExpiredUse: true, MaxGraceDays: 30}, HTTP: policyHTTP{ErrorRate: 40},
	}})
	shed, state, ttl = g6.snapshot()
	if state != Shedding || shed != 40 || ttl <= 0 || ttl > float64(30*day) {
		t.Errorf("过期宽限内应 shedding/40/ttl>0, got %v/%v/%v", state, shed, ttl)
	}
}

// TestServingStateString 状态名与 Grafana value-mapping 口径一致。
func TestServingStateString(t *testing.T) {
	cases := map[ServingState]string{Serving: "serving", Shedding: "shedding", Tripped: "tripped", Isolated: "isolated", Disabled: "disabled", ServingState(99): "unknown"}
	for s, want := range cases {
		if got := s.String(); got != want {
			t.Errorf("ServingState(%d).String()=%q 期望 %q", int(s), got, want)
		}
	}
}

// signRaw 用 priv 对**原始 body 字节**签名并封成 {b,s} 信封(供前向兼容测试塞未知字段)。
func signRaw(t *testing.T, priv ed25519.PrivateKey, body []byte) []byte {
	t.Helper()
	env, err := json.Marshal(struct {
		B []byte `json:"b"`
		S []byte `json:"s"`
	}{B: body, S: ed25519.Sign(priv, body)})
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	return env
}

// TestRunRenewRejectsBadSig 拒绝路径 fail-closed:假 control 能连上、但返回**坏签名**裁决 →
// gate 永不 apply(AuthorizedUntil 不变)→ 超启动期时间门后 faultRate=100。证明"能连但裁决不可信"也 fail-close。
func TestRunRenewRejectsBadSig(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(crand.Reader)      // run() 用这把公钥验签
	_, wrongPriv, _ := ed25519.GenerateKey(crand.Reader) // 假 control 却用**另一把**私钥签 → 验签必败
	const dep = "dep-bad"

	calls := 0
	srv := fakeControl(t, wrongPriv, dep, time.Now().Unix()+3600, &calls)
	defer srv.Close()

	statePath := filepath.Join(tempStateDir(t), "bad.cache")
	g := &gate{name: "t", clock: newTrustedClock()}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go g.run(ctx, srv.URL, "proj", dep, pub, 15*time.Millisecond, statePath, sealKey(pub))

	// 给它几轮续约的时间;坏签名一律被拒,gate 不应 apply。
	time.Sleep(200 * time.Millisecond)
	cancel()

	if calls == 0 {
		t.Fatal("前置:假 control 应被打到(否则测的不是拒绝路径)")
	}
	if g.AuthorizedUntil() != 0 {
		t.Errorf("坏签名裁决绝不应被 apply,AuthorizedUntil 应保持 0, got %d", g.AuthorizedUntil())
	}
	// 从没拿到可信授权 + 超启动宽限 → fail-closed。
	g.clock.started = time.Now().Add(-defaultStartupGrace - time.Minute)
	if got := g.faultRate(); got != 100 {
		t.Errorf("能连但裁决不可信 + 超宽限应 fail-close(100), got %d", got)
	}
}

// TestApplyWiresAirGapSource air-gap 接线:apply 带非空 NtsServers 的裁决 → 源 Available()==true
// (启用 air-gap 判定);空名单 → false(不启用,防误锁)。串起"control 下发名单 → 启用 air-gap"。
func TestApplyWiresAirGapSource(t *testing.T) {
	nts := newNtsSource("t", false)
	g := &gate{name: "t", clock: newTrustedClock(), sources: []timeSource{nts}, nts: nts}

	if g.hasAvailableSource() {
		t.Fatal("初始无名单不应有可试源")
	}
	// control 下发非空 NTS 名单 → 启用 air-gap。
	g.apply(verdict{Now: time.Now().Unix(), AuthorizedUntil: time.Now().Unix() + 3600,
		NtsServers: []ntsServerWire{{Host: "nts.example", Port: 4460, Pin: "sha256/AAAA"}}})
	if !g.hasAvailableSource() {
		t.Error("下发非空 NTS 名单后应有可试源(air-gap 启用)")
	}
	// 再下发空名单 → 关掉 air-gap(防 control 短挂 + 重启误锁拿不到源的合法机器)。
	g.apply(verdict{Now: time.Now().Unix(), AuthorizedUntil: time.Now().Unix() + 3600, NtsServers: nil})
	if g.hasAvailableSource() {
		t.Error("下发空名单后不应再有可试源(air-gap 关闭)")
	}
}

// TestVerdictForwardCompat 前向兼容:裁决含**未知字段 + 未知命名空间**仍正常 parse、不报错、已知字段不受影响。
// 锁"control 加键/加命名空间对本消费方零改动"承诺。
func TestVerdictForwardCompat(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(crand.Reader)
	const dep = "dep-fc"
	au := time.Now().Unix() + 7200

	// 顶层未知字段 futureTopLevel;policy 里未知命名空间 futureNamespace;已知字段照常。
	body := []byte(`{
		"nonce": "n",
		"deploymentUuid": "` + dep + `",
		"now": 100,
		"authorizedUntil": ` + strconv.FormatInt(au, 10) + `,
		"revoked": false,
		"futureTopLevel": {"x": 1},
		"policy": {"v": 9, "lease": {"allowExpiredUse": true, "maxGraceDays": 5}, "http": {"errorRate": 20}, "futureNamespace": {"k": "v"}},
		"ntsServers": [{"host": "a", "port": 4460, "pin": "sha256/AAAA", "futureField": 7}]
	}`)
	blob := signRaw(t, priv, body)

	v, err := parseVerdict(blob, pub, dep)
	if err != nil {
		t.Fatalf("含未知字段的合法裁决应正常 parse, got %v", err)
	}
	if v.AuthorizedUntil != au || v.Policy.V != 9 || !v.Policy.Lease.AllowExpiredUse ||
		v.Policy.Lease.MaxGraceDays != 5 || v.Policy.HTTP.ErrorRate != 20 || len(v.NtsServers) != 1 {
		t.Errorf("已知字段应被正确解析,未知字段被忽略, got %+v", v)
	}
}

// TestNoNtsResidueShortGraceWindow 无 NTS 源时的窄残留特征化(有意残留,非 bug):
// **没配 NTS 源**时,删 state → never-seen → 短 grace 窗口(默认 2min)内无条件放行。攻击者 root 只能
// 靠每 < grace 重启一次来维持(噪声大、监控看穿),且远小于旧 30min。没有认证时间源就无法区分"气隙偷跑"
// vs "合法冷启动",只能认这个窄窗口;真正根治靠后续「在线材料依赖」阶段(节点凭据现领)。
// 与 TestAuthGateFreshRequiredWhenSourceAvailable 互补(**配了** NTS 源时 never-seen 也必须拿 fresh)。
func TestNoNtsResidueShortGraceWindow(t *testing.T) {
	g := newGate(0) // never-seen(删 state 后的状态),无 sources
	if g.hasAvailableSource() {
		t.Fatal("前置:本用例应无可试时间源(才构成窄残留)")
	}
	// grace 窗口内 → 放行(窄残留)。
	if got := g.faultRate(); got != 0 {
		t.Errorf("无 NTS + never-seen + grace 窗口内应 serve(0,窄残留), got %d", got)
	}
	// 超 grace 窗口 → fail-close(不再有旧 30min 白嫖)。
	g.clock.started = time.Now().Add(-defaultStartupGrace - time.Minute)
	if got := g.faultRate(); got != 100 {
		t.Errorf("无 NTS + never-seen + 超 grace 应 fail-close(100), got %d", got)
	}
}

// TestAssertNoRoutes P1:engine 已注册路由时 Setup 的路由时序自守卫应报错(那些路由会绕过门禁)。
func TestAssertNoRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 空 engine(controlgate 在 RegisterRoutes 之前)→ 放行。
	if err := assertNoRoutes(gin.New(), "t"); err != nil {
		t.Errorf("空 engine 不应报错: %v", err)
	}

	// 已注册路由(controlgate 在 RegisterRoutes 之后)→ fail-startup。
	eng := gin.New()
	eng.GET("/pre-existing", func(c *gin.Context) {})
	if err := assertNoRoutes(eng, "t"); err == nil {
		t.Error("engine 已有路由时应报错(路由会静默绕过门禁)")
	}
}
