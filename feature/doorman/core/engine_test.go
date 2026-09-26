package core

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// ── 测试辅助 ──

type mapReq struct {
	headers, fields map[string]string
}

func (m mapReq) Header(k string) string { return m.headers[k] }
func (m mapReq) Field(k string) string  { return m.fields[k] }

func raw(s string) json.RawMessage { return json.RawMessage(s) }

func newReg() *Registry { r := NewRegistry(); RegisterBuiltins(r); return r }

func ctxWith(a Attempt, req RequestView) *Context {
	return &Context{Ctx: context.Background(), Scope: "register", Attempt: a, Req: req}
}

// cond 造一个条件项。
func cond(t, params string) RuleCondition { return RuleCondition{Type: t, Params: raw(params)} }

// assess 编译一组规则并评估一次(测试便捷)。
func assess(reg *Registry, rules []Rule, a Attempt, req RequestView) Assessment {
	compiled, _ := reg.Compile(rules, "register")
	return Assess(ctxWith(a, req), compiled)
}

// ── Registry.Validate:类别存在 + 参数合法 + 风险等级/组合/条件数 ──
func TestRegistry_Validate(t *testing.T) {
	reg := newReg()
	ok := Rule{RiskLevel: RiskCritical, Conditions: []RuleCondition{cond("ua_match", `{"patterns":["x"]}`)}}
	if err := reg.Validate(ok); err != nil {
		t.Fatalf("合法规则不该报错: %v", err)
	}
	cases := map[string]Rule{
		"未知条件":   {RiskLevel: RiskHigh, Conditions: []RuleCondition{cond("nope", ``)}},
		"未知等级":   {RiskLevel: RiskLevel("nope"), Conditions: []RuleCondition{cond("asn_hosting", ``)}},
		"条件参数非法": {RiskLevel: RiskHigh, Conditions: []RuleCondition{cond("ua_match", `{"patterns":[]}`)}},
		"无条件":    {RiskLevel: RiskHigh},
		"组合非法":   {RiskLevel: RiskHigh, Combine: "xor", Conditions: []RuleCondition{cond("asn_hosting", ``)}},
	}
	for name, r := range cases {
		if err := reg.Validate(r); err == nil {
			t.Errorf("%s 应校验失败,但通过了", name)
		}
	}
}

// ── ua_match → critical:抓 Go-http-client / 空 UA;浏览器判 none ──
func TestAssess_UAMatch(t *testing.T) {
	reg := newReg()
	rules := []Rule{{
		Name: "go", Enabled: true, Scope: "register", RiskLevel: RiskCritical,
		Conditions: []RuleCondition{cond("ua_match", `{"patterns":["Go-http-client","python-requests",""]}`)},
	}}
	if a := assess(reg, rules, Attempt{UA: "Go-http-client/1.1"}, nil); a.Level != RiskCritical {
		t.Fatalf("Go client 应 critical,got %+v", a)
	}
	if a := assess(reg, rules, Attempt{UA: ""}, nil); a.Level != RiskCritical {
		t.Fatalf("空 UA 应 critical,got %+v", a)
	}
	if a := assess(reg, rules, Attempt{UA: "Mozilla/5.0 (Windows) Chrome/120"}, nil); a.Level != RiskNone {
		t.Fatalf("浏览器 UA 应 none,got %+v", a)
	}
}

// ── 多条件 AND:机房 且 海外 → high;缺一不命中 ──
func TestAssess_MultiConditionAnd(t *testing.T) {
	reg := newReg()
	rules := []Rule{{
		Name: "hosting-foreign", Enabled: true, Scope: "register", RiskLevel: RiskHigh, Combine: CombineAnd,
		Conditions: []RuleCondition{cond("asn_hosting", ``), cond("country_in", `{"countries":["US","NL"]}`)},
	}}
	if a := assess(reg, rules, Attempt{IsHosting: true, Country: "US"}, nil); a.Level != RiskHigh {
		t.Fatalf("机房+海外应 high,got %+v", a)
	}
	if a := assess(reg, rules, Attempt{IsHosting: true, Country: "CN"}, nil); a.Level != RiskNone {
		t.Fatalf("机房但国内 → AND 不命中,应 none,got %+v", a)
	}
	if a := assess(reg, rules, Attempt{IsHosting: false, Country: "US"}, nil); a.Level != RiskNone {
		t.Fatalf("海外但住宅 → AND 不命中,应 none,got %+v", a)
	}
}

