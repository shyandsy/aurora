package doorman

// 本文件是 doorman 的**管理面**:规则(增删改查)+「风险→动作」策略读写的 DTO 与 Console 实现,
// 供配置页后端调用。观测面(统计/漏斗/决策明细)在 stats.go;HTTP 路由在 controller.go。
import (
	"errors"
	"fmt"
	"time"
)

// ConditionDTO 是规则里一个条件项的对外读写模型。params 是 JSON 对象(不是字符串)。
type ConditionDTO struct {
	Type   string `json:"type"`
	Params any    `json:"params,omitempty"`
}

// RuleDTO 是规则的对外读写模型(admin API / 配置页用)。不含「动作」——动作在业务侧据 riskLevel 映射。
type RuleDTO struct {
	ID         int64          `json:"id"`
	Scope      string         `json:"scope"`
	Name       string         `json:"name"`
	Conditions []ConditionDTO `json:"conditions"`
	Combine    string         `json:"combine"`   // and / or
	RiskLevel  string         `json:"riskLevel"` // none/low/medium/high/critical
	Enabled    bool           `json:"enabled"`
	Created    time.Time      `json:"created,omitempty"`
	Modified   time.Time      `json:"modified,omitempty"`
}

// KindsDTO 是「有哪些条件类别 + 各自配置字段」与「风险等级枚举」的 schema,给配置页动态渲染表单/下拉。
type KindsDTO struct {
	Conditions []KindInfo  `json:"conditions"`
	RiskLevels []RiskLevel `json:"riskLevels"`
}

// ErrRuleNotFound 更新/删除不存在的规则。
var ErrRuleNotFound = errors.New("doorman: rule not found")

// PolicyKindsDTO 是「风险→动作」策略配置页所需的选项:风险等级枚举 + 该 scope 可选的动作名清单。
type PolicyKindsDTO struct {
	RiskLevels []RiskLevel `json:"riskLevels"`
	Actions    []string    `json:"actions"` // 该 scope 登记的动作名(下拉候选);空 = 该 scope 没登记动作
}

// ActionDTO 一个动作的对外定义(配置页数据驱动:渲染「风险→动作」下拉、按 kind 决定漏斗)。
type ActionDTO struct {
	Name  string `json:"name"`
	Label string `json:"label"`
	Kind  string `json:"kind"` // friction(减速器,进漏斗)/ terminal(硬卡,只记一笔)
}

// ScopeDTO 一个 scope 的对外定义(配置页 tab + 动作目录)。前端据此数据驱动,不再硬编码 scope/动作标签。
type ScopeDTO struct {
	ID      string      `json:"id"`
	Label   string      `json:"label"`
	Actions []ActionDTO `json:"actions"`
}

// Console 规则 + 策略管理(配置页后端)。校验走 Registry,与运行时同一真相源。
type Console interface {
	Kinds() KindsDTO
	Scopes() []ScopeDTO // 已注册的 scope 定义(id+标签+动作目录);配置页数据驱动 tab/下拉
	ListRules(scope string) ([]RuleDTO, error)
	UpsertRule(in RuleDTO) (*RuleDTO, error) // ID==0 新增,否则更新;参数非法/类别未知/scope 未注册 → error
	DeleteRule(id int64) error
	// ── 「风险等级 → 动作」策略 ──
	PolicyKinds(scope string) PolicyKindsDTO                 // 配置页渲染:风险等级 + 该 scope 可选动作名
	GetPolicy(scope string) (map[string]string, error)       // 当前映射 {风险等级: 动作名}
	SetPolicy(scope string, mapping map[string]string) error // 整体覆盖;校验等级/动作名合法
	// ── 统计 + 明细 ──
	Stats(scope string, days int) (StatsDTO, error)                               // 近 days 天(<=0 默认 7)的决策汇总
	ListDecisions(scope string, limit int, beforeID int64) ([]DecisionDTO, error) // 决策明细(id 倒序,游标翻页)
}

type console struct {
	reg   *Registry
	store ruleStore
}

func newConsole(reg *Registry, store ruleStore) Console {
	return &console{reg: reg, store: store}
}

func (a *console) Kinds() KindsDTO {
	return KindsDTO{Conditions: a.reg.ConditionKinds(), RiskLevels: RiskLevels}
}

// Scopes 返回已注册的 scope 定义(id+标签+动作目录),配置页据此数据驱动 tab 与「风险→动作」下拉。
func (a *console) Scopes() []ScopeDTO {
	defs := a.reg.Scopes()
	out := make([]ScopeDTO, 0, len(defs))
	for _, s := range defs {
		acts := make([]ActionDTO, 0, len(s.Actions))
		for _, act := range s.Actions {
			acts = append(acts, ActionDTO{Name: act.Name, Label: act.Label, Kind: string(act.Kind)})
		}
		out = append(out, ScopeDTO{ID: s.ID, Label: s.Label, Actions: acts})
	}
	return out
}

