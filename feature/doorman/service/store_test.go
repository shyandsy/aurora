package service

// dbStore 的真库集成测试(sqlite 内存库):专测 fakeStore 顶不掉的真 SQL——
// 漏斗聚合、markOutcome 清 PII、prune 分批删、stats group by、decisions 游标翻页、策略先删后插。

import (
	"testing"
	"time"

	"github.com/shyandsy/aurora/feature/doorman/core"
	"github.com/shyandsy/aurora/feature/doorman/model/entity"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("打开 sqlite 失败: %v", err)
	}
	// 测试里用 AutoMigrate 建表(生产靠宿主 goose,doorman 不自 DDL)。
	if err := db.AutoMigrate(&entity.RuleRow{}, &entity.ActionPolicyRow{}, &entity.DecisionRow{}); err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	return db
}

// ── 规则 CRUD:落库 → 读回 → 改 → 删 ──
func TestDBStore_RuleCRUD(t *testing.T) {
	s := newDBStore(openTestDB(t))

	row := &entity.RuleRow{Scope: "register", Name: "r1", Conditions: `[{"type":"asn_hosting"}]`, Combine: "and", RiskLevel: "high", Enabled: true}
	if err := s.upsertRow(row); err != nil {
		t.Fatalf("新增失败: %v", err)
	}
	if row.ID == 0 || row.Created.IsZero() || row.Modified.IsZero() {
		t.Fatalf("新增应回填 ID + 时间戳,got %+v", row)
	}

	// Rules() 映射成 core.Rule(conditions JSON → 结构)
	rules, err := s.Rules("register")
	if err != nil || len(rules) != 1 || len(rules[0].Conditions) != 1 || rules[0].RiskLevel != core.RiskHigh {
		t.Fatalf("Rules 映射不对: %v %+v", err, rules)
	}

	// 改:保留 Created,更新 Modified
	created := row.Created
	time.Sleep(2 * time.Millisecond)
	row.RiskLevel = "critical"
	if err := s.upsertRow(row); err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	got, _ := s.getRow(row.ID)
	if got.RiskLevel != "critical" || !got.Created.Equal(created) {
		t.Fatalf("更新应改等级、留 Created,got %+v", got)
	}

	// 删
	if err := s.deleteRow(row.ID); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	if got, _ := s.getRow(row.ID); got != nil {
		t.Fatal("删除后应查不到")
	}
}

// ── 策略先删后插(整体覆盖)+ 空 map 清空 ──
func TestDBStore_Policy(t *testing.T) {
	s := newDBStore(openTestDB(t))

	if err := s.setPolicy("register", map[string]string{"critical": "block", "high": "email_verify"}); err != nil {
		t.Fatalf("写策略失败: %v", err)
	}
	m, _ := s.Policy("register")
	if m["critical"] != "block" || m["high"] != "email_verify" {
		t.Fatalf("策略读回不对: %+v", m)
	}
	// 覆盖:只留 critical
	if err := s.setPolicy("register", map[string]string{"critical": "allow"}); err != nil {
		t.Fatalf("覆盖策略失败: %v", err)
	}
	m, _ = s.Policy("register")
	if len(m) != 1 || m["critical"] != "allow" {
		t.Fatalf("覆盖后应只剩 critical=allow,got %+v", m)
	}
	// 空 map 清空
	if err := s.setPolicy("register", nil); err != nil {
		t.Fatalf("清空策略失败: %v", err)
	}
	if m, _ := s.Policy("register"); len(m) != 0 {
		t.Fatalf("清空后应为空,got %+v", m)
	}
}

// ── 统计 group by:按风险等级 / 动作各聚合;scope 与时间窗隔离 ──
func TestDBStore_Stats(t *testing.T) {
	s := newDBStore(openTestDB(t))
	rec := func(scope, level, action string) {
		if err := s.Record(core.Decision{Scope: scope, RiskLevel: level, Action: action}); err != nil {
			t.Fatalf("记录失败: %v", err)
		}
	}
	rec("register", "high", "email_verify")
	rec("register", "high", "email_verify")
	rec("register", "critical", "block")
	rec("login", "high", "email_verify") // 别的 scope 不该混进来

	byRisk, byAction, err := s.stats("register", time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("stats 失败: %v", err)
	}
	if byRisk["high"] != 2 || byRisk["critical"] != 1 || len(byRisk) != 2 {
		t.Fatalf("按等级聚合不对: %+v", byRisk)
	}
	if byAction["email_verify"] != 2 || byAction["block"] != 1 {
		t.Fatalf("按动作聚合不对: %+v", byAction)
	}
	// 时间窗:未来起点 → 数不到
	if b, _, _ := s.stats("register", time.Now().Add(time.Hour)); len(b) != 0 {
		t.Fatalf("窗外不该有数据,got %+v", b)
	}
}

