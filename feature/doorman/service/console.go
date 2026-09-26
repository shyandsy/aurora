package service

// 本文件是 doorman 的**管理面**(Console):接口 + schema(条件类别 / 已注册 scope)。
// 规则增删改在 rules.go;「风险→动作」策略在 policy.go;观测(统计 / 漏斗 / 明细)在 stats.go。
// 校验走 core.Registry,与运行时同一真相源;对外读写模型统一在 model/dto。HTTP 路由在 controller 子包。

import (
	"fmt"

	"github.com/shyandsy/aurora/feature/doorman/core"
	"github.com/shyandsy/aurora/feature/doorman/model/dto"
)

// Console 规则 + 策略管理 + 观测(配置页后端)。校验走 Registry,与运行时同一真相源。
type Console interface {
	Kinds() dto.KindsDTO
	Scopes() []dto.ScopeDTO // 已注册的 scope 定义(id+标签+动作目录);配置页数据驱动 tab/下拉
	ListRules(scope string) ([]dto.RuleDTO, error)
	UpsertRule(in dto.RuleDTO) (*dto.RuleDTO, error) // ID==0 新增,否则更新;参数非法/类别未知/scope 未注册 → error
	DeleteRule(id int64) error
	// ── 「风险等级 → 动作」策略 ──
	PolicyKinds(scope string) dto.PolicyKindsDTO             // 配置页渲染:风险等级 + 该 scope 可选动作名
	GetPolicy(scope string) (map[string]string, error)       // 当前映射 {风险等级: 动作名}
	SetPolicy(scope string, mapping map[string]string) error // 整体覆盖;校验等级/动作名合法
	// ── 统计 + 明细 ──
	Stats(scope string, days int) (dto.StatsDTO, error)                               // 近 days 天(<=0 默认 7)的决策汇总
	ListDecisions(scope string, limit int, beforeID int64) ([]dto.DecisionDTO, error) // 决策明细(id 倒序,游标翻页)
}

type console struct {
	reg   *core.Registry
	store consoleStore
}

// newConsole 建管理服务(配置页后端)。
func newConsole(reg *core.Registry, store consoleStore) Console {
	return &console{reg: reg, store: store}
}

func (a *console) Kinds() dto.KindsDTO {
	return dto.KindsDTO{Conditions: a.reg.ConditionKinds(), RiskLevels: core.RiskLevelStrings()}
}

// Scopes 返回已注册的 scope 定义(id+标签+动作目录),配置页据此数据驱动 tab 与「风险→动作」下拉。
func (a *console) Scopes() []dto.ScopeDTO {
	defs := a.reg.Scopes()
	out := make([]dto.ScopeDTO, 0, len(defs))
	for _, s := range defs {
		acts := make([]dto.ActionDTO, 0, len(s.Actions))
		for _, act := range s.Actions {
			acts = append(acts, dto.ActionDTO{Name: act.Name, Label: act.Label, Kind: string(act.Kind)})
		}
		out = append(out, dto.ScopeDTO{ID: s.ID, Label: s.Label, Actions: acts})
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
