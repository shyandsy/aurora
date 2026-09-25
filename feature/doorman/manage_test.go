package doorman

import (
	"errors"
	"sort"
	"testing"
	"time"
)

// fakeStore 是 ruleStore 的内存替身,测 Console 不必起真 DB。
type fakeStore struct {
	rows   map[int64]ruleRow
	next   int64
	policy map[string]map[string]string // scope → {level: action}
}

func newFakeStore() *fakeStore {
	return &fakeStore{rows: map[int64]ruleRow{}, next: 1, policy: map[string]map[string]string{}}
}

func (f *fakeStore) Policy(scope string) (map[string]string, error) {
	out := map[string]string{}
	for k, v := range f.policy[scope] {
		out[k] = v
	}
	return out, nil
}
func (f *fakeStore) setPolicy(scope string, mapping map[string]string) error {
	m := map[string]string{}
	for k, v := range mapping {
		m[k] = v
	}
	f.policy[scope] = m
	return nil
}
func (f *fakeStore) recordDecision(row *decisionRow) error            { return nil }
func (f *fakeStore) markOutcome(scope, subject, outcome string) error { return nil }
func (f *fakeStore) funnel(scope string, since time.Time) (funnelCounts, error) {
	return funnelCounts{}, nil
}
func (f *fakeStore) stats(scope string, since time.Time) (map[string]int64, map[string]int64, error) {
	return map[string]int64{}, map[string]int64{}, nil
}
func (f *fakeStore) decisions(scope string, limit int, beforeID int64) ([]decisionRow, error) {
	return nil, nil
}

