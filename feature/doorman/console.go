package doorman

// 本文件是 doorman 的**管理面**(Console)核心:接口 + schema(条件类别/已注册 scope)。
// 规则增删改在 rules.go;「风险→动作」策略在 policy.go;观测(统计/漏斗/明细)在 stats.go;HTTP 路由在 controller 子包。

import "fmt"

// KindsDTO 是「有哪些条件类别 + 各自配置字段」与「风险等级枚举」的 schema,给配置页动态渲染表单/下拉。
type KindsDTO struct {
	Conditions []KindInfo  `json:"conditions"`
	RiskLevels []RiskLevel `json:"riskLevels"`
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

// Console 规则 + 策略管理 + 观测(配置页后端)。校验走 Registry,与运行时同一真相源。
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
// 挡住"给未注册 scope 配规则/策略"——那种配置永远不会被运行时消费(幻影 scope)。rules.go / policy.go 共用。
func (a *console) requireScope(scope string) error {
	if a.reg.HasAnyScope() && !a.reg.HasScope(scope) {
		return fmt.Errorf("scope %q 未注册(该 scope 没有代码消费,配了也不会生效)", scope)
	}
	return nil
}
