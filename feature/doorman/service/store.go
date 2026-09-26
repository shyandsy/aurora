package service

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/shyandsy/aurora/feature/doorman/core"
	"github.com/shyandsy/aurora/feature/doorman/model/entity"
	"gorm.io/gorm"
)

// consoleStore 是 Console 依赖的存储抽象(dbStore 实现;单测可替身)。
// 含规则只读 + 增删改、策略(风险→动作)读写、决策统计聚合;运行时契约(Rules/Policy/Record)由 core 定义。
type consoleStore interface {
	core.PolicyStore
	listRows(scope string) ([]entity.RuleRow, error) // scope 空 = 全部
	getRow(id int64) (*entity.RuleRow, error)
	upsertRow(row *entity.RuleRow) error
	deleteRow(id int64) error
	// setPolicy 用给定映射**整体覆盖**某 scope 的策略(删该 scope 旧行,再写新行;空 map = 清空该 scope)。
	setPolicy(scope string, mapping map[string]string) error
	// stats 统计某 scope 自 since 起的决策:按风险等级、按动作名各出一份计数。
	stats(scope string, since time.Time) (byRisk map[string]int64, byAction map[string]int64, err error)
	// funnel 统计某 scope 自 since 起「有后续结果的决策」的漏斗:有 subject 的为分母(判要激活),
	// 其中 outcome 非空 = 已完成(激活)。未完成 = 分母 − 已完成(不猜时间,回填了就是完成、没回填就是没完成)。
	funnel(scope string, since time.Time) (funnelCounts, error)
	// decisions 取某 scope 的决策明细(按 id 倒序,新→旧);beforeID>0 时只取 id<beforeID(游标翻页)。
	decisions(scope string, limit int, beforeID int64) ([]entity.DecisionRow, error)
}

// dbStore 是基于 gorm 的默认存储:实现 core 的 RuleSource/PolicyStore/DecisionRecorder(运行时)
// 与 consoleStore(管理面),复用方只需注入 *gorm.DB。三张 doorman_* 表由宿主服务的 goose 建(不自 DDL)。
type dbStore struct {
	db *gorm.DB
}

// newDBStore 建 DB store。
func newDBStore(db *gorm.DB) *dbStore { return &dbStore{db: db} }

// funnelCounts 「判定→结果」漏斗计数(只统计有 subject 的决策)。未完成 = Challenged − Resolved(展示侧算)。
type funnelCounts struct {
	Challenged int64 // 有 subject(判定后需回填结果,如判要激活)
	Resolved   int64 // 其中 outcome 非空(业务回填了结果,如激活成功)
}

// ── core.DecisionRecorder ──

// Record 记一次门禁决策流水(供统计 + 明细)。best-effort:调用方吞错。core.Decision → 实体行。
func (s *dbStore) Record(d core.Decision) error {
	return s.db.Create(&entity.DecisionRow{
		Scope: d.Scope, RiskLevel: d.RiskLevel, Action: d.Action, Matched: d.Matched,
		UA: d.UA, IP: d.IP, Country: d.Country, ISP: d.ISP,
		ASN: d.ASN, ASNOrg: d.ASNOrg, IsHosting: d.IsHosting,
		Subject: d.Subject, Challenged: d.Challenged, Created: time.Now(),
	}).Error
}