func (f *fakeStore) Rules(scope string) ([]Rule, error) {
	rows, _ := f.listRows(scope)
	out := make([]Rule, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.toRule())
	}
	return out, nil
}
func (f *fakeStore) listRows(scope string) ([]ruleRow, error) {
	var out []ruleRow
	for _, r := range f.rows {
		if scope == "" || r.Scope == scope {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
func (f *fakeStore) getRow(id int64) (*ruleRow, error) {
	if r, ok := f.rows[id]; ok {
		rr := r
		return &rr, nil
	}
	return nil, nil
}
func (f *fakeStore) upsertRow(row *ruleRow) error {
	if row.ID == 0 {
		row.ID = f.next
		f.next++
	}
	f.rows[row.ID] = *row
	return nil
}
func (f *fakeStore) deleteRow(id int64) error { delete(f.rows, id); return nil }

func newTestConsole() (*fakeStore, Console) {
	reg := newReg()
	st := newFakeStore()
	return st, newConsole(reg, st)
}

// dtoCond 造一个 ConditionDTO(params 用内联 map)。
func dtoCond(t string, params any) ConditionDTO { return ConditionDTO{Type: t, Params: params} }

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

// ── scope 注册表:Scopes 反映注册 + 动作类型 + 保存校验 scope(挡幻影配置) ──
func TestConsole_ScopeRegistry(t *testing.T) {
	reg := newReg()
	reg.RegisterScope("register", "注册", []ActionDef{
		{Name: "allow", Label: "放行", Kind: ActionTerminal},
		{Name: "email_verify", Label: "要求邮件激活", Kind: ActionFriction},
	})
	svc := newConsole(reg, newFakeStore())

	// Scopes() 反映注册(id/标签/动作/类型)
	scopes := svc.Scopes()
	if len(scopes) != 1 || scopes[0].ID != "register" || scopes[0].Label != "注册" || len(scopes[0].Actions) != 2 {
		t.Fatalf("Scopes 应含 register+2动作,got %+v", scopes)
	}

	// 动作类型:email_verify=friction、allow=terminal;默认(未声明)= terminal
	if k, ok := reg.ActionKindOf("register", "email_verify"); !ok || k != ActionFriction {
		t.Errorf("email_verify 应 friction,got %v %v", k, ok)
	}
	if k, _ := reg.ActionKindOf("register", "allow"); k != ActionTerminal {
		t.Errorf("allow 应 terminal,got %v", k)
	}
	reg.RegisterScope("register", "", []ActionDef{{Name: "captcha"}}) // 不给 kind → 默认 terminal
	if k, _ := reg.ActionKindOf("register", "captcha"); k != ActionTerminal {
		t.Errorf("未声明类型应默认 terminal,got %v", k)
	}

	// UpsertRule:注册过的 scope 放行,未注册的拒(幻影 scope)
	good := RuleDTO{Scope: "register", Name: "r", RiskLevel: string(RiskHigh),
		Conditions: []ConditionDTO{dtoCond("asn_hosting", nil)}}
	if _, err := svc.UpsertRule(good); err != nil {
		t.Fatalf("已注册 scope 的规则应能存: %v", err)
	}
	bad := RuleDTO{Scope: "login", Name: "r2", RiskLevel: string(RiskHigh),
		Conditions: []ConditionDTO{dtoCond("asn_hosting", nil)}}
	if _, err := svc.UpsertRule(bad); err == nil {
		t.Error("未注册 scope 的规则应被拒(幻影配置)")
	}
	// SetPolicy 同样校验 scope
	if err := svc.SetPolicy("login", map[string]string{string(RiskHigh): "block"}); err == nil {
		t.Error("未注册 scope 的策略应被拒")
	}
}

// ── Console:增改删 + 校验拦截 + 不存在 ──
func TestConsole_CRUD(t *testing.T) {
	_, svc := newTestConsole()

	// 新增合法规则(多条件 AND → critical)
	created, err := svc.UpsertRule(RuleDTO{
		Scope: "register", Name: "kill go", RiskLevel: string(RiskCritical), Combine: CombineAnd, Enabled: true,
		Conditions: []ConditionDTO{dtoCond("ua_match", map[string]any{"patterns": []string{"Go-http-client"}})},
	})
	if err != nil {
		t.Fatalf("新增合法规则失败: %v", err)
	}
	if created.ID == 0 {
		t.Fatal("新增应回填 ID")
	}

	// 列出 + 往返:条件类别与风险等级保住
	list, _ := svc.ListRules("register")
	if len(list) != 1 || len(list[0].Conditions) != 1 || list[0].Conditions[0].Type != "ua_match" || list[0].RiskLevel != string(RiskCritical) {
		t.Fatalf("应列出 1 条 ua_match/critical,got %+v", list)
	}

	// 非法(无条件)被拦,不入库
	if _, err := svc.UpsertRule(RuleDTO{Scope: "register", RiskLevel: string(RiskHigh)}); err == nil {
		t.Fatal("无条件应被校验拦截")
	}
	if list, _ := svc.ListRules("register"); len(list) != 1 {
		t.Fatalf("非法规则不该入库,仍应 1 条,got %d", len(list))
	}

	// 更新已存在(改等级)
	created.RiskLevel = string(RiskHigh)
	if _, err := svc.UpsertRule(*created); err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	if list, _ := svc.ListRules("register"); list[0].RiskLevel != string(RiskHigh) {
		t.Fatalf("更新未生效,got riskLevel=%s", list[0].RiskLevel)
	}

	// 更新不存在
	if _, err := svc.UpsertRule(RuleDTO{ID: 999, Scope: "register", RiskLevel: string(RiskHigh), Conditions: []ConditionDTO{dtoCond("asn_hosting", nil)}}); !errors.Is(err, ErrRuleNotFound) {
		t.Fatalf("更新不存在应回 ErrRuleNotFound,got %v", err)
	}

	// 删除
	if err := svc.DeleteRule(created.ID); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	if list, _ := svc.ListRules("register"); len(list) != 0 {
		t.Fatalf("删除后应为空,got %d", len(list))
	}
	if err := svc.DeleteRule(created.ID); !errors.Is(err, ErrRuleNotFound) {
		t.Fatalf("删除不存在应回 ErrRuleNotFound,got %v", err)
	}
}

// ── Kinds:返回内置条件类别 schema + 风险等级枚举(无动作) ──
func TestConsole_Kinds(t *testing.T) {
	_, svc := newTestConsole()
	k := svc.Kinds()
	if len(k.Conditions) != 3 {
		t.Fatalf("应有 3 条件类别,got %d", len(k.Conditions))
	}
	if len(k.RiskLevels) != 5 {
		t.Fatalf("应有 5 个风险等级,got %d", len(k.RiskLevels))
	}
}

// ── 「风险→动作」策略:登记动作目录 + 保存校验 + 往返 ──
func TestConsole_Policy(t *testing.T) {
	reg := newReg()
	reg.RegisterActions("register", []string{"allow", "block", "email_verify"})
	st := newFakeStore()
	svc := newConsole(reg, st)

	// PolicyKinds:5 等级 + 3 动作
	pk := svc.PolicyKinds("register")
	if len(pk.RiskLevels) != 5 || len(pk.Actions) != 3 {
		t.Fatalf("PolicyKinds 应 5 等级/3 动作,got %d/%d", len(pk.RiskLevels), len(pk.Actions))
	}

	// 合法保存 + 往返
	if err := svc.SetPolicy("register", map[string]string{"critical": "block", "high": "email_verify", "low": ""}); err != nil {
		t.Fatalf("合法策略保存失败: %v", err)
	}
	got, _ := svc.GetPolicy("register")
	if got["critical"] != "block" || got["high"] != "email_verify" {
		t.Fatalf("策略往返不符,got %+v", got)
	}
	if _, ok := got["low"]; ok {
		t.Fatalf("空动作名应被剔除,不入库,got %+v", got)
	}

	// 非法:未知等级 / 目录外动作 → 拒
	if err := svc.SetPolicy("register", map[string]string{"nope": "block"}); err == nil {
		t.Fatal("未知风险等级应被拒")
	}
	if err := svc.SetPolicy("register", map[string]string{"critical": "captcha"}); err == nil {
		t.Fatal("目录外动作名应被拒")
	}
}
