// Package core 是 doorman 的领域内核:风险等级、评估上下文、条件插件契约、注册表、评估引擎,
// 以及对外的 Doorman / 存储契约。纯领域,不碰 gorm/gin,只依赖 model/dto(渲染用的字段/类别元信息)。
// 分层动机见根包 doorman.go:根包只做「类型别名 + feature 装配」,实现散在 core / service / controller。
package core

import (
	"context"
	"time"
)

// RiskLevel 一次评估的风险等级(固定枚举,由低到高)。命中多条规则时取最高;无命中 = RiskNone。
type RiskLevel string

const (
	RiskNone     RiskLevel = "none"     // 无风险 / 未命中
	RiskLow      RiskLevel = "low"      // 低
	RiskMedium   RiskLevel = "medium"   // 中
	RiskHigh     RiskLevel = "high"     // 高
	RiskCritical RiskLevel = "critical" // 极高
)

// riskRank 给各等级一个可比较的次序(取最高用)。未知等级视为 0(最低)。
var riskRank = map[RiskLevel]int{RiskNone: 0, RiskLow: 1, RiskMedium: 2, RiskHigh: 3, RiskCritical: 4}

// RiskLevels 全部合法等级(由低到高),给配置页渲染下拉、给校验用。
var RiskLevels = []RiskLevel{RiskNone, RiskLow, RiskMedium, RiskHigh, RiskCritical}

// RiskLevelStrings 全部合法等级的字符串形式(给对外 DTO;DTO 用 string 不引领域类型)。
func RiskLevelStrings() []string {
	out := make([]string, 0, len(RiskLevels))
	for _, l := range RiskLevels {
		out = append(out, string(l))
	}
	return out
}

// Rank 返回等级次序(越大越严重);未知等级 = 0。
func (r RiskLevel) Rank() int { return riskRank[r] }

// Valid 是否是已知合法等级。
func (r RiskLevel) Valid() bool { _, ok := riskRank[r]; return ok }

// MatchedRule 一条命中规则的摘要(给业务记日志 / 统计)。
type MatchedRule struct {
	Name  string    `json:"name"`
	Level RiskLevel `json:"level"`
}

// Assessment 一次评估的结论:最高风险等级 + 命中的规则。doorman 不给动作——业务据 Level 自己决定。
type Assessment struct {
	Level   RiskLevel     // 命中规则中的最高等级;无命中 = RiskNone
	Matched []MatchedRule // 命中的规则(审计 / 调试 / 统计);无命中为空
}

// Attempt 一次请求的事实快照(评估的输入)。核心字段固定 + Ext 扩展袋(不同场景事实不同)。
// 由业务填,doorman 只读、不采集。
type Attempt struct {
	UA        string         // User-Agent 原文
	IP        string         // 客户端 IP
	Country   string         // ISO-3166 alpha-2(可空)
	ISP       string         // 运营商(仅国内 geoip 有:电信/联通/移动…;可空)
	ASN       uint           // 自治系统号(0=未知)
	ASNOrg    string         // AS 归属组织名
	IsHosting bool           // 是否机房 / 云 / 托管 / Tor 出口(来自 geoip ASN 面)
	Channel   string         // 渠道(direct/organic/…)
	Ext       map[string]any // 业务自定义事实(login 的失败次数、register 的邮箱域名…)
}

// Store 有状态条件(限流计数等)所需的最小存储抽象。业务注入实现(如 Redis)。
// 纯函数条件(ua/asn/country)用不到它;可为 nil。
type Store interface {
	// Incr 对 key 计数并返回递增后的值;key 首次出现时落 window 作为 TTL。
	Incr(ctx context.Context, key string, window time.Duration) (int64, error)
}

// RequestView 原始请求的只读视图(取 header / 表单字段),避免 doorman 直接依赖 gin/aurora。
// 业务在接线处用 gin.Context 适配一个实现即可。
type RequestView interface {
	Header(name string) string
	Field(name string) string // 表单 / JSON 字段
}

// Context 一次评估的全部上下文;每个条件插件都读它,也能用 Set/Get 在插件间传值。
type Context struct {
	Ctx     context.Context
	Scope   string
	Attempt Attempt
	Req     RequestView
	Store   Store
	// Subject 业务给的**不透明关联键**(如 sha256(邮箱)):只在「判定后会有后续结果、需回填」的评估上设
	// (如判要激活)。设了它,Record 会落库,供之后 MarkOutcome(scope, subject, outcome) 把结果对上号,
	// 形成「判定→激活/放弃」漏斗。空 = 该次判定无后续(终态,如放行/拦截)。doorman 不解释它。
	Subject string
	scratch map[string]any
}

// Set 在插件间暂存一个值。
func (c *Context) Set(k string, v any) {
	if c.scratch == nil {
		c.scratch = map[string]any{}
	}
	c.scratch[k] = v
}

// Get 取暂存值。
func (c *Context) Get(k string) (any, bool) {
	v, ok := c.scratch[k]
	return v, ok
}
