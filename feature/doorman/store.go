package doorman

import (
	"encoding/json"
	"strings"
	"time"

	"gorm.io/gorm"
)

// ruleRow 是规则的持久化模型(表 doorman_rule)。conditions 以 JSON 文本存;加条件类别不改表结构。
// 不存「动作」——动作在业务侧据 risk_level 映射。
type ruleRow struct {
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
func (ruleRow) TableName() string { return "doorman_rule" }

// actionPolicyRow 是「风险等级 → 动作名」映射的持久化模型(表 doorman_action_policy)。
// 每 (scope, risk_level) 一行 → 一个动作名。action 是**业务约定的字符串**,doorman 只存不解释。
type actionPolicyRow struct {
	Scope     string    `gorm:"column:scope;size:64;primaryKey"`
	RiskLevel string    `gorm:"column:risk_level;size:16;primaryKey"`
	Action    string    `gorm:"column:action;size:64;not null"`
	Modified  time.Time `gorm:"column:modified"`
}

// TableName 固定表名。
func (actionPolicyRow) TableName() string { return "doorman_action_policy" }

// decisionRow 是一次门禁决策的流水(表 doorman_decision):业务每次评估后记一行,供统计 + 明细。
// 中性:存的都是 Attempt 上的事实(UA/IP/ASN/国家/机房)+ 命中规则名 + 风险等级 + 动作名 + 时间,
// doorman 不解释动作名。既用于「各等级/各动作」汇总,也用于「决策明细」表(时间/命中规则/风险/动作/UA·IP·ASN)。
type decisionRow struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement"`
	Scope     string    `gorm:"column:scope;size:64;index:idx_doorman_decision_scope_id,priority:1;index:idx_doorman_decision_scope_subject,priority:1"`
	RiskLevel string    `gorm:"column:risk_level;size:16"`
	Action    string    `gorm:"column:action;size:64"`
	Matched   string    `gorm:"column:matched;size:512"` // 命中规则名,逗号分隔(无命中为空)
	UA        string    `gorm:"column:ua;size:512"`
	IP        string    `gorm:"column:ip;size:45"`
	Country   string    `gorm:"column:country;size:8"`
	ISP       string    `gorm:"column:isp;size:64"` // 运营商(仅国内 geoip 有:电信/联通/移动…);空=未知
	ASN       uint      `gorm:"column:asn"`
	ASNOrg    string    `gorm:"column:asn_org;size:128"`
	IsHosting bool      `gorm:"column:is_hosting"`
	Created   time.Time `gorm:"column:created;index"`
	// Challenged 标记这条决策「判后有后续、需回填结果」(如判要激活)。业务设了 Subject 即为 true。
	// 单独立一个布尔,是因为回填(完成)后会清掉 Subject 等 PII,但漏斗分母仍要数得到它(见 markOutcome / funnel)。
	Challenged bool `gorm:"column:challenged"`
	// Subject 业务给的**关联键 + 标识**(如注册场景 = 规整后的邮箱):只在需回填的决策上设。
	// 两用:①把这条决策和之后 MarkOutcome 的结果对上号;②明细里展示"是谁"(哪个邮箱没完成激活)。
	// ⚠️ 回填成功后会被清空:合法用户已在业务库里,doorman 不囤其 PII,只留最小计数行(Challenged + Outcome)。
	Subject string `gorm:"column:subject;size:191;index:idx_doorman_decision_scope_subject,priority:2"`
	// Outcome 业务回填的**不透明结果串**(如 "activated"):空 = 未回填 = 判了要激活但没完成。doorman 只判空/非空。
	Outcome string `gorm:"column:outcome;size:32"`
}

// TableName 固定表名。
func (decisionRow) TableName() string { return "doorman_decision" }

// toRule 映射成引擎用的 Rule(conditions JSON → []RuleCondition)。
func (r ruleRow) toRule() Rule {
	return Rule{
		Name: r.Name, Enabled: r.Enabled, Scope: r.Scope,
		Conditions: parseConditions(r.Conditions),
		Combine:    r.Combine,
		RiskLevel:  RiskLevel(r.RiskLevel),
	}
}

// parseConditions 把存储里的 conditions JSON 文本解成 []RuleCondition;空 / 非法 → 空切片(交给校验层拦)。
func parseConditions(s string) []RuleCondition {
	s = strings.TrimSpace(s)
	if s == "" || s == "null" {
		return nil
	}
	var conds []RuleCondition
	if err := json.Unmarshal([]byte(s), &conds); err != nil {
		return nil
	}
	return conds
}

