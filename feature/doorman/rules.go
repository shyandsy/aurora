package doorman

// 本文件是 Console 的**规则**面:规则增删改查 + 规则 DTO + DTO↔存储/引擎 映射。

import (
	"errors"
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

// ErrRuleNotFound 更新/删除不存在的规则。
var ErrRuleNotFound = errors.New("doorman: rule not found")

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
