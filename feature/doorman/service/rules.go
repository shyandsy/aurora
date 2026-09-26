package service

// 本文件是 Console 的**规则**面:规则增删改查 + DTO↔存储/引擎 映射。

import (
	"errors"

	"github.com/shyandsy/aurora/feature/doorman/core"
	"github.com/shyandsy/aurora/feature/doorman/model/dto"
	"github.com/shyandsy/aurora/feature/doorman/model/entity"
)

// ErrRuleNotFound 更新/删除不存在的规则。
var ErrRuleNotFound = errors.New("doorman: rule not found")

func (a *console) ListRules(scope string) ([]dto.RuleDTO, error) {
	rows, err := a.store.listRows(scope)
	if err != nil {
		return nil, err
	}
	out := make([]dto.RuleDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, rowToDTO(r))
	}
	return out, nil
}

func (a *console) UpsertRule(in dto.RuleDTO) (*dto.RuleDTO, error) {
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
	out := rowToDTO(row)
	return &out, nil
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

// dtoConditions 把 ConditionDTO(params 是任意 JSON 对象)转成引擎用的 core.RuleCondition(params 是 RawMessage)。
func dtoConditions(in []dto.ConditionDTO) []core.RuleCondition {
	out := make([]core.RuleCondition, 0, len(in))
	for _, c := range in {
		out = append(out, core.RuleCondition{Type: c.Type, Params: marshalAny(c.Params)})
	}
	return out
}

func dtoToRule(d dto.RuleDTO) core.Rule {
	combine := d.Combine
	if combine == "" {
		combine = core.CombineAnd
	}
	return core.Rule{
		Name: d.Name, Enabled: d.Enabled, Scope: d.Scope,
		Conditions: dtoConditions(d.Conditions),
		Combine:    combine,
		RiskLevel:  core.RiskLevel(d.RiskLevel),
	}
}

func dtoToRow(d dto.RuleDTO) entity.RuleRow {
	combine := d.Combine
	if combine == "" {
		combine = core.CombineAnd
	}
	return entity.RuleRow{
		ID: d.ID, Scope: d.Scope, Name: d.Name,
		Conditions: marshalConditions(dtoConditions(d.Conditions)),
		Combine:    combine,
		RiskLevel:  d.RiskLevel,
		Enabled:    d.Enabled,
	}
}

func rowToDTO(r entity.RuleRow) dto.RuleDTO {
	conds := parseConditions(r.Conditions)
	out := make([]dto.ConditionDTO, 0, len(conds))
	for _, c := range conds {
		out = append(out, dto.ConditionDTO{Type: c.Type, Params: rawToAny(c.Params)})
	}
	return dto.RuleDTO{
		ID: r.ID, Scope: r.Scope, Name: r.Name,
		Conditions: out, Combine: r.Combine, RiskLevel: r.RiskLevel,
		Enabled: r.Enabled, Created: r.Created, Modified: r.Modified,
	}
}