// ── 多条件 OR:任一命中即命中 ──
func TestAssess_MultiConditionOr(t *testing.T) {
	reg := newReg()
	rules := []Rule{{
		Name: "hosting-or-foreign", Enabled: true, Scope: "register", RiskLevel: RiskMedium, Combine: CombineOr,
		Conditions: []RuleCondition{cond("asn_hosting", ``), cond("country_in", `{"countries":["US"]}`)},
	}}
	if a := assess(reg, rules, Attempt{IsHosting: true, Country: "CN"}, nil); a.Level != RiskMedium {
		t.Fatalf("机房(OR 任一)应 medium,got %+v", a)
	}
	if a := assess(reg, rules, Attempt{IsHosting: false, Country: "US"}, nil); a.Level != RiskMedium {
		t.Fatalf("海外(OR 任一)应 medium,got %+v", a)
	}
	if a := assess(reg, rules, Attempt{IsHosting: false, Country: "CN"}, nil); a.Level != RiskNone {
		t.Fatalf("都不命中应 none,got %+v", a)
	}
}

// ── 多规则命中取最高等级 + Matched 记录全部命中 ──
func TestAssess_TakesMaxLevel(t *testing.T) {
	reg := newReg()
	rules := []Rule{
		{Name: "foreign", Enabled: true, Scope: "register", RiskLevel: RiskMedium,
			Conditions: []RuleCondition{cond("country_in", `{"countries":["US"]}`)}},
		{Name: "script", Enabled: true, Scope: "register", RiskLevel: RiskCritical,
			Conditions: []RuleCondition{cond("ua_match", `{"patterns":["go-http-client"]}`)}},
	}
	a := assess(reg, rules, Attempt{UA: "Go-http-client/1.1", Country: "US"}, nil)
	if a.Level != RiskCritical {
		t.Fatalf("两条命中应取最高 critical,got %+v", a)
	}
	if len(a.Matched) != 2 {
		t.Fatalf("应记录 2 条命中,got %d: %+v", len(a.Matched), a.Matched)
	}
}

// ── country_in:大小写不敏感 ──
func TestCondition_CountryIn(t *testing.T) {
	reg := newReg()
	rules := []Rule{{
		Name: "foreign", Enabled: true, Scope: "register", RiskLevel: RiskMedium,
		Conditions: []RuleCondition{cond("country_in", `{"countries":["US","NL"]}`)},
	}}
	if a := assess(reg, rules, Attempt{Country: "us"}, nil); a.Level != RiskMedium {
		t.Fatalf("US 应命中,got %+v", a)
	}
	if a := assess(reg, rules, Attempt{Country: "CN"}, nil); a.Level != RiskNone {
		t.Fatalf("CN 应 none,got %+v", a)
	}
}

// ── Compile:跳过 关闭/异 scope,收集 未知类别/非法参数/未知等级/无条件 的错误 ──
func TestCompile_SkipsAndErrors(t *testing.T) {
	reg := newReg()
	rules := []Rule{
		{Name: "disabled", Enabled: false, Scope: "register", RiskLevel: RiskHigh, Conditions: []RuleCondition{cond("asn_hosting", ``)}},
		{Name: "otherscope", Enabled: true, Scope: "login", RiskLevel: RiskHigh, Conditions: []RuleCondition{cond("asn_hosting", ``)}},
		{Name: "unknowncond", Enabled: true, Scope: "register", RiskLevel: RiskHigh, Conditions: []RuleCondition{cond("nope", ``)}},
		{Name: "badlevel", Enabled: true, Scope: "register", RiskLevel: RiskLevel("nope"), Conditions: []RuleCondition{cond("asn_hosting", ``)}},
		{Name: "badparams", Enabled: true, Scope: "register", RiskLevel: RiskHigh, Conditions: []RuleCondition{cond("ua_match", `{"patterns":[]}`)}},
		{Name: "nocond", Enabled: true, Scope: "register", RiskLevel: RiskHigh},
		{Name: "good", Enabled: true, Scope: "register", RiskLevel: RiskHigh, Conditions: []RuleCondition{cond("asn_hosting", ``)}},
	}
	compiled, errs := reg.Compile(rules, "register")
	if len(compiled) != 1 {
		t.Fatalf("应只编译出 good 一条,got %d", len(compiled))
	}
	if len(errs) != 4 {
		t.Fatalf("应有 4 条错误(unknowncond/badlevel/badparams/nocond),got %d: %v", len(errs), errs)
	}
}

type fakeSource struct{ m map[string][]Rule }

func (f fakeSource) Rules(scope string) ([]Rule, error) { return f.m[scope], nil }

