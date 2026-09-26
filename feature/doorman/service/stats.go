package service

// 本文件是 Console 的**观测面**:决策统计(各风险等级/各动作计数)、判定→激活漏斗、决策明细(翻页)。
// 与 rules.go / policy.go(配什么)分开:这边是"发生了什么"。都挂在 Console 接口上,同一套后端。

import (
	"time"

	"github.com/shyandsy/aurora/feature/doorman/model/dto"
)

// Stats 近 days 天(<=0 → 7)的决策统计:各风险等级/各动作计数 + 判定→激活漏斗。
func (a *console) Stats(scope string, days int) (dto.StatsDTO, error) {
	if days <= 0 {
		days = 7
	}
	since := time.Now().AddDate(0, 0, -days)
	byRisk, byAction, err := a.store.stats(scope, since)
	if err != nil {
		return dto.StatsDTO{}, err
	}
	var total int64
	for _, n := range byRisk {
		total += n
	}
	// 「判定→结果」漏斗:判要激活(有 subject)为分母,已回填结果(outcome 非空)= 已完成。
	// 未完成 = 分母 − 已完成,展示侧算(不猜时间:回填了就是完成,没回填就是没完成)。
	fc, err := a.store.funnel(scope, since)
	if err != nil {
		return dto.StatsDTO{}, err
	}
	return dto.StatsDTO{
		SinceDays: days, Total: total, ByRisk: byRisk, ByAction: byAction,
		Funnel: dto.FunnelDTO{Challenged: fc.Challenged, Resolved: fc.Resolved},
	}, nil
}

// ListDecisions 决策明细(id 倒序,新→旧);beforeID>0 游标翻页;limit<=0/过大时存储层归一。
func (a *console) ListDecisions(scope string, limit int, beforeID int64) ([]dto.DecisionDTO, error) {
	rows, err := a.store.decisions(scope, limit, beforeID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.DecisionDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, dto.DecisionDTO{
			ID: r.ID, RiskLevel: r.RiskLevel, Action: r.Action, Matched: r.Matched,
			Subject: r.Subject, UA: r.UA, IP: r.IP, Country: r.Country, ISP: r.ISP,
			ASN: r.ASN, ASNOrg: r.ASNOrg, IsHosting: r.IsHosting, Outcome: r.Outcome, Created: r.Created,
		})
	}
	return out, nil
}
