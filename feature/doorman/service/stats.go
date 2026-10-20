package service

// 本文件是 Console 的**观测面**:决策统计(各风险等级/各动作计数)、判定→激活漏斗、决策明细(翻页)。
// 与 rules.go / policy.go(配什么)分开:这边是"发生了什么"。都挂在 Console 接口上,同一套后端。

import (
	"time"

	"github.com/shyandsy/aurora/feature/doorman/model/dto"
	"github.com/shyandsy/aurora/feature/doorman/model/entity"
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

// 决策明细页码翻页的归一:默认页大小、上限。
const (
	defaultDecisionsPageSize = 20
	maxDecisionsPageSize     = 200
)

// ListDecisions 决策明细(id 倒序,新→旧);beforeID>0 游标翻页;limit<=0/过大时存储层归一。
func (a *console) ListDecisions(scope string, limit int, beforeID int64) ([]dto.DecisionDTO, error) {
	rows, err := a.store.decisions(scope, limit, beforeID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.DecisionDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, rowToDecisionDTO(r))
	}
	return out, nil
}

// ListDecisionsPage 决策明细页码翻页(id 倒序,新→旧;scope 空=全量)。
// page 从 1 起(<1 归一到 1);pageSize<=0 → 默认 20,>200 → 截到 200;offset=(page-1)*pageSize。
// 返回已归一的 page/pageSize + 该 scope 的总数,供前端渲染分页。
func (a *console) ListDecisionsPage(scope string, page, pageSize int) (dto.DecisionPageDTO, error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = defaultDecisionsPageSize
	}
	if pageSize > maxDecisionsPageSize {
		pageSize = maxDecisionsPageSize
	}
	total, err := a.store.countDecisions(scope)
	if err != nil {
		return dto.DecisionPageDTO{}, err
	}
	rows, err := a.store.decisionsPage(scope, (page-1)*pageSize, pageSize)
	if err != nil {
		return dto.DecisionPageDTO{}, err
	}
	items := make([]dto.DecisionDTO, 0, len(rows))
	for _, r := range rows {
		items = append(items, rowToDecisionDTO(r))
	}
	return dto.DecisionPageDTO{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// rowToDecisionDTO 决策实体行 → 对外 DTO(游标版 / 页码版共用)。
func rowToDecisionDTO(r entity.DecisionRow) dto.DecisionDTO {
	return dto.DecisionDTO{
		ID: r.ID, RiskLevel: r.RiskLevel, Action: r.Action, Matched: r.Matched,
		Subject: r.Subject, UA: r.UA, IP: r.IP, Country: r.Country, ISP: r.ISP,
		ASN: r.ASN, ASNOrg: r.ASNOrg, IsHosting: r.IsHosting, Outcome: r.Outcome, Created: r.Created,
	}
}
