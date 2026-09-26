package core

import (
	"fmt"
	"sort"

	"github.com/shyandsy/aurora/feature/doorman/model/dto"
)

// ActionKind 动作类型:决定要不要追踪后续(漏斗)。doorman 只据它决定"这决策要不要回填结果",不解释动作含义。
type ActionKind string

const (
	// ActionTerminal 硬卡:当场终结(如直接拦截 / 放行),无后续,只记一笔,不进漏斗。
	ActionTerminal ActionKind = "terminal"
	// ActionFriction 减速器:施加一道人工阻力(如邮件激活 / 2FA / OTP),有后续(用户完成 or 放弃)→ 进「完成漏斗」。
	ActionFriction ActionKind = "friction"
)

// Valid 是否合法动作类型。
func (k ActionKind) Valid() bool { return k == ActionTerminal || k == ActionFriction }

// ActionDef 一个动作的定义:名字(与业务代码约定的契约)+ 展示标签 + 类型。doorman 只存/传,不执行。
type ActionDef struct {
	Name  string
	Label string
	Kind  ActionKind
}

// ScopeDef 一个 scope 的定义:id(业务代码里 Assess 时用的字符串)+ 展示标签 + 该 scope 的动作目录。
type ScopeDef struct {
	ID      string
	Label   string
	Actions []ActionDef
}

// Registry 条件插件 + scope 定义注册表(scope→标签+动作目录+动作类型)。
//   - 条件插件:doorman 自带引擎评估(唯一"逻辑"插件)。
//   - scope 定义:**业务在接入时代码注册**的词汇表——哪些 scope、每个 scope 有哪些动作(名+标签+类型)。
//     doorman 不解释含义,只拿它:给配置页渲染 tab/「风险→动作」下拉、给保存规则/策略时校验 scope 与动作名合法、
//     据动作类型决定要不要追踪后续。名字必须和业务代码里的 Assess/动作执行一致(代码是权威,防"幻影 scope")。
type Registry struct {
	conds      map[string]Condition
	scopes     map[string]*ScopeDef // 按 id 查
	scopeOrder []string             // 注册顺序(给 tab 稳定排序)
}

// NewRegistry 建空注册表。通用内置条件用 RegisterBuiltins 注册;业务可再补自己的(WithCondition)。
func NewRegistry() *Registry {
	return &Registry{conds: map[string]Condition{}, scopes: map[string]*ScopeDef{}}
}

// RegisterCondition 注册一个条件类别(重名覆盖)。
func (r *Registry) RegisterCondition(c Condition) { r.conds[c.Type()] = c }

// RegisterScope 登记一个 scope 定义(标签 + 动作目录)。同 id 再登记则合并:非空 label 覆盖,动作按名去重追加。
// 动作 Label 空 → 回退到 Name;Kind 空/非法 → 默认 ActionTerminal(只记不追踪)。
func (r *Registry) RegisterScope(id, label string, actions []ActionDef) {
	if id == "" {
		return
	}
	sd, ok := r.scopes[id]
	if !ok {
		sd = &ScopeDef{ID: id}
		r.scopes[id] = sd
		r.scopeOrder = append(r.scopeOrder, id)
	}
	if label != "" {
		sd.Label = label
	}
	seen := map[string]bool{}
	for _, a := range sd.Actions {
		seen[a.Name] = true
	}
	for _, a := range actions {
		if a.Name == "" || seen[a.Name] {
			continue
		}
		if a.Label == "" {
			a.Label = a.Name
		}
		if !a.Kind.Valid() {
			a.Kind = ActionTerminal
		}
		sd.Actions = append(sd.Actions, a)
		seen[a.Name] = true
	}
}

// RegisterActions 向后兼容旧接口:只登记动作名(标签回退到名、类型默认 terminal),等价于登记一个只有动作的 scope。
func (r *Registry) RegisterActions(scope string, types []string) {
	defs := make([]ActionDef, 0, len(types))
	for _, t := range types {
		defs = append(defs, ActionDef{Name: t})
	}
	r.RegisterScope(scope, "", defs)
}

