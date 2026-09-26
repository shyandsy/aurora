// Package doorman 是一个「门房 / 风险评估器」:对某个业务操作(scope,如注册 / 登录 / 某项敏感操作),
// 按可配规则评估这次请求,输出**一个风险等级**(none/low/medium/high/critical)。
//
// doorman **不认识任何业务动作**。拦截 / 要求邮件激活 / 放行这些都由**业务侧**根据风险等级自己映射、
// 自己执行(如注册场景:critical→拦截、high/medium→要求邮件激活、low/none→放行)。
//
// 设计要点(详见 README.md):
//   - 中性、与业务解耦:只读 Attempt 上的事实字段,不碰 geoip / 不采集 / 不认业务名 / 不含任何动作。
//   - 一条规则 = 多个条件(且/或组合)→ 一个风险等级。条件(Condition)是唯一的可注册插件类别,自带配置。
//   - 一次判定过所有规则,命中的取**最高**风险等级;无优先级、无责任链、无短路。
//
// 本包是 aurora 框架层的通用 feature,跨项目复用:严格中性。分层:
//   - 本文件(根包):对外类型别名 + 常量,唯一稳定 API 面(消费方只 import 本包写 doorman.XXX)。
//   - feature.go:aurora Feature 装配(NewFeature + Options + Setup)。
//   - core/:领域内核(风险等级、评估引擎、条件插件、注册表、Doorman 契约)。
//   - service/:管理面 Console + 默认 DB 存储(实现 core 契约)。
//   - controller/:管理 API(配置页后端)HTTP 层。
//   - model/{entity,dto}:持久化实体 / 对外读写模型。
package doorman

import (
	"github.com/shyandsy/aurora/feature/doorman/core"
	"github.com/shyandsy/aurora/feature/doorman/model/dto"
	"github.com/shyandsy/aurora/feature/doorman/service"
)

// ── 领域类型(re-export 自 core;消费方写 doorman.XXX,无需 import 子包) ──
type (
	// Doorman 门房:对外唯一入口(Assess / ActionFor / Record / MarkOutcome)。
	Doorman = core.Doorman
	// Context 一次评估的上下文(Scope + Attempt + Subject + Req/Store)。
	Context = core.Context
	// Attempt 一次请求的事实快照(评估输入,业务填)。
	Attempt = core.Attempt
	// Assessment 一次评估的结论(最高风险等级 + 命中规则)。
	Assessment = core.Assessment
	// MatchedRule 一条命中规则的摘要。
	MatchedRule = core.MatchedRule
	// RiskLevel 风险等级枚举(none/low/medium/high/critical)。
	RiskLevel = core.RiskLevel
	// Store 有状态条件所需的最小存储抽象(限流计数等;可 nil)。
	Store = core.Store
	// RequestView 原始请求只读视图(取 header / 表单字段)。
	RequestView = core.RequestView
	// Condition 条件类别插件(doorman 唯一扩展点)。
	Condition = core.Condition
	// Check 一条已配置好的条件判定。
	Check = core.Check
	// Rule 一条配置规则(多个条件 → 一个风险等级)。
	Rule = core.Rule
	// RuleCondition 规则里的一个条件项。
	RuleCondition = core.RuleCondition
	// RuleSource 规则来源契约(业务提供 / 默认 DB 存储实现)。
	RuleSource = core.RuleSource
	// PolicyStore 「风险→动作」策略只读来源。
	PolicyStore = core.PolicyStore
	// DecisionRecorder 决策流水记录契约。
	DecisionRecorder = core.DecisionRecorder
	// Decision 一次决策的领域快照(落流水用)。
	Decision = core.Decision
	// Registry 条件插件 + scope 定义注册表。
	Registry = core.Registry
	// ActionKind 动作类型(friction 进漏斗 / terminal 只记一笔)。
	ActionKind = core.ActionKind
	// ActionDef 一个动作的定义(名 + 标签 + 类型)。
	ActionDef = core.ActionDef
	// ScopeDef 一个 scope 的定义(id + 标签 + 动作目录)。
	ScopeDef = core.ScopeDef
	// Console 管理面(配置页后端:规则 / 策略 / 观测)。
	Console = service.Console
)

// ── 对外读写模型(re-export 自 model/dto) ──
type (
	Field          = dto.Field
	KindInfo       = dto.KindInfo
	ConditionDTO   = dto.ConditionDTO
	RuleDTO        = dto.RuleDTO
	KindsDTO       = dto.KindsDTO
	PolicyKindsDTO = dto.PolicyKindsDTO
	ActionDTO      = dto.ActionDTO
	ScopeDTO       = dto.ScopeDTO
	StatsDTO       = dto.StatsDTO
	FunnelDTO      = dto.FunnelDTO
	DecisionDTO    = dto.DecisionDTO
)

// ── 常量 / 变量 re-export ──
const (
	RiskNone     = core.RiskNone
	RiskLow      = core.RiskLow
	RiskMedium   = core.RiskMedium
	RiskHigh     = core.RiskHigh
	RiskCritical = core.RiskCritical

	ActionTerminal = core.ActionTerminal
	ActionFriction = core.ActionFriction

	CombineAnd = core.CombineAnd
	CombineOr  = core.CombineOr

	FieldString     = core.FieldString
	FieldStringList = core.FieldStringList
	FieldInt        = core.FieldInt
	FieldIntList    = core.FieldIntList
	FieldDuration   = core.FieldDuration
	FieldSelect     = core.FieldSelect

	TypeUAMatch    = core.TypeUAMatch
	TypeASNHosting = core.TypeASNHosting
	TypeCountryIn  = core.TypeCountryIn
)

// RiskLevels 全部合法等级(由低到高)。
var RiskLevels = core.RiskLevels

// ErrRuleNotFound 更新/删除不存在的规则(re-export,供 controller 判别)。
var ErrRuleNotFound = service.ErrRuleNotFound