// marshalConditions 把 []RuleCondition 编成存储用 JSON 文本(nil → "[]")。
func marshalConditions(conds []RuleCondition) string {
	if len(conds) == 0 {
		return "[]"
	}
	b, err := json.Marshal(conds)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// normParams 把空参数归一成 JSON null,避免 json.Unmarshal 空串报错(插件按需自校验)。
func normParams(s string) json.RawMessage {
	if strings.TrimSpace(s) == "" {
		return json.RawMessage("null")
	}
	return json.RawMessage(s)
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
func rawToAny(raw json.RawMessage) any {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil
	}
	return v
}

// PolicyStore 「风险等级 → 动作名」策略的只读来源(运行时业务据它把等级映射成动作)。
type PolicyStore interface {
	// Policy 返回某 scope 的映射 {风险等级: 动作名};未配置的等级不出现在 map 里。
	Policy(scope string) (map[string]string, error)
}

// decisionRecorder 记一次门禁决策流水(best-effort)。svc.Record / svc.MarkOutcome 用它;dbStore 实现。
type decisionRecorder interface {
	recordDecision(row *decisionRow) error
	// markOutcome 给某 (scope, subject) **最近一条尚未回填**(outcome 空)的决策写上结果串。
	// 找不到匹配 = no-op(不报错)。best-effort:调用方吞错。
	markOutcome(scope, subject, outcome string) error
}

// ruleStore 是 Console 依赖的存储抽象(dbStore 实现;单测可替身)。
// 含规则只读 + 增删改、策略(风险→动作)读写、决策流水记录 + 统计聚合。
type ruleStore interface {
	RuleSource
	PolicyStore
	decisionRecorder
	listRows(scope string) ([]ruleRow, error) // scope 空 = 全部
	getRow(id int64) (*ruleRow, error)
	upsertRow(row *ruleRow) error
	deleteRow(id int64) error
	// setPolicy 用给定映射**整体覆盖**某 scope 的策略(删该 scope 旧行,再写新行;空 map = 清空该 scope)。
	setPolicy(scope string, mapping map[string]string) error
	// stats 统计某 scope 自 since 起的决策:按风险等级、按动作名各出一份计数。
	stats(scope string, since time.Time) (byRisk map[string]int64, byAction map[string]int64, err error)
	// funnel 统计某 scope 自 since 起「有后续结果的决策」的漏斗:有 subject 的为分母(判要激活),
	// 其中 outcome 非空 = 已完成(激活)。未完成 = 分母 − 已完成(不猜时间,回填了就是完成、没回填就是没完成)。
	funnel(scope string, since time.Time) (funnelCounts, error)
	// decisions 取某 scope 的决策明细(按 id 倒序,新→旧);beforeID>0 时只取 id<beforeID(游标翻页)。
	decisions(scope string, limit int, beforeID int64) ([]decisionRow, error)
}

// dbStore 是基于 gorm 的默认存储(feature 自建表、自持存储,复用方只需注入 db)。
type dbStore struct {
	db *gorm.DB
}

// NewDBStore 建 DB store。
func NewDBStore(db *gorm.DB) *dbStore { return &dbStore{db: db} }

// funnelCounts 「判定→结果」漏斗计数(只统计有 subject 的决策)。未完成 = Challenged − Resolved(展示侧算)。
type funnelCounts struct {
	Challenged int64 // 有 subject(判定后需回填结果,如判要激活)
	Resolved   int64 // 其中 outcome 非空(业务回填了结果,如激活成功)
}

// recordDecision 记一次门禁决策流水(供统计 + 明细)。best-effort:调用方吞错。
func (s *dbStore) recordDecision(row *decisionRow) error {
	if row.Created.IsZero() {
		row.Created = time.Now()
	}
	return s.db.Create(row).Error
}

// markOutcome 给某 (scope, subject) 最近一条尚未回填的决策写 outcome,并**清空 PII**
// (邮箱/UA/IP/运营商/ASN):合法用户已在业务库里,doorman 完成后不囤其明细,只留最小计数行
// (Challenged=true + Outcome + 等级/动作/时间)。Challenged 保留,漏斗分母仍数得到。找不到=影响 0 行,不报错。
func (s *dbStore) markOutcome(scope, subject, outcome string) error {
	if scope == "" || subject == "" {
		return nil
	}
	// 先定位那一条(id 倒序取一条),再按 id 精确更新——UPDATE ... ORDER BY LIMIT 并非所有 MySQL 版本都稳。
	var row decisionRow
	err := s.db.Model(&decisionRow{}).
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
	return s.db.Model(&decisionRow{}).Where("id = ?", row.ID).Updates(map[string]any{
		"outcome": outcome,
		"subject": "", "ua": "", "ip": "", "country": "", "isp": "", "asn": 0, "asn_org": "",
	}).Error
}

// funnel 见 ruleStore.funnel:一次聚合出分母(有 subject)与已完成(outcome 非空)。
func (s *dbStore) funnel(scope string, since time.Time) (funnelCounts, error) {
	var fc funnelCounts
	err := s.db.Model(&decisionRow{}).
		Select("COUNT(*) AS challenged, "+
			"COALESCE(SUM(CASE WHEN outcome <> '' THEN 1 ELSE 0 END), 0) AS resolved").
		Where("scope = ? AND created >= ? AND challenged = ?", scope, since, true).
		Scan(&fc).Error
	return fc, err
}

// pruneDecisions 批量删除 created 早于 before 的决策流水(每批最多 batch 条,循环删到删不动为止)。
// 返回删除总数。分批是为了不锁大表:一次 DELETE 上万行会长时间持锁,拆成小批更平滑。
func (s *dbStore) pruneDecisions(before time.Time, batch int) (int64, error) {
	if batch <= 0 {
		batch = 1000
	}
	var total int64
	for {
		res := s.db.Where("created < ?", before).Limit(batch).Delete(&decisionRow{})
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

// decisions 取某 scope 决策明细(id 倒序);beforeID>0 → 只取 id<beforeID(游标翻页)。
func (s *dbStore) decisions(scope string, limit int, beforeID int64) ([]decisionRow, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := s.db.Model(&decisionRow{}).Where("scope = ?", scope)
	if beforeID > 0 {
		q = q.Where("id < ?", beforeID)
	}
	var rows []decisionRow
	if err := q.Order("id DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// stats 统计某 scope 自 since 起的决策数:按风险等级、按动作名各聚合一份。
func (s *dbStore) stats(scope string, since time.Time) (map[string]int64, map[string]int64, error) {
	type countRow struct {
		K string
		N int64
	}
	agg := func(col string) (map[string]int64, error) {
		var rows []countRow
		err := s.db.Model(&decisionRow{}).
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

// Policy 见 PolicyStore:取某 scope 的「风险等级→动作名」映射。
func (s *dbStore) Policy(scope string) (map[string]string, error) {
	var rows []actionPolicyRow
	if err := s.db.Where("scope = ?", scope).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		out[r.RiskLevel] = r.Action
	}
	return out, nil
}

// setPolicy 整体覆盖某 scope 的策略(事务内先删后插)。空 map = 清空。
func (s *dbStore) setPolicy(scope string, mapping map[string]string) error {
	now := time.Now()
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("scope = ?", scope).Delete(&actionPolicyRow{}).Error; err != nil {
			return err
		}
		if len(mapping) == 0 {
			return nil
		}
		rows := make([]actionPolicyRow, 0, len(mapping))
		for level, action := range mapping {
			rows = append(rows, actionPolicyRow{Scope: scope, RiskLevel: level, Action: action, Modified: now})
		}
		return tx.Create(&rows).Error
	})
}

// Rules 见 RuleSource:取某 scope 的规则(启用与否都给,Compile 过滤 enabled)。
func (s *dbStore) Rules(scope string) ([]Rule, error) {
	rows, err := s.listRows(scope)
	if err != nil {
		return nil, err
	}
	out := make([]Rule, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.toRule())
	}
	return out, nil
}

func (s *dbStore) listRows(scope string) ([]ruleRow, error) {
	var rows []ruleRow
	q := s.db.Model(&ruleRow{})
	if scope != "" {
		q = q.Where("scope = ?", scope)
	}
	if err := q.Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *dbStore) getRow(id int64) (*ruleRow, error) {
	var row ruleRow
	if err := s.db.First(&row, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (s *dbStore) upsertRow(row *ruleRow) error {
	now := time.Now()
	row.Modified = now
	if row.ID == 0 {
		row.Created = now
		return s.db.Create(row).Error
	}
	return s.db.Save(row).Error
}

func (s *dbStore) deleteRow(id int64) error {
	return s.db.Delete(&ruleRow{}, id).Error
}
