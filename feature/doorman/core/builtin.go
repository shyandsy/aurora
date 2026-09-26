package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

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
	TypeRateLimit  = "rate_limit"  // 条件:某维度(IP/ASN/标识)在时间窗内的次数超阈值(有状态,需注入 Store)
)

// 内置条件配置字段的 key。同一个 key 既在 Fields() 声明、又是 Compile 里 JSON 的字段名,提成常量对齐。
const (
	keyPatterns  = "patterns"  // ua_match
	keyCountries = "countries" // country_in
	keyBy        = "by"        // rate_limit:按哪个维度分桶
	keyWindow    = "window"    // rate_limit:时间窗(如 "1h")
	keyMax       = "max"       // rate_limit:窗内允许的次数阈值(超过即命中)
)

// rate_limit 的分桶维度取值(配置页下拉候选)。
const (
	RateByIP      = "ip"      // 按客户端 IP
	RateByASN     = "asn"     // 按自治系统号
	RateBySubject = "subject" // 按业务关联键(Context.Subject,如规整后的邮箱)
)

// RegisterBuiltins 注册**业务中性**的通用条件:纯函数条件(ua/asn/country)+ 有状态限流(rate_limit)。
// 业务可再补自己的(WithCondition)。rate_limit 需业务注入 Context.Store(如 Redis);没注入时它恒不命中。
// doorman 不含任何「动作」。
func RegisterBuiltins(reg *Registry) {
	reg.RegisterCondition(uaMatch{})
	reg.RegisterCondition(asnHosting{})
	reg.RegisterCondition(countryIn{})
	reg.RegisterCondition(rateLimit{})
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

// ── 条件:rate_limit —— 某维度(IP/ASN/标识)在时间窗内出现次数超阈值即命中。有状态,靠 Context.Store 计数。──
// 中性:doorman 只数「同一个桶在窗内第几次」,不认识这是登录还是注册,更不做拦截——命中只贡献风险等级。
type rateLimit struct{}

func (rateLimit) Type() string { return TypeRateLimit }
func (rateLimit) Fields() []dto.Field {
	return []dto.Field{
		{Key: keyBy, Type: FieldSelect, LabelKey: "doorman.field.rateBy", Required: true,
			Options: []string{RateByIP, RateByASN, RateBySubject}},
		{Key: keyWindow, Type: FieldDuration, LabelKey: "doorman.field.rateWindow", Required: true},
		{Key: keyMax, Type: FieldInt, LabelKey: "doorman.field.rateMax", Required: true},
	}
}
func (rateLimit) Compile(raw json.RawMessage) (Check, error) {
	var c struct {
		By     string `json:"by"`     // 对齐 keyBy
		Window string `json:"window"` // 对齐 keyWindow(如 "1h")
		Max    int    `json:"max"`    // 对齐 keyMax
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, err
	}
	by := strings.TrimSpace(c.By)
	if by != RateByIP && by != RateByASN && by != RateBySubject {
		return nil, fmt.Errorf("%s 必须是 %s/%s/%s 之一", keyBy, RateByIP, RateByASN, RateBySubject)
	}
	window, err := time.ParseDuration(strings.TrimSpace(c.Window))
	if err != nil || window <= 0 {
		return nil, fmt.Errorf("%s 非法(需正的时长,如 \"1h\"):%v", keyWindow, c.Window)
	}
	if c.Max < 1 {
		return nil, fmt.Errorf("%s 需 >=1", keyMax)
	}
	max := int64(c.Max)
	return func(g *Context) bool {
		if g.Store == nil { // 没注入计数存储:限流条件恒不命中(best-effort,不误伤)
			return false
		}
		val := rateBucketValue(by, g)
		if val == "" { // 该维度取不到值(如按 subject 但没设):无从计数,不命中
			return false
		}
		key := "doorman:rl:" + g.Scope + ":" + by + ":" + val
		n, err := g.Store.Incr(g.Ctx, key, window)
		if err != nil { // 计数存储抖动:失败开放(不因基础设施故障而突然多判风险)
			return false
		}
		return n > max // 第 max+1 次起命中
	}, nil
}

// rateBucketValue 取限流分桶的值(按 by 维度从 Attempt / Subject 取)。取不到返回空串。
func rateBucketValue(by string, g *Context) string {
	switch by {
	case RateByIP:
		return g.Attempt.IP
	case RateByASN:
		if g.Attempt.ASN == 0 {
			return ""
		}
		return strconv.FormatUint(uint64(g.Attempt.ASN), 10)
	case RateBySubject:
		return g.Subject
	}
	return ""
}