// ── 门面 Assess + 缓存 + 无规则=none ──
func TestDoorman_Assess(t *testing.T) {
	reg := newReg()
	src := fakeSource{m: map[string][]Rule{"register": {{
		Name: "go", Enabled: true, Scope: "register", RiskLevel: RiskCritical,
		Conditions: []RuleCondition{cond("ua_match", `{"patterns":["go-http-client"]}`)},
	}}}}
	d := New(reg, src, nil, nil, time.Second)
	if a := d.Assess(ctxWith(Attempt{UA: "Go-http-client/1.1"}, nil)); a.Level != RiskCritical {
		t.Fatalf("门面应判 critical,got %+v", a)
	}
	gc := ctxWith(Attempt{UA: "Go-http-client/1.1"}, nil)
	gc.Scope = "unknown"
	if a := d.Assess(gc); a.Level != RiskNone {
		t.Fatalf("无规则的 scope 应判 none,got %+v", a)
	}
}

// ── 限流条件(有状态,靠 Store 计数) ──

// fakeCounter 是 Store 的内存替身:同 key 累加计数,忽略 window。
type fakeCounter struct{ n map[string]int64 }

func newCounter() *fakeCounter { return &fakeCounter{n: map[string]int64{}} }
func (f *fakeCounter) Incr(_ context.Context, key string, _ time.Duration) (int64, error) {
	f.n[key]++
	return f.n[key], nil
}

func TestCondition_RateLimit(t *testing.T) {
	reg := newReg()
	rules := []Rule{{
		Name: "ip-flood", Enabled: true, Scope: "register", RiskLevel: RiskHigh,
		Conditions: []RuleCondition{cond("rate_limit", `{"by":"ip","window":"1h","max":2}`)},
	}}
	compiled, errs := reg.Compile(rules, "register")
	if len(errs) != 0 || len(compiled) != 1 {
		t.Fatalf("合法 rate_limit 规则应编译成功,errs=%v", errs)
	}
	store := newCounter()
	hit := func() bool {
		c := &Context{Ctx: context.Background(), Scope: "register", Attempt: Attempt{IP: "1.2.3.4"}, Store: store}
		return Assess(c, compiled).Level == RiskHigh
	}
	if hit() || hit() { // 第 1、2 次:n=1,2 不 > 2,不命中
		t.Fatal("窗内前 max 次不该命中")
	}
	if !hit() { // 第 3 次:n=3 > 2,命中
		t.Fatal("第 max+1 次应命中")
	}
	// 不同 IP 独立分桶
	c2 := &Context{Ctx: context.Background(), Scope: "register", Attempt: Attempt{IP: "9.9.9.9"}, Store: store}
	if Assess(c2, compiled).Level == RiskHigh {
		t.Fatal("不同 IP 应独立计数,不该被别的桶带命中")
	}
}

func TestCondition_RateLimit_NoStoreOrValue(t *testing.T) {
	reg := newReg()
	compiled, _ := reg.Compile([]Rule{{
		Name: "rl", Enabled: true, Scope: "register", RiskLevel: RiskHigh,
		Conditions: []RuleCondition{cond("rate_limit", `{"by":"ip","window":"1h","max":1}`)},
	}}, "register")
	// 没注入 Store:恒不命中(best-effort,不误伤)
	noStore := &Context{Ctx: context.Background(), Scope: "register", Attempt: Attempt{IP: "1.2.3.4"}}
	if Assess(noStore, compiled).Level == RiskHigh {
		t.Fatal("没注入 Store 时限流条件应恒不命中")
	}
	// 按 subject 分桶但没设 subject:取不到值,不命中
	compiledSub, _ := reg.Compile([]Rule{{
		Name: "rl2", Enabled: true, Scope: "register", RiskLevel: RiskHigh,
		Conditions: []RuleCondition{cond("rate_limit", `{"by":"subject","window":"1h","max":1}`)},
	}}, "register")
	noSubj := &Context{Ctx: context.Background(), Scope: "register", Store: newCounter()}
	if Assess(noSubj, compiledSub).Level == RiskHigh {
		t.Fatal("按 subject 分桶但未设 subject 时不该命中")
	}
}

func TestCondition_RateLimit_BadConfig(t *testing.T) {
	reg := newReg()
	bad := []string{
		`{"by":"nope","window":"1h","max":2}`, // by 非法
		`{"by":"ip","window":"","max":2}`,     // window 空
		`{"by":"ip","window":"1h","max":0}`,   // max < 1
	}
	for _, params := range bad {
		r := Rule{Name: "x", RiskLevel: RiskHigh, Conditions: []RuleCondition{cond("rate_limit", params)}}
		if err := reg.Validate(r); err == nil {
			t.Errorf("非法 rate_limit 配置应校验失败:%s", params)
		}
	}
}

// ── svc 记录路径:Record 建 Decision(含 Challenged)、MarkOutcome、ActionFor 策略缓存 ──

