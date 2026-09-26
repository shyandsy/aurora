// Package dto 是 doorman 管理 API / 配置页的对外读写模型(纯数据,叶子包,不引领域类型)。
// 风险等级用 string(枚举 RiskLevel 是领域类型,留 core;DTO 只做序列化)。
package dto

import "time"

// Field 一个配置字段的描述(给配置页动态渲染表单;新条件类别自带自己的字段)。
type Field struct {
	Key      string   `json:"key"`
	Type     string   `json:"type"` // 取值见 core 的 Field* 常量
	LabelKey string   `json:"labelKey"`
	Required bool     `json:"required,omitempty"`
	Options  []string `json:"options,omitempty"` // select 类型的候选
}

// KindInfo 一个条件类别的元信息(类别名 + 配置字段),给配置页渲染下拉与动态表单。
type KindInfo struct {
	Type   string  `json:"type"`
	Fields []Field `json:"fields"`
}

// ConditionDTO 规则里一个条件项的对外读写模型。params 是 JSON 对象(不是字符串)。
type ConditionDTO struct {
	Type   string `json:"type"`
	Params any    `json:"params,omitempty"`
}

// RuleDTO 规则的对外读写模型。不含「动作」——动作在业务侧据 riskLevel 映射。
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

// KindsDTO 「有哪些条件类别 + 各自配置字段」与「风险等级枚举」的 schema。
type KindsDTO struct {
	Conditions []KindInfo `json:"conditions"`
	RiskLevels []string   `json:"riskLevels"`
}

// PolicyKindsDTO 「风险→动作」策略配置页所需的选项:风险等级枚举 + 该 scope 可选的动作名清单。
type PolicyKindsDTO struct {
	RiskLevels []string `json:"riskLevels"`
	Actions    []string `json:"actions"` // 该 scope 登记的动作名(下拉候选);空 = 没登记动作
}

// ActionDTO 一个动作的对外定义(渲染「风险→动作」下拉、按 kind 决定漏斗)。
type ActionDTO struct {
	Name  string `json:"name"`
	Label string `json:"label"`
	Kind  string `json:"kind"` // friction(减速器,进漏斗)/ terminal(硬卡,只记一笔)
}

// ScopeDTO 一个 scope 的对外定义(配置页 tab + 动作目录)。前端据此数据驱动。
type ScopeDTO struct {
	ID      string      `json:"id"`
	Label   string      `json:"label"`
	Actions []ActionDTO `json:"actions"`
}

// StatsDTO 门禁决策统计(某 scope 近 N 天):按风险等级、按动作名各一份计数 + 总数 + 「判定→激活」漏斗。
type StatsDTO struct {
	SinceDays int              `json:"sinceDays"`
	Total     int64            `json:"total"`
	ByRisk    map[string]int64 `json:"byRisk"`
	ByAction  map[string]int64 `json:"byAction"`
	Funnel    FunnelDTO        `json:"funnel"`
}

// FunnelDTO 「判定→激活」漏斗:分母 = 判后有后续的决策(有 subject),Resolved = 已回填结果。未完成 = 两者之差。
type FunnelDTO struct {
	Challenged int64 `json:"challenged"`
	Resolved   int64 `json:"resolved"`
}

// DecisionDTO 一条决策明细。完成回填后 subject 及各网络事实会被清空,只剩计数所需字段。
type DecisionDTO struct {
	ID        int64     `json:"id"`
	RiskLevel string    `json:"riskLevel"`
	Action    string    `json:"action"`
	Matched   string    `json:"matched"`
	Subject   string    `json:"subject,omitempty"`
	UA        string    `json:"ua,omitempty"`
	IP        string    `json:"ip,omitempty"`
	Country   string    `json:"country,omitempty"`
	ISP       string    `json:"isp,omitempty"`
	ASN       uint      `json:"asn,omitempty"`
	ASNOrg    string    `json:"asnOrg,omitempty"`
	IsHosting bool      `json:"isHosting"`
	Outcome   string    `json:"outcome,omitempty"`
	Created   time.Time `json:"created"`
}