// ── 漏斗 + markOutcome:分母=有 subject,已完成=回填 outcome;回填清 PII 但留 Challenged ──
func TestDBStore_FunnelAndMarkOutcome(t *testing.T) {
	s := newDBStore(openTestDB(t))
	since := time.Now().Add(-time.Hour)

	// 两条判要激活(有 subject),一条无后续(无 subject,不进分母)
	_ = s.Record(core.Decision{Scope: "register", RiskLevel: "high", Action: "email_verify", Subject: "a@x", Challenged: true, UA: "ua", IP: "1.1.1.1", ASN: 42})
	_ = s.Record(core.Decision{Scope: "register", RiskLevel: "high", Action: "email_verify", Subject: "b@x", Challenged: true})
	_ = s.Record(core.Decision{Scope: "register", RiskLevel: "none", Action: "allow"}) // 无 subject

	fc, err := s.funnel("register", since)
	if err != nil {
		t.Fatalf("funnel 失败: %v", err)
	}
	if fc.Challenged != 2 || fc.Resolved != 0 {
		t.Fatalf("回填前:分母应 2、已完成 0,got %+v", fc)
	}

	// 回填 a@x → 已完成 1;且清 PII(subject/ua/ip/asn 清空),但 Challenged 仍在
	if err := s.MarkOutcome("register", "a@x", "activated"); err != nil {
		t.Fatalf("markOutcome 失败: %v", err)
	}
	fc, _ = s.funnel("register", since)
	if fc.Challenged != 2 || fc.Resolved != 1 {
		t.Fatalf("回填后:分母仍 2、已完成 1,got %+v", fc)
	}
	// 校验那条被清了 PII、留了 outcome + challenged
	var row entity.DecisionRow
	s.db.Where("scope = ? AND outcome = ?", "register", "activated").Take(&row)
	if row.Subject != "" || row.UA != "" || row.IP != "" || row.ASN != 0 {
		t.Fatalf("回填后应清空 PII,got %+v", row)
	}
	if !row.Challenged || row.Outcome != "activated" {
		t.Fatalf("回填后应留 Challenged + Outcome,got %+v", row)
	}

	// 重复回填 a@x:已无未回填的匹配 → no-op,不报错、不改变已完成数
	if err := s.MarkOutcome("register", "a@x", "again"); err != nil {
		t.Fatalf("重复回填不该报错: %v", err)
	}
	if fc, _ := s.funnel("register", since); fc.Resolved != 1 {
		t.Fatalf("重复回填不该多算,got %+v", fc)
	}
	// 找不到 subject → no-op
	if err := s.MarkOutcome("register", "nobody@x", "x"); err != nil {
		t.Fatalf("回填不存在的 subject 应 no-op,got %v", err)
	}
}

// ── 决策明细游标翻页(id 倒序,before 游标) ──
func TestDBStore_DecisionsPaging(t *testing.T) {
	s := newDBStore(openTestDB(t))
	for i := 0; i < 5; i++ {
		_ = s.Record(core.Decision{Scope: "register", RiskLevel: "low", Action: "allow"})
	}
	page1, err := s.decisions("register", 2, 0)
	if err != nil || len(page1) != 2 {
		t.Fatalf("第一页应 2 条: %v %d", err, len(page1))
	}
	if page1[0].ID < page1[1].ID {
		t.Fatal("应按 id 倒序(新→旧)")
	}
	page2, _ := s.decisions("register", 2, page1[1].ID)
	if len(page2) != 2 || page2[0].ID >= page1[1].ID {
		t.Fatalf("第二页应接着游标往下,got %+v", page2)
	}
}

// ── 分批清理:删早于 before 的,分批循环删干净;不误删新数据 ──
func TestDBStore_PruneDecisions(t *testing.T) {
	db := openTestDB(t)
	s := newDBStore(db)
	old := time.Now().Add(-48 * time.Hour)
	fresh := time.Now()
	// 直接插入带指定 Created 的行(绕过 Record 的 now())
	for i := 0; i < 5; i++ {
		db.Create(&entity.DecisionRow{Scope: "register", RiskLevel: "low", Created: old})
	}
	for i := 0; i < 3; i++ {
		db.Create(&entity.DecisionRow{Scope: "register", RiskLevel: "low", Created: fresh})
	}

	// batch=2,before=24h 前 → 应删掉 5 条旧的,分多批
	n, err := s.pruneDecisions(time.Now().Add(-24*time.Hour), 2)
	if err != nil || n != 5 {
		t.Fatalf("应删 5 条旧数据,got n=%d err=%v", n, err)
	}
	var remain int64
	db.Model(&entity.DecisionRow{}).Count(&remain)
	if remain != 3 {
		t.Fatalf("应剩 3 条新数据,got %d", remain)
	}
}