type fakeRec struct {
	last     *Decision
	outcomes []string // "scope|subject|outcome"
}

func (f *fakeRec) Record(d Decision) error { f.last = &d; return nil }
func (f *fakeRec) MarkOutcome(scope, subject, outcome string) error {
	f.outcomes = append(f.outcomes, scope+"|"+subject+"|"+outcome)
	return nil
}

type fakePolicy struct{ m map[string]map[string]string }

func (f fakePolicy) Policy(scope string) (map[string]string, error) { return f.m[scope], nil }

func TestSvc_RecordAndMarkOutcome(t *testing.T) {
	rec := &fakeRec{}
	d := New(newReg(), fakeSource{}, nil, rec, time.Second)

	// 有 Subject → Challenged=true,事实字段映射进 Decision
	c := &Context{Scope: "register", Subject: "hash@x", Attempt: Attempt{UA: "u", IP: "1.1.1.1", ASN: 42, IsHosting: true}}
	a := Assessment{Level: RiskHigh, Matched: []MatchedRule{{Name: "r1"}, {Name: "r2"}}}
	d.Record(c, a, "email_verify")
	if rec.last == nil || !rec.last.Challenged {
		t.Fatal("设了 Subject 的决策应 Challenged=true")
	}
	if rec.last.Matched != "r1,r2" || rec.last.Action != "email_verify" || rec.last.ASN != 42 || !rec.last.IsHosting {
		t.Fatalf("Decision 字段映射不对:%+v", rec.last)
	}

	// 无 Subject → Challenged=false
	rec.last = nil
	d.Record(&Context{Scope: "login", Attempt: Attempt{IP: "2.2.2.2"}}, Assessment{Level: RiskLow}, "allow")
	if rec.last == nil || rec.last.Challenged {
		t.Fatal("没 Subject 的决策应 Challenged=false")
	}

	// MarkOutcome 透传;空 scope/subject 被吞
	d.MarkOutcome("register", "hash@x", "activated")
	d.MarkOutcome("", "hash@x", "activated")
	if len(rec.outcomes) != 1 || rec.outcomes[0] != "register|hash@x|activated" {
		t.Fatalf("MarkOutcome 透传不对:%v", rec.outcomes)
	}
}

func TestSvc_RecordNoRecorderIsNoop(t *testing.T) {
	d := New(newReg(), fakeSource{}, nil, nil, time.Second) // 没接 recorder
	// 不 panic 即可
	d.Record(&Context{Scope: "register", Subject: "x"}, Assessment{Level: RiskHigh}, "block")
	d.MarkOutcome("register", "x", "activated")
}

func TestSvc_ActionFor(t *testing.T) {
	pol := fakePolicy{m: map[string]map[string]string{
		"register": {"critical": "block", "high": "email_verify", "low": ""},
	}}
	d := New(newReg(), fakeSource{}, pol, nil, time.Second)

	if a, ok := d.ActionFor("register", RiskCritical); !ok || a != "block" {
		t.Fatalf("critical 应映射 block,got %q %v", a, ok)
	}
	if a, ok := d.ActionFor("register", RiskHigh); !ok || a != "email_verify" {
		t.Fatalf("high 应映射 email_verify,got %q %v", a, ok)
	}
	if _, ok := d.ActionFor("register", RiskLow); ok {
		t.Fatal("空动作名应 ok=false(回退业务默认)")
	}
	if _, ok := d.ActionFor("register", RiskMedium); ok {
		t.Fatal("没配的等级应 ok=false")
	}
	// 没接策略来源 → 恒 ok=false
	d2 := New(newReg(), fakeSource{}, nil, nil, time.Second)
	if _, ok := d2.ActionFor("register", RiskCritical); ok {
		t.Fatal("没接策略来源时 ActionFor 应恒 ok=false")
	}
}

// ── 条件类别 schema(给配置页 UI):按 Type 排序 + 带各自字段;无动作类别 ──
func TestRegistry_Kinds(t *testing.T) {
	reg := newReg()
	ck := reg.ConditionKinds()
	if len(ck) != 4 || ck[0].Type != "asn_hosting" || ck[1].Type != "country_in" || ck[2].Type != "rate_limit" || ck[3].Type != "ua_match" {
		t.Fatalf("条件类别应排序为 asn_hosting/country_in/rate_limit/ua_match,got %+v", ck)
	}
	if len(ck[0].Fields) != 0 {
		t.Fatalf("asn_hosting 应无配置字段")
	}
	if len(ck[3].Fields) != 1 || ck[3].Fields[0].Key != "patterns" {
		t.Fatalf("ua_match 应有 patterns 字段,got %+v", ck[3].Fields)
	}
}
