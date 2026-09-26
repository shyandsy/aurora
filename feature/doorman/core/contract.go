package core

// 本文件是 doorman 对外/对下的契约:Doorman(业务唯一入口)+ 三个存储契约(规则/策略/决策流水)。
// 存储契约用领域类型(Rule / Decision),不引 gorm 实体——实现在 service 层做实体映射(依赖倒置)。

// RuleSource 规则来源契约(业务提供:通常是 DB 表 + 自己的读缓存)。doorman 不自带存储。
type RuleSource interface {
	// Rules 返回某 scope 下的全部规则(启用与否都可给,Compile 会过滤 enabled)。
	Rules(scope string) ([]Rule, error)
}

// PolicyStore 「风险等级 → 动作名」策略的只读来源(运行时业务据它把等级映射成动作)。
type PolicyStore interface {
	// Policy 返回某 scope 的映射 {风险等级: 动作名};未配置的等级不出现在 map 里。
	Policy(scope string) (map[string]string, error)
}

// Decision 一次门禁决策的领域快照(要落流水的事实 + 结论)。service 层把它映射成 gorm 实体。
// 中性:全是 Attempt 上的事实 + 命中规则名 + 风险等级 + 动作名;doorman 不解释动作名。
type Decision struct {
	Scope      string
	RiskLevel  string
	Action     string
	Matched    string // 命中规则名,逗号分隔(无命中为空)
	UA         string
	IP         string
	Country    string
	ISP        string
	ASN        uint
	ASNOrg     string
	IsHosting  bool
	Subject    string // 关联键(需回填的决策才设);回填成功后 service 清空
	Challenged bool   // 判后需回填结果(设了 Subject 即 true),进漏斗分母
}

// DecisionRecorder 记一次门禁决策流水 + 回填结果(best-effort)。svc.Record / MarkOutcome 用它;service.dbStore 实现。
type DecisionRecorder interface {
	Record(d Decision) error
	// MarkOutcome 给某 (scope, subject) **最近一条尚未回填**(outcome 空)的决策写上结果串。
	// 找不到匹配 = no-op(不报错)。best-effort:调用方吞错。
	MarkOutcome(scope, subject, outcome string) error
}

// Doorman 门房:对外唯一入口。业务用它对一次请求做风险评估(在 handler 里手动调,或包成 middleware)。
type Doorman interface {
	// Assess 对一次请求跑该 scope 的规则,返回风险等级 + 命中规则。业务据 Assessment.Level 自己决定动作。
	Assess(c *Context) Assessment
	// ActionFor 查某 scope 某风险等级配置的**动作名**(来自「风险→动作」策略,后台可配)。
	// ok=false 表示该等级没配策略(或没接策略存储)——业务应回退到自己的内置默认。
	// doorman 只回传动作名字符串,不认识其含义、不执行动作。
	ActionFor(scope string, level RiskLevel) (action string, ok bool)
	// Record 记一次门禁决策流水(供统计 + 明细):从 Context 取事实 + 评估结果 + 业务最终选的动作名。
	// best-effort:失败静默,绝不影响主流程;没接存储时 no-op。
	// 若 Context.Subject 非空,会一并落库,供之后 MarkOutcome 回填结果(判定→结果漏斗)。
	Record(c *Context, a Assessment, action string)
	// MarkOutcome 给某 (scope, subject) 最近一条尚未回填的决策写上业务结果串(如注册激活成功时写 "activated")。
	// subject 必须和当初 Record 时 Context.Subject 一致。doorman 不解释 outcome 值,只用它区分「已回填/未回填」。
	// best-effort:失败静默;没接存储时 no-op。
	MarkOutcome(scope, subject, outcome string)
}