// MarkOutcome 给某 (scope, subject) 最近一条尚未回填的决策写 outcome,并**清空 PII**
// (邮箱/UA/IP/运营商/ASN):合法用户已在业务库里,doorman 完成后不囤其明细,只留最小计数行
// (Challenged=true + Outcome + 等级/动作/时间)。Challenged 保留,漏斗分母仍数得到。找不到=影响 0 行,不报错。
func (s *dbStore) MarkOutcome(scope, subject, outcome string) error {
	if scope == "" || subject == "" {
		return nil
	}
	// 先定位那一条(id 倒序取一条),再按 id 精确更新——UPDATE ... ORDER BY LIMIT 并非所有 MySQL 版本都稳。
	var row entity.DecisionRow
	err := s.db.Model(&entity.DecisionRow{}).
		Select("id").
		Where("scope = ? AND subject = ? AND outcome = ''", scope, subject).
		Order("id DESC").Limit(1).
		Take(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return err
	}
	return s.db.Model(&entity.DecisionRow{}).Where("id = ?", row.ID).Updates(map[string]any{
		"outcome": outcome,
		"subject": "", "ua": "", "ip": "", "country": "", "isp": "", "asn": 0, "asn_org": "",
	}).Error
}

// ── core.RuleSource ──

// Rules 取某 scope 的规则(启用与否都给,Compile 过滤 enabled)。
func (s *dbStore) Rules(scope string) ([]core.Rule, error) {
	rows, err := s.listRows(scope)
	if err != nil {
		return nil, err
	}
	out := make([]core.Rule, 0, len(rows))
	for _, r := range rows {
		out = append(out, rowToRule(r))
	}
	return out, nil
}

// ── core.PolicyStore ──

// Policy 取某 scope 的「风险等级→动作名」映射。
func (s *dbStore) Policy(scope string) (map[string]string, error) {
	var rows []entity.ActionPolicyRow
	if err := s.db.Where("scope = ?", scope).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		out[r.RiskLevel] = r.Action
	}
	return out, nil
}

// ── consoleStore:规则 CRUD ──

func (s *dbStore) listRows(scope string) ([]entity.RuleRow, error) {
	var rows []entity.RuleRow
	q := s.db.Model(&entity.RuleRow{})
	if scope != "" {
		q = q.Where("scope = ?", scope)
	}
	if err := q.Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *dbStore) getRow(id int64) (*entity.RuleRow, error) {
	var row entity.RuleRow
	if err := s.db.First(&row, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (s *dbStore) upsertRow(row *entity.RuleRow) error {
	now := time.Now()
	row.Modified = now
	if row.ID == 0 {
		row.Created = now
		return s.db.Create(row).Error
	}
	return s.db.Save(row).Error
}

func (s *dbStore) deleteRow(id int64) error {
	return s.db.Delete(&entity.RuleRow{}, id).Error
}

// ── consoleStore:策略读写 ──

// setPolicy 整体覆盖某 scope 的策略(事务内先删后插)。空 map = 清空。
func (s *dbStore) setPolicy(scope string, mapping map[string]string) error {
	now := time.Now()
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("scope = ?", scope).Delete(&entity.ActionPolicyRow{}).Error; err != nil {
			return err
		}
		if len(mapping) == 0 {
			return nil
		}
		rows := make([]entity.ActionPolicyRow, 0, len(mapping))
		for level, action := range mapping {
			rows = append(rows, entity.ActionPolicyRow{Scope: scope, RiskLevel: level, Action: action, Modified: now})
		}
		return tx.Create(&rows).Error
	})
}

// ── consoleStore:统计 / 漏斗 / 明细 ──

// stats 统计某 scope 自 since 起的决策数:按风险等级、按动作名各聚合一份。
func (s *dbStore) stats(scope string, since time.Time) (map[string]int64, map[string]int64, error) {
	type countRow struct {
		K string
		N int64
	}
	agg := func(col string) (map[string]int64, error) {
		var rows []countRow
		err := s.db.Model(&entity.DecisionRow{}).
			Select(col+" AS k, COUNT(*) AS n").
			Where("scope = ? AND created >= ?", scope, since).
			Group(col).Scan(&rows).Error
		if err != nil {
			return nil, err
		}
		out := make(map[string]int64, len(rows))
		for _, r := range rows {
			out[r.K] = r.N
		}
		return out, nil
	}
	byRisk, err := agg("risk_level")
	if err != nil {
		return nil, nil, err
	}
	byAction, err := agg("action")
	if err != nil {
		return nil, nil, err
	}
	return byRisk, byAction, nil
}

