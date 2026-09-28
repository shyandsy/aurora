package ratelimit

import (
	"sort"
	"strings"
	"time"
)

// kind 桶种类(**内部**):由 New*Bucket 构造器设定,消费方经**句柄类型**(CountBucket 等)体现,不直接碰。
type kind int

const (
	kindCount    kind = iota + 1 // 计数+窗口(Hit/Peek)
	kindFailLock                 // 失败计数→短锁(Fail/Locked/ClearFail)
	kindCooldown                 // 冷却/最小间隔(Cooldown)
)

// bucketDef 桶的形状(**内部**)。消费方不直接建,而是用 New*Bucket 拿类型化句柄。
type bucketDef struct {
	name    string
	kind    kind
	keyDims []string
}

// Bucket 是三种桶句柄的公共接口(供 WithBucket 注册)。方法未导出 → 只有本包的三种句柄实现它,
// 外部无法自造桶种类。消费方拿到的是**具体类型**(CountBucket / FailLockBucket / CooldownBucket),
// 传给对应的 Service 方法——传错方法(如把 CountBucket 传给 Fail)= **编译错**。
type Bucket interface{ def() bucketDef }

// CountBucket 计数桶句柄(用 Hit/Peek)。
type CountBucket struct{ d bucketDef }

// FailLockBucket 失败锁桶句柄(用 Fail/Locked/ClearFail)。
type FailLockBucket struct{ d bucketDef }

// CooldownBucket 冷却桶句柄(用 Cooldown)。
type CooldownBucket struct{ d bucketDef }

func (b CountBucket) def() bucketDef    { return b.d }
func (b FailLockBucket) def() bucketDef { return b.d }
func (b CooldownBucket) def() bucketDef { return b.d }

// NewCountBucket 声明一个计数桶(Hit/Peek)。name 唯一(也是 LimitsProvider 查阈值的键);
// keyDims 组 key 的维度顺序(ip/account/email/route…),值由调用方在调用时按维度传。
// 通常声明为包变量,注册(WithBucket)与调用点(Service)复用同一句柄 → 桶名 typo 不可能。
func NewCountBucket(name string, keyDims ...string) CountBucket {
	return CountBucket{bucketDef{name: name, kind: kindCount, keyDims: keyDims}}
}

// NewFailLockBucket 声明一个失败锁桶(Fail/Locked/ClearFail)。
func NewFailLockBucket(name string, keyDims ...string) FailLockBucket {
	return FailLockBucket{bucketDef{name: name, kind: kindFailLock, keyDims: keyDims}}
}

// NewCooldownBucket 声明一个冷却桶(Cooldown)。
func NewCooldownBucket(name string, keyDims ...string) CooldownBucket {
	return CooldownBucket{bucketDef{name: name, kind: kindCooldown, keyDims: keyDims}}
}

// Limits 是一个桶的**值**(阈值)。各字段按桶种类取用:计数桶用 Window+Limit;失败锁另用 LockSeconds;冷却用 Gap。
type Limits struct {
	Window      time.Duration
	Limit       int
	LockSeconds int
	Gap         time.Duration
}

// LimitsProvider 提供各桶的阈值(依赖倒置:ratelimit 不认任何项目的设置结构)。
// 每次判定都会调它 —— 支持运行时改阈值,故实现应**内存缓存,绝不在此查 DB**。
type LimitsProvider interface {
	Limits(bucket string) Limits
}

// StaticLimits 把一份固定阈值表封成 LimitsProvider(阈值编译期定、不在线调的项目/桶用)。
func StaticLimits(m map[string]Limits) LimitsProvider { return staticLimitsProvider{m: m} }

type staticLimitsProvider struct{ m map[string]Limits }

func (s staticLimitsProvider) Limits(bucket string) Limits { return s.m[bucket] }

// escapeDim 转义维度**值**里会破坏 `dim=val:dim=val` 结构的字符(`%` `:` `=` → 百分号编码),
// 保证 key 对 (dims, values) **单射**——否则 IPv6 值自带的冒号、或 crafted 值里的 `:`/`=` 会冲乱
// key 结构、串到别的桶(破坏计数完整性 = 本 feature 的全部意义)。维度**名**来自代码声明的 keyDims,无需转义。
func escapeDim(s string) string {
	if !strings.ContainsAny(s, "%:=") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 8)
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '%':
			b.WriteString("%25")
		case ':':
			b.WriteString("%3A")
		case '=':
			b.WriteString("%3D")
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// keyBase 组一个桶+维度的 namespaced 基 key:`rate_limit:<namespace>:<bucket>[:dim=escVal...]`。
// 维度按 bucketDef.keyDims 声明顺序拼(缺的跳过);dims 里多给的维度排序后附上(稳定)。值走 escapeDim 防冲突。
// namespace 前缀把不同服务/realm 的 key 由构造隔离(多服务共用一个 Redis DB 也不撞键)。
// 前缀用完整的 rate_limit(不缩写成 rl):redis key 要运维肉眼看/grep(手动解锁),且与既有 key 约定一致。
func keyBase(ns string, d bucketDef, dims map[string]string) string {
	var sb strings.Builder
	sb.WriteString("rate_limit:")
	sb.WriteString(ns)
	sb.WriteString(":")
	sb.WriteString(d.name)
	seen := map[string]bool{}
	for _, dim := range d.keyDims {
		if v, ok := dims[dim]; ok {
			sb.WriteString(":")
			sb.WriteString(dim)
			sb.WriteString("=")
			sb.WriteString(escapeDim(v))
			seen[dim] = true
		}
	}
	extra := make([]string, 0)
	for dim := range dims {
		if !seen[dim] {
			extra = append(extra, dim)
		}
	}
	sort.Strings(extra)
	for _, dim := range extra {
		sb.WriteString(":")
		sb.WriteString(dim)
		sb.WriteString("=")
		sb.WriteString(escapeDim(dims[dim]))
	}
	return sb.String()
}
