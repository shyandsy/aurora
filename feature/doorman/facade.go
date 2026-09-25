package doorman

import (
	"strings"
	"sync"
	"time"
)

// RuleSource 规则来源契约(业务提供:通常是 DB 表 + 自己的读缓存)。doorman 不自带存储。
type RuleSource interface {
	// Rules 返回某 scope 下的全部规则(启用与否都可给,Compile 会过滤 enabled)。
	Rules(scope string) ([]Rule, error)
}

// Doorman 门房:对外唯一入口。业务用它对一次请求做风险评估(在 handler 里手动调,或包成 middleware)。
type Doorman interface {
	// Assess 对一次请求跑该 scope 的规则,返回风险等级 + 命中规则。业务据 Assessment.Level 自己决定动作。
	Assess(c *Context) Assessment
	// ActionFor 查某 scope 某风险等级配置的**动作名**(来自「风险→动作」策略,后台可配)。
	// ok=false 表示该等级没配策略(或没接策略存储)——业务应回退到自己的内置默认。
	// doorman 只回传动作名字符串,不认识其含义、不执行动作。
	ActionFor(scope string, level RiskLevel) (action string, ok bool)
	// Record 记一次门禁决策流水(供统计 + 明细):从 Context 取事实(scope/UA/IP/ASN/国家/机房)+
	// 评估结果(等级、命中规则)+ 业务最终选的动作名。best-effort:失败静默,绝不影响主流程;没接存储时 no-op。
	// 若 Context.Subject 非空,会一并落库,供之后 MarkOutcome 回填结果(判定→结果漏斗)。
	Record(c *Context, a Assessment, action string)
	// MarkOutcome 给某 (scope, subject) 最近一条尚未回填的决策写上业务结果串(如注册激活成功时写 "activated")。
	// subject 必须和当初 Record 时 Context.Subject 一致。doorman 不解释 outcome 值,只用它区分「已回填/未回填」。
	// best-effort:失败静默;没接存储时 no-op。
	MarkOutcome(scope, subject, outcome string)
}

// svc 是 Doorman 的实现:持有注册表 + 规则来源 + 策略来源,编译/策略结果按 scope 缓存并短 TTL 热加载。
type svc struct {
	reg   *Registry
	src   RuleSource
	psrc  PolicyStore      // 「风险→动作」策略来源;可为 nil(ActionFor 一律 ok=false)
	rec   decisionRecorder // 决策流水记录;可为 nil(Record 是 no-op)
	ttl   time.Duration
	mu    sync.RWMutex
	cache map[string]cachedRules
	pmu   sync.RWMutex
	pol   map[string]cachedPolicy
}

type cachedRules struct {
	rules []*compiledRule
	at    time.Time
}

type cachedPolicy struct {
	m  map[string]string
	at time.Time
}

// defaultCacheTTL 编译缓存的默认热加载间隔(ttl<=0 时用)。
const defaultCacheTTL = 30 * time.Second

// New 建门房。ttl<=0 时用 defaultCacheTTL。psrc / rec 可为 nil(无策略来源 → ActionFor 恒 ok=false;无记录器 → Record no-op)。
func New(reg *Registry, src RuleSource, psrc PolicyStore, rec decisionRecorder, ttl time.Duration) Doorman {
	if ttl <= 0 {
		ttl = defaultCacheTTL
	}
	return &svc{reg: reg, src: src, psrc: psrc, rec: rec, ttl: ttl, cache: map[string]cachedRules{}, pol: map[string]cachedPolicy{}}
}

// Record 见 Doorman。best-effort:失败静默(统计非关键)。从 Context 取事实 + 命中规则名。
func (s *svc) Record(c *Context, a Assessment, action string) {
	if s.rec == nil || c == nil {
		return
	}
	names := make([]string, 0, len(a.Matched))
	for _, m := range a.Matched {
		names = append(names, m.Name)
	}
	_ = s.rec.recordDecision(&decisionRow{
		Scope:      c.Scope,
		RiskLevel:  string(a.Level),
		Action:     action,
		Matched:    strings.Join(names, ","),
		UA:         c.Attempt.UA,
		IP:         c.Attempt.IP,
		Country:    c.Attempt.Country,
		ISP:        c.Attempt.ISP,
		ASN:        c.Attempt.ASN,
		ASNOrg:     c.Attempt.ASNOrg,
		IsHosting:  c.Attempt.IsHosting,
		Subject:    c.Subject,
		Challenged: c.Subject != "", // 设了关联键 = 判后需回填(如判要激活),进漏斗分母;回填后清 subject 仍靠它计数
	})
}

// MarkOutcome 见 Doorman。best-effort:失败静默;没接存储时 no-op。
func (s *svc) MarkOutcome(scope, subject, outcome string) {
	if s.rec == nil || scope == "" || subject == "" {
		return
	}
	_ = s.rec.markOutcome(scope, subject, outcome)
}

// ActionFor 见 Doorman。按 scope 缓存策略、短 TTL 热加载;读失败沿用旧缓存。
func (s *svc) ActionFor(scope string, level RiskLevel) (string, bool) {
	if s.psrc == nil {
		return "", false
	}
	s.pmu.RLock()
	c, ok := s.pol[scope]
	s.pmu.RUnlock()
	if !ok || time.Since(c.at) >= s.ttl {
		m, err := s.psrc.Policy(scope)
		if err != nil {
			if ok {
				m = c.m // 读失败:沿用旧缓存
			} else {
				return "", false
			}
		}
		c = cachedPolicy{m: m, at: time.Now()}
		s.pmu.Lock()
		s.pol[scope] = c
		s.pmu.Unlock()
	}
	action, found := c.m[string(level)]
	return action, found && action != ""
}

// Assess 见 Doorman。规则来源报错 / 无规则 → RiskNone(无命中);调用方据 Level 处理。
func (s *svc) Assess(c *Context) Assessment {
	rules := s.load(c.Scope)
	if len(rules) == 0 {
		return Assessment{Level: RiskNone}
	}
	return Assess(c, rules)
}

func (s *svc) load(scope string) []*compiledRule {
	s.mu.RLock()
	c, ok := s.cache[scope]
	s.mu.RUnlock()
	if ok && time.Since(c.at) < s.ttl {
		return c.rules
	}

	rules, err := s.src.Rules(scope)
	if err != nil {
		if ok {
			return c.rules // 来源抖动:沿用旧缓存,别因读失败突然全判 none
		}
		return nil
	}
	compiled, _ := s.reg.Compile(rules, scope) // 编译错的规则被跳过(不阻断其它规则)
	s.mu.Lock()
	s.cache[scope] = cachedRules{rules: compiled, at: time.Now()}
	s.mu.Unlock()
	return compiled
}
