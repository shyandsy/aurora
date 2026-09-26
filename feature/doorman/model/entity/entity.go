// Package entity 是 doorman 的持久化实体(gorm 行结构),中性、doorman_ 前缀。
package entity

import "time"

// RuleRow 规则的持久化模型(表 doorman_rule)。conditions 以 JSON 文本存;加条件类别不改表结构。
// 不存「动作」——动作在业务侧据 risk_level 映射。
type RuleRow struct {
	ID         int64     `gorm:"column:id;primaryKey;autoIncrement"`
	Scope      string    `gorm:"column:scope;size:64;index:idx_doorman_scope"`
	Name       string    `gorm:"column:name;size:128"`
	Conditions string    `gorm:"column:conditions;type:text"` // JSON 数组 [{type,params}]
	Combine    string    `gorm:"column:combine;size:8;not null;default:and"`
	RiskLevel  string    `gorm:"column:risk_level;size:16;not null;default:none"`
	Enabled    bool      `gorm:"column:enabled;not null;default:0"`
	Created    time.Time `gorm:"column:created"`
	Modified   time.Time `gorm:"column:modified"`
}

// TableName 固定表名(中性,无业务前缀)。
func (RuleRow) TableName() string { return "doorman_rule" }

// ActionPolicyRow 「风险等级 → 动作名」映射的持久化模型(表 doorman_action_policy)。
// 每 (scope, risk_level) 一行 → 一个动作名。action 是**业务约定的字符串**,doorman 只存不解释。
type ActionPolicyRow struct {
	Scope     string    `gorm:"column:scope;size:64;primaryKey"`
	RiskLevel string    `gorm:"column:risk_level;size:16;primaryKey"`
	Action    string    `gorm:"column:action;size:64;not null"`
	Modified  time.Time `gorm:"column:modified"`
}

// TableName 固定表名。
func (ActionPolicyRow) TableName() string { return "doorman_action_policy" }

// DecisionRow 一次门禁决策的流水(表 doorman_decision):业务每次评估后记一行,供统计 + 明细。
// 中性:存的都是 Attempt 上的事实 + 命中规则名 + 风险等级 + 动作名 + 时间;doorman 不解释动作名。
type DecisionRow struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement"`
	Scope     string    `gorm:"column:scope;size:64;index:idx_doorman_decision_scope_id,priority:1;index:idx_doorman_decision_scope_subject,priority:1"`
	RiskLevel string    `gorm:"column:risk_level;size:16"`
	Action    string    `gorm:"column:action;size:64"`
	Matched   string    `gorm:"column:matched;size:512"` // 命中规则名,逗号分隔(无命中为空)
	UA        string    `gorm:"column:ua;size:512"`
	IP        string    `gorm:"column:ip;size:45"`
	Country   string    `gorm:"column:country;size:8"`
	ISP       string    `gorm:"column:isp;size:64"` // 运营商(仅国内 geoip 有);空=未知
	ASN       uint      `gorm:"column:asn"`
	ASNOrg    string    `gorm:"column:asn_org;size:128"`
	IsHosting bool      `gorm:"column:is_hosting"`
	Created   time.Time `gorm:"column:created;index"`
	// Challenged 标记「判后有后续、需回填结果」(如判要激活)。业务设了 Subject 即为 true。
	// 单独立布尔:回填(完成)后会清掉 Subject 等 PII,但漏斗分母仍要数得到它。
	Challenged bool `gorm:"column:challenged"`
	// Subject 业务给的关联键 + 标识(注册场景=规整邮箱):只在需回填的决策上设。回填成功后清空(不囤 PII)。
	Subject string `gorm:"column:subject;size:191;index:idx_doorman_decision_scope_subject,priority:2"`
	// Outcome 业务回填的不透明结果串(如 "activated"):空 = 未回填 = 判了要激活但没完成。
	Outcome string `gorm:"column:outcome;size:32"`
}

// TableName 固定表名。
func (DecisionRow) TableName() string { return "doorman_decision" }
