package core

import (
	"strings"
	"sync"
	"time"
)

// svc 是 Doorman 的实现:持有注册表 + 规则来源 + 策略来源,编译/策略结果按 scope 缓存并短 TTL 热加载。
// 决策落库经 DecisionRecorder(领域契约),不认识 gorm 实体。
type svc struct {
	reg   *Registry
	src   RuleSource
	psrc  PolicyStore      // 「风险→动作」策略来源;可为 nil(ActionFor 一律 ok=false)
	rec   DecisionRecorder // 决策流水记录;可为 nil(Record 是 no-op)
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
func New(reg *Registry, src RuleSource, psrc PolicyStore, rec DecisionRecorder, ttl time.Duration) Doorman {
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
	_ = s.rec.Record(Decision{
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
	_ = s.rec.MarkOutcome(scope, subject, outcome)
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
