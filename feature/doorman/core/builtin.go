package core

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/shyandsy/aurora/feature/doorman/model/dto"
)

// Field.Type 的取值:配置页据此渲染控件。集中定义,避免各插件散落魔法字符串。
const (
	FieldString     = "string"
	FieldStringList = "string_list"
	FieldInt        = "int"
	FieldIntList    = "int_list"
	FieldDuration   = "duration"
	FieldSelect     = "select"
)

// 内置条件的类别标识。导出成常量:插件 Type()、业务配规则、配置页下拉都引它,不各写各的裸串。
const (
	TypeUAMatch    = "ua_match"    // 条件:UA 命中关键词
	TypeASNHosting = "asn_hosting" // 条件:IP 属机房/云/Tor
	TypeCountryIn  = "country_in"  // 条件:国家命中集合
)

// 内置条件配置字段的 key。同一个 key 既在 Fields() 声明、又是 Compile 里 JSON 的字段名,提成常量对齐。
const (
	keyPatterns  = "patterns"  // ua_match
	keyCountries = "countries" // country_in
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
func (uaMatch) Fields() []dto.Field {
	return []dto.Field{{Key: keyPatterns, Type: FieldStringList, LabelKey: "doorman.field.uaPatterns", Required: true}}
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

func (asnHosting) Type() string        { return TypeASNHosting }
func (asnHosting) Fields() []dto.Field { return nil }
func (asnHosting) Compile(json.RawMessage) (Check, error) {
	return func(g *Context) bool { return g.Attempt.IsHosting }, nil
}

// ── 条件:country_in —— 国家命中集合(ISO alpha-2,大小写不敏感)。──
type countryIn struct{}

func (countryIn) Type() string { return TypeCountryIn }
func (countryIn) Fields() []dto.Field {
	return []dto.Field{{Key: keyCountries, Type: FieldStringList, LabelKey: "doorman.field.countries", Required: true}}
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
