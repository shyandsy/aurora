package doorman

// 本文件是 doorman 的**观测面**:决策统计(各风险等级/各动作计数)、判定→激活漏斗、决策明细(翻页)。
// 与 manage.go(规则+策略的增删改查管理)分开:那边是"配什么",这边是"发生了什么"。
// 都挂在 Console 接口上(配置页后端同一套),实现方法在此。

import "time"

// StatsDTO 门禁决策统计(某 scope 近 N 天):按风险等级、按动作名各一份计数 + 总数 + 「判定→激活」漏斗。
type StatsDTO struct {
	SinceDays int              `json:"sinceDays"`
	Total     int64            `json:"total"`
	ByRisk    map[string]int64 `json:"byRisk"`   // 风险等级 → 命中数
	ByAction  map[string]int64 `json:"byAction"` // 动作名 → 数
	Funnel    FunnelDTO        `json:"funnel"`   // 判定→激活漏斗(只统计判后有后续的决策,如判要激活的)
}

// FunnelDTO 「判定→激活」漏斗:分母 = 判后有后续的决策(有 subject,如判要激活),
// Resolved = 已回填结果(如激活成功)。未完成 = Challenged − Resolved(展示侧算,不猜时间)。
// doorman 中性:只按「有没有 subject / 有没有回填 outcome」分桶,「激活/未激活」的语义由业务侧贴标签。
type FunnelDTO struct {
	Challenged int64 `json:"challenged"` // 需后续(有 subject),如判要激活
	Resolved   int64 `json:"resolved"`   // 已回填结果,如激活成功
}

// DecisionDTO 一条决策明细。字段 = 时间/命中规则/风险/动作 + 标识(subject)+ 网络事实(UA·IP·国家·运营商·ASN·机房)
// + 后续结果(outcome)。完成回填后 subject 及各网络事实会被清空(见 store.markOutcome),只剩计数所需字段。
type DecisionDTO struct {
	ID        int64     `json:"id"`
	RiskLevel string    `json:"riskLevel"`
	Action    string    `json:"action"`
	Matched   string    `json:"matched"`           // 命中规则名,逗号分隔
	Subject   string    `json:"subject,omitempty"` // 关联键/标识(注册场景=邮箱);完成回填后清空
	UA        string    `json:"ua,omitempty"`
	IP        string    `json:"ip,omitempty"`
	Country   string    `json:"country,omitempty"`
	ISP       string    `json:"isp,omitempty"` // 运营商(仅国内有)
	ASN       uint      `json:"asn,omitempty"`
	ASNOrg    string    `json:"asnOrg,omitempty"`
	IsHosting bool      `json:"isHosting"`
	Outcome   string    `json:"outcome,omitempty"` // 后续结果(如 "activated");空 = 无后续 / 判了要激活但未完成
	Created   time.Time `json:"created"`
}

// Stats 近 days 天(<=0 → 7)的决策统计:各风险等级/各动作计数 + 判定→激活漏斗。
func (a *console) Stats(scope string, days int) (StatsDTO, error) {
	if days <= 0 {
		days = 7
	}
	since := time.Now().AddDate(0, 0, -days)
	byRisk, byAction, err := a.store.stats(scope, since)
	if err != nil {
		return StatsDTO{}, err
	}
	var total int64
	for _, n := range byRisk {
		total += n
	}
	// 「判定→结果」漏斗:判要激活(有 subject)为分母,已回填结果(outcome 非空)= 已完成。
	// 未完成 = 分母 − 已完成,展示侧算(不猜时间:回填了就是完成,没回填就是没完成)。
	fc, err := a.store.funnel(scope, since)
	if err != nil {
		return StatsDTO{}, err
	}
	return StatsDTO{
		SinceDays: days, Total: total, ByRisk: byRisk, ByAction: byAction,
		Funnel: FunnelDTO{Challenged: fc.Challenged, Resolved: fc.Resolved},
	}, nil
}

// ListDecisions 决策明细(id 倒序,新→旧);beforeID>0 游标翻页;limit<=0/过大时存储层归一。
func (a *console) ListDecisions(scope string, limit int, beforeID int64) ([]DecisionDTO, error) {
	rows, err := a.store.decisions(scope, limit, beforeID)
	if err != nil {
		return nil, err
	}
	out := make([]DecisionDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, DecisionDTO{
			ID: r.ID, RiskLevel: r.RiskLevel, Action: r.Action, Matched: r.Matched,
			Subject: r.Subject, UA: r.UA, IP: r.IP, Country: r.Country, ISP: r.ISP,
			ASN: r.ASN, ASNOrg: r.ASNOrg, IsHosting: r.IsHosting, Outcome: r.Outcome, Created: r.Created,
		})
	}
	return out, nil
}
