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

// ── 条件类别 schema(给配置页 UI):按 Type 排序 + 带各自字段;无动作类别 ──
func TestRegistry_Kinds(t *testing.T) {
	reg := newReg()
	ck := reg.ConditionKinds()
	if len(ck) != 3 || ck[0].Type != "asn_hosting" || ck[1].Type != "country_in" || ck[2].Type != "ua_match" {
		t.Fatalf("条件类别应排序为 asn_hosting/country_in/ua_match,got %+v", ck)
	}
	if len(ck[0].Fields) != 0 {
		t.Fatalf("asn_hosting 应无配置字段")
	}
	if len(ck[2].Fields) != 1 || ck[2].Fields[0].Key != "patterns" {
		t.Fatalf("ua_match 应有 patterns 字段,got %+v", ck[2].Fields)
	}
}
