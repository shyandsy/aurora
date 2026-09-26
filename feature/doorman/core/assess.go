package core

import (
	"encoding/json"
	"strings"
)

// 条件组合方式。
const (
	CombineAnd = "and" // 全部条件命中(默认)
	CombineOr  = "or"  // 任一条件命中
)

// RuleCondition 规则里的一个条件项:类别 + 该类别的私有 JSON 配置(由对应插件 Compile 解析)。
type RuleCondition struct {
	Type   string          `json:"type"`             // 条件类别,如 "ua_match"
	Params json.RawMessage `json:"params,omitempty"` // 该条件的私有配置
}

// Rule 一条配置规则(来自 RuleSource):某 scope 下「多个条件(且/或)命中 → 给出一个风险等级」。
// 不含任何「动作」——动作由业务侧据风险等级映射。存储层通用,加条件类别不改表。
type Rule struct {
	Name       string          // 规则名(审计 / 报错定位)
	Enabled    bool            // 关掉的规则不参与评估
	Scope      string          // 场景:"register" / "login" / …
	Conditions []RuleCondition // 多个条件
	Combine    string          // 条件组合:CombineAnd(默认)/ CombineOr
	RiskLevel  RiskLevel       // 命中后给出的风险等级
}

// compiledRule 编译后的规则(条件都已按各自参数编好 Check)。
type compiledRule struct {
	name    string
	level   RiskLevel
	combine string
	checks  []Check
}

// match 按 combine 组合评估本规则的条件。无条件视为不命中(空规则不误伤)。
func (r *compiledRule) match(g *Context) bool {
	if len(r.checks) == 0 {
		return false
	}
	if r.combine == CombineOr {
		for _, c := range r.checks {
			if c(g) {
				return true
			}
		}
		return false
	}
	// 默认 and:全部命中才算命中。
	for _, c := range r.checks {
		if !c(g) {
			return false
		}
	}
	return true
}

// Assess 过所有规则,收集命中的,取**最高**风险等级。无命中 → RiskNone。
// 无优先级、无短路:每条规则各自判命中,doorman 不做任何处置——处置由业务据返回的 Level 决定。
func Assess(gctx *Context, rules []*compiledRule) Assessment {
	res := Assessment{Level: RiskNone}
	for _, r := range rules {
		if !r.match(gctx) {
			continue
		}
		res.Matched = append(res.Matched, MatchedRule{Name: r.name, Level: r.level})
		if r.level.Rank() > res.Level.Rank() {
			res.Level = r.level
		}
	}
	return res
}

// normParams 把空参数归一成 JSON null,避免 json.Unmarshal 空串报错(插件按需自校验)。
func normParams(s string) json.RawMessage {
	if strings.TrimSpace(s) == "" {
		return json.RawMessage("null")
	}
	return json.RawMessage(s)
}