// funnel 见 consoleStore.funnel:一次聚合出分母(有 subject)与已完成(outcome 非空)。
func (s *dbStore) funnel(scope string, since time.Time) (funnelCounts, error) {
	var fc funnelCounts
	err := s.db.Model(&entity.DecisionRow{}).
		Select("COUNT(*) AS challenged, "+
			"COALESCE(SUM(CASE WHEN outcome <> '' THEN 1 ELSE 0 END), 0) AS resolved").
		Where("scope = ? AND created >= ? AND challenged = ?", scope, since, true).
		Scan(&fc).Error
	return fc, err
}

// decisions 取某 scope 决策明细(id 倒序);beforeID>0 → 只取 id<beforeID(游标翻页)。
func (s *dbStore) decisions(scope string, limit int, beforeID int64) ([]entity.DecisionRow, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := s.db.Model(&entity.DecisionRow{}).Where("scope = ?", scope)
	if beforeID > 0 {
		q = q.Where("id < ?", beforeID)
	}
	var rows []entity.DecisionRow
	if err := q.Order("id DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// pruneDecisions 批量删除 created 早于 before 的决策流水(每批最多 batch 条,循环删到删不动为止)。
// 返回删除总数。分批是为了不锁大表:一次 DELETE 上万行会长时间持锁,拆成小批更平滑。retention goroutine 用它。
func (s *dbStore) pruneDecisions(before time.Time, batch int) (int64, error) {
	if batch <= 0 {
		batch = 1000
	}
	var total int64
	for {
		res := s.db.Where("created < ?", before).Limit(batch).Delete(&entity.DecisionRow{})
		if res.Error != nil {
			return total, res.Error
		}
		total += res.RowsAffected
		if res.RowsAffected < int64(batch) {
			break
		}
	}
	return total, nil
}

// ── JSON 编解码 helpers(实体 conditions 文本 ↔ 领域/DTO) ──

// parseConditions 把存储里的 conditions JSON 文本解成 []core.RuleCondition;空 / 非法 → 空切片(交给校验层拦)。
func parseConditions(str string) []core.RuleCondition {
	str = strings.TrimSpace(str)
	if str == "" || str == "null" {
		return nil
	}
	var conds []core.RuleCondition
	if err := json.Unmarshal([]byte(str), &conds); err != nil {
		return nil
	}
	return conds
}

// marshalConditions 把 []core.RuleCondition 编成存储用 JSON 文本(nil → "[]")。
func marshalConditions(conds []core.RuleCondition) string {
	if len(conds) == 0 {
		return "[]"
	}
	b, err := json.Marshal(conds)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// marshalAny 把 DTO 里任意 JSON 值(params)编回 RawMessage;nil / 出错 → null。
func marshalAny(v any) json.RawMessage {
	if v == nil {
		return json.RawMessage("null")
	}
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("null")
	}
	return json.RawMessage(b)
}

// rawToAny 把存储/引擎里的 RawMessage 解成任意值(给 DTO 输出);空 / 出错 → nil。
func rawToAny(rawMsg json.RawMessage) any {
	str := strings.TrimSpace(string(rawMsg))
	if str == "" || str == "null" {
		return nil
	}
	var v any
	if err := json.Unmarshal(rawMsg, &v); err != nil {
		return nil
	}
	return v
}

// rowToRule 把规则实体行映射成引擎用的 core.Rule(conditions JSON → []RuleCondition)。
func rowToRule(r entity.RuleRow) core.Rule {
	return core.Rule{
		Name: r.Name, Enabled: r.Enabled, Scope: r.Scope,
		Conditions: parseConditions(r.Conditions),
		Combine:    r.Combine,
		RiskLevel:  core.RiskLevel(r.RiskLevel),
	}
}