// requireScope 校验 scope 已注册。仅当注册过任意 scope 时才强校验(没注册=最小接入,向后兼容不拦)。
// 挡住"给未注册 scope 配规则/策略"——那种配置永远不会被运行时消费(幻影 scope)。
func (a *console) requireScope(scope string) error {
	if a.reg.HasAnyScope() && !a.reg.HasScope(scope) {
		return fmt.Errorf("scope %q 未注册(该 scope 没有代码消费,配了也不会生效)", scope)
	}
	return nil
}

func (a *console) ListRules(scope string) ([]RuleDTO, error) {
	rows, err := a.store.listRows(scope)
	if err != nil {
		return nil, err
	}
	out := make([]RuleDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, rowToDTO(r))
	}
	return out, nil
}

func (a *console) UpsertRule(in RuleDTO) (*RuleDTO, error) {
	// scope 必须已注册(否则是永不生效的幻影配置);再用运行时同一套校验条件/参数/风险等级/组合。挡在存库前。
	if err := a.requireScope(in.Scope); err != nil {
		return nil, err
	}
	if err := a.reg.Validate(dtoToRule(in)); err != nil {
		return nil, err
	}
	row := dtoToRow(in)
	if row.ID != 0 {
		existing, err := a.store.getRow(row.ID)
		if err != nil {
			return nil, err
		}
		if existing == nil {
			return nil, ErrRuleNotFound
		}
		row.Created = existing.Created // 保留创建时间
	}
	if err := a.store.upsertRow(&row); err != nil {
		return nil, err
	}
	dto := rowToDTO(row)
	return &dto, nil
}

func (a *console) DeleteRule(id int64) error {
	existing, err := a.store.getRow(id)
	if err != nil {
		return err
	}
	if existing == nil {
		return ErrRuleNotFound
	}
	return a.store.deleteRow(id)
}

// ── 「风险等级 → 动作」策略 ──

func (a *console) PolicyKinds(scope string) PolicyKindsDTO {
	return PolicyKindsDTO{RiskLevels: RiskLevels, Actions: a.reg.ActionsFor(scope)}
}

func (a *console) GetPolicy(scope string) (map[string]string, error) {
	return a.store.Policy(scope)
}

// SetPolicy 整体覆盖某 scope 的策略。校验:风险等级必须合法;动作名必须在该 scope 登记的动作目录内
// (与运行时同源,挡在存库前)。动作名为空串视为"该等级不配"(从映射里剔除)。
func (a *console) SetPolicy(scope string, mapping map[string]string) error {
	if err := a.requireScope(scope); err != nil {
		return err
	}
	allowed := map[string]bool{}
	for _, t := range a.reg.ActionsFor(scope) {
		allowed[t] = true
	}
	clean := make(map[string]string, len(mapping))
	for level, action := range mapping {
		if !RiskLevel(level).Valid() {
			return fmt.Errorf("未知风险等级 %q", level)
		}
		if action == "" {
			continue // 空 = 该等级不配(回退业务默认)
		}
		if !allowed[action] {
			return fmt.Errorf("scope %q 不支持动作 %q", scope, action)
		}
		clean[level] = action
	}
	return a.store.setPolicy(scope, clean)
}

// ── DTO ↔ 存储/引擎 映射 ──

// dtoConditions 把 ConditionDTO(params 是任意 JSON 对象)转成引擎用的 RuleCondition(params 是 RawMessage)。
func dtoConditions(in []ConditionDTO) []RuleCondition {
	out := make([]RuleCondition, 0, len(in))
	for _, c := range in {
		out = append(out, RuleCondition{Type: c.Type, Params: marshalAny(c.Params)})
	}
	return out
}

func dtoToRule(d RuleDTO) Rule {
	combine := d.Combine
	if combine == "" {
		combine = CombineAnd
	}
	return Rule{
		Name: d.Name, Enabled: d.Enabled, Scope: d.Scope,
		Conditions: dtoConditions(d.Conditions),
		Combine:    combine,
		RiskLevel:  RiskLevel(d.RiskLevel),
	}
}

func dtoToRow(d RuleDTO) ruleRow {
	combine := d.Combine
	if combine == "" {
		combine = CombineAnd
	}
	return ruleRow{
		ID: d.ID, Scope: d.Scope, Name: d.Name,
		Conditions: marshalConditions(dtoConditions(d.Conditions)),
		Combine:    combine,
		RiskLevel:  d.RiskLevel,
		Enabled:    d.Enabled,
	}
}

func rowToDTO(r ruleRow) RuleDTO {
	conds := parseConditions(r.Conditions)
	out := make([]ConditionDTO, 0, len(conds))
	for _, c := range conds {
		out = append(out, ConditionDTO{Type: c.Type, Params: rawToAny(c.Params)})
	}
	return RuleDTO{
		ID: r.ID, Scope: r.Scope, Name: r.Name,
		Conditions: out, Combine: r.Combine, RiskLevel: r.RiskLevel,
		Enabled: r.Enabled, Created: r.Created, Modified: r.Modified,
	}
}
