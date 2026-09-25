package doorman

import (
	"encoding/json"
	"errors"
	"strings"
)

// RegisterBuiltins 注册**业务中性**的通用条件。业务可再补自己的(WithCondition)。
// 只含纯函数条件(不依赖 Store);限流(有状态条件)后续补。doorman 不含任何「动作」。
func RegisterBuiltins(reg *Registry) {
	reg.RegisterCondition(uaMatch{})
	reg.RegisterCondition(asnHosting{})
	reg.RegisterCondition(countryIn{})
}

// ── 条件:ua_match —— UA 子串命中任一关键词(空串关键词 = 匹配空 UA)。抓 Go-http-client/python 之类脚本。──
type uaMatch struct{}

func (uaMatch) Type() string { return TypeUAMatch }
func (uaMatch) Fields() []Field {
	return []Field{{Key: keyPatterns, Type: FieldStringList, LabelKey: "doorman.field.uaPatterns", Required: true}}
}
func (uaMatch) Compile(raw json.RawMessage) (Check, error) {
	var c struct {
		Patterns []string `json:"patterns"` // 对齐 keyPatterns
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, err
	}
	if len(c.Patterns) == 0 {
		return nil, errors.New(keyPatterns + " 不能为空")
	}
	pats := make([]string, len(c.Patterns))
	for i, p := range c.Patterns {
		pats[i] = strings.ToLower(strings.TrimSpace(p))
	}
	return func(g *Context) bool {
		ua := strings.ToLower(g.Attempt.UA)
		for _, p := range pats {
			if p == "" {
				if strings.TrimSpace(g.Attempt.UA) == "" {
					return true // 空关键词专门匹配「空 UA」
				}
				continue
			}
			if strings.Contains(ua, p) {
				return true
			}
		}
		return false
	}, nil
}

// ── 条件:asn_hosting —— IP 属机房/云/托管/Tor 出口(读 Attempt.IsHosting 一个 bool,无配置)。──
type asnHosting struct{}

func (asnHosting) Type() string    { return TypeASNHosting }
func (asnHosting) Fields() []Field { return nil }
func (asnHosting) Compile(json.RawMessage) (Check, error) {
	return func(g *Context) bool { return g.Attempt.IsHosting }, nil
}

// ── 条件:country_in —— 国家命中集合(ISO alpha-2,大小写不敏感)。──
type countryIn struct{}

func (countryIn) Type() string { return TypeCountryIn }
func (countryIn) Fields() []Field {
	return []Field{{Key: keyCountries, Type: FieldStringList, LabelKey: "doorman.field.countries", Required: true}}
}
func (countryIn) Compile(raw json.RawMessage) (Check, error) {
	var c struct {
		Countries []string `json:"countries"` // 对齐 keyCountries
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, err
	}
	if len(c.Countries) == 0 {
		return nil, errors.New(keyCountries + " 不能为空")
	}
	set := make(map[string]bool, len(c.Countries))
	for _, cc := range c.Countries {
		set[strings.ToUpper(strings.TrimSpace(cc))] = true
	}
	return func(g *Context) bool {
		return g.Attempt.Country != "" && set[strings.ToUpper(g.Attempt.Country)]
	}, nil
}