// ActionsFor 返回某 scope 登记的动作名清单(顺序即登记顺序;未登记返回空)。
func (r *Registry) ActionsFor(scope string) []string {
	sd, ok := r.scopes[scope]
	if !ok {
		return nil
	}
	out := make([]string, 0, len(sd.Actions))
	for _, a := range sd.Actions {
		out = append(out, a.Name)
	}
	return out
}

// HasAnyScope 是否登记过任何 scope(用于是否启用 scope 校验:没登记=最小接入,不强校验,向后兼容)。
func (r *Registry) HasAnyScope() bool { return len(r.scopeOrder) > 0 }

// HasScope 某 scope 是否已注册。
func (r *Registry) HasScope(scope string) bool { _, ok := r.scopes[scope]; return ok }

// ActionKindOf 查某 scope 某动作的类型;未找到返回 ("", false)。
func (r *Registry) ActionKindOf(scope, name string) (ActionKind, bool) {
	sd, ok := r.scopes[scope]
	if !ok {
		return "", false
	}
	for _, a := range sd.Actions {
		if a.Name == name {
			return a.Kind, true
		}
	}
	return "", false
}

// Scopes 按注册顺序返回全部 scope 定义(给配置页 /scopes 数据驱动)。
func (r *Registry) Scopes() []ScopeDef {
	out := make([]ScopeDef, 0, len(r.scopeOrder))
	for _, id := range r.scopeOrder {
		out = append(out, *r.scopes[id])
	}
	return out
}

// Compile 把一组规则编译(只取 enabled 且 scope 匹配的)。
// 非法规则(未知条件类别 / 参数非法 / 未知风险等级 / 组合非法 / 无条件)会被跳过并收进 errs——
// 供**保存时校验**(admin 存规则先编译一遍,有 err 就拒)与运行时日志共用同一套真相源。
func (r *Registry) Compile(rules []Rule, scope string) (compiled []*compiledRule, errs []error) {
	for _, rule := range rules {
		if !rule.Enabled || rule.Scope != scope {
			continue
		}
		cr, err := r.compileOne(rule)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		compiled = append(compiled, cr)
	}
	return compiled, errs
}

// compileOne 编译单条规则:校验风险等级 / 组合方式 / 至少一个条件,再逐个编译条件。
func (r *Registry) compileOne(rule Rule) (*compiledRule, error) {
	if !rule.RiskLevel.Valid() {
		return nil, fmt.Errorf("规则 %q:未知风险等级 %q", rule.Name, rule.RiskLevel)
	}
	combine := rule.Combine
	if combine == "" {
		combine = CombineAnd
	}
	if combine != CombineAnd && combine != CombineOr {
		return nil, fmt.Errorf("规则 %q:未知条件组合方式 %q", rule.Name, combine)
	}
	if len(rule.Conditions) == 0 {
		return nil, fmt.Errorf("规则 %q:至少需要一个条件", rule.Name)
	}
	checks := make([]Check, 0, len(rule.Conditions))
	for _, rc := range rule.Conditions {
		cond, ok := r.conds[rc.Type]
		if !ok {
			return nil, fmt.Errorf("规则 %q:未知条件类别 %q", rule.Name, rc.Type)
		}
		chk, err := cond.Compile(normParams(string(rc.Params)))
		if err != nil {
			return nil, fmt.Errorf("规则 %q:条件 %q 配置非法:%w", rule.Name, rc.Type, err)
		}
		checks = append(checks, chk)
	}
	return &compiledRule{name: rule.Name, level: rule.RiskLevel, combine: combine, checks: checks}, nil
}

// Validate 校验单条规则(用 compileOne 试编译)。供保存规则时用,与运行时同一套真相源。
func (r *Registry) Validate(rule Rule) error {
	_, err := r.compileOne(rule)
	return err
}

// ConditionKinds 列出所有已注册条件类别(按 Type 排序,稳定给前端)。返回对外元信息(dto.KindInfo)。
func (r *Registry) ConditionKinds() []dto.KindInfo {
	out := make([]dto.KindInfo, 0, len(r.conds))
	for _, c := range r.conds {
		out = append(out, dto.KindInfo{Type: c.Type(), Fields: c.Fields()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Type < out[j].Type })
	return out
}
