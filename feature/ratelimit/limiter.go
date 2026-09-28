// Package ratelimit 是 aurora 防护体系的**计数地基**:按不透明字符串 key,在窗口内做
// 计数 / 失败锁 / 冷却。不认识任何业务概念(login/register/email…),只收 key。
//
// 登录爆破锁(loginguard)、注册/发信/通用请求限频、重发冷却,都是它的不同用法(桶),
// 建其上、共用这一份实现,不各自再数一份。见 doc/security-suite.md。
//
// 运行时 **fail-open**:Redis 抖动时一律放行、记录 no-op —— 限流是次级防护,绝不能因
// 基础设施抖动把所有人挡在门外(与主级安全控制 tokenguard 的 fail-close 刻意相反)。
package ratelimit

import (
	"context"
	"strconv"
	"time"

	auroraFeature "github.com/shyandsy/aurora/feature"
	"github.com/shyandsy/aurora/logger"
)

// Outcome 一次限流判定的结果。
type Outcome struct {
	Count      int64 // 当前窗口计数
	Over       bool  // 是否已超限(调用方据此决定拦不拦)
	Remaining  int   // 距上限还剩几次(不为负;上限<=0 时为 0)
	RetryAfter int64 // 建议等待秒数(锁/窗口未过);0 = 无需等
}

// limiter 是计数地基的底层原语(按 key 操作),包内私有:业务只通过上层 Service(桶+维度)用,
// 不直接碰 key。三类原语:Hit/Peek(计数+窗口)、Fail/LockState/Clear(失败锁)、Cooldown(冷却)。

// redis 能力直接依赖 redis feature 的 RedisService(契约的单一真源在 redis feature,
// 不在此另抄一份最小接口)。单测用内嵌 RedisService、只实现用到的几个方法的假实现。
type limiter struct{ redis auroraFeature.RedisService }

func newLimiter(r auroraFeature.RedisService) *limiter { return &limiter{redis: r} }

// untilSuffix 拼 until 伴生 key(与主计数器同 TTL,存窗口/锁的解除时刻,供 RetryAfter;RedisService 读不到 TTL)。
const untilSuffix = ":until"

func (l *limiter) Hit(ctx context.Context, key string, window time.Duration, limit int) Outcome {
	if l == nil || l.redis == nil || key == "" {
		return Outcome{} // fail-open
	}
	c := l.incr(ctx, key, window)
	if c == 1 {
		_, _ = l.redis.SetNX(ctx, key+untilSuffix, unixAfter(window), window)
	}
	o := Outcome{Count: c, Remaining: remaining(limit, c)}
	if limit > 0 && c > int64(limit) {
		o.Over = true
		o.RetryAfter = l.retryAfter(ctx, key+untilSuffix, window)
	}
	return o
}

func (l *limiter) Peek(ctx context.Context, key string, window time.Duration, limit int) Outcome {
	if l == nil || l.redis == nil || key == "" {
		return Outcome{}
	}
	c := l.count(ctx, key)
	o := Outcome{Count: c, Remaining: remaining(limit, c)}
	if limit > 0 && c > int64(limit) {
		o.Over = true
		o.RetryAfter = l.retryAfter(ctx, key+untilSuffix, window)
	}
	return o
}

func (l *limiter) Fail(ctx context.Context, failKey, lockKey string, window time.Duration, limit, lockSeconds int) Outcome {
	if l == nil || l.redis == nil || failKey == "" {
		return Outcome{}
	}
	c := l.incr(ctx, failKey, window)
	o := Outcome{Count: c, Remaining: remaining(limit, c)}
	if limit > 0 && c > int64(limit) {
		o.Over = true
		if lockSeconds > 0 && lockKey != "" {
			lockDur := time.Duration(lockSeconds) * time.Second
			// 锁 value 存解除时刻,供后续 LockState 算 RetryAfter(RedisService 读不到 TTL)。
			_, _ = l.redis.SetNX(ctx, lockKey, unixAfter(lockDur), lockDur)
			o.RetryAfter = int64(lockSeconds)
		}
	}
	return o
}

func (l *limiter) LockState(ctx context.Context, lockKey string) (bool, int64) {
	if l == nil || l.redis == nil || lockKey == "" {
		return false, 0 // fail-open:读不到锁 = 放行
	}
	raw, err := l.redis.Get(ctx, lockKey)
	if err != nil || raw == "" {
		return false, 0
	}
	return true, retryAfterFrom(raw, time.Minute)
}

func (l *limiter) Clear(ctx context.Context, keys ...string) {
	if l == nil || l.redis == nil || len(keys) == 0 {
		return
	}
	_, _ = l.redis.Delete(ctx, keys...)
}

func (l *limiter) Cooldown(ctx context.Context, key string, gap time.Duration) (bool, int64) {
	if l == nil || l.redis == nil || key == "" || gap <= 0 {
		return true, 0 // fail-open:放行
	}
	// SetNX 原子占位:创建成功 = 不在冷却期(放行);已存在 = 冷却中(拦),从 value 算还要等多久。
	created, err := l.redis.SetNX(ctx, key, unixAfter(gap), gap)
	if err != nil {
		return true, 0 // fail-open
	}
	if created {
		return true, 0
	}
	raw, _ := l.redis.Get(ctx, key)
	return false, retryAfterFrom(raw, gap)
}

// incrTTLScript 原子「INCR + 首次 PEXPIRE」:INCR 与 PEXPIRE 在 Redis 服务端一条 Lua 脚本内完成,
// 彻底杜绝"两条命令之间 key 过期被重建成无 TTL"的竞态——否则无 TTL 的计数永不衰减、累加到阈值后
// 永久锁死该 IP/账号,而受害者被锁又无法成功登录去清计数 = 自我 DoS(单次概率低,海量 key 下迟早发生)。
//
// TTL 用**毫秒**(PEXPIRE)而非整秒(EXPIRE):窗口按 time.Duration 如实设置,不向下截断。
// 否则亚秒窗口(如 500ms)会被 int64(Seconds()) 截成 0 → EXPIRE key 0 = Redis 立即删键 →
// 计数永远卡在 1、Over 永不触发 = 该桶静默失效。毫秒也与本包其余 SetNX(go-redis 亚秒走 PX)一致。
const incrTTLScript = `local c = redis.call('INCR', KEYS[1])
if c == 1 then redis.call('PEXPIRE', KEYS[1], ARGV[1]) end
return c`

// incr 计数 +1,并原子保证 key 首次创建即带 TTL(见 incrTTLScript)。
// fail-open:出错记日志、返回 0(不触发超限/锁)。
func (l *limiter) incr(ctx context.Context, key string, ttl time.Duration) int64 {
	ms := ttl.Milliseconds()
	if ms < 1 {
		ms = 1 // 兜底:亚毫秒窗口(非现实配置)至少给 1ms,绝不让 PEXPIRE 0 立即删键
	}
	v, err := l.redis.Eval(ctx, incrTTLScript, []string{key}, ms)
	if err != nil {
		logger.Errorf("ratelimit: incr(lua) %s 失败(fail-open): %v", key, err)
		return 0
	}
	c, _ := v.(int64) // Lua 整数 → Redis integer → int64
	return c
}

func (l *limiter) count(ctx context.Context, key string) int64 {
	v, err := l.redis.Get(ctx, key)
	if err != nil || v == "" {
		return 0
	}
	n, _ := strconv.ParseInt(v, 10, 64)
	return n
}

func (l *limiter) retryAfter(ctx context.Context, untilKey string, fallback time.Duration) int64 {
	raw, _ := l.redis.Get(ctx, untilKey)
	return retryAfterFrom(raw, fallback)
}

// ── 无状态工具 ──

func remaining(limit int, used int64) int {
	if limit <= 0 {
		return 0
	}
	if r := int64(limit) - used; r > 0 {
		return int(r)
	}
	return 0
}

// unixAfter 返回"从现在起 d 之后"的 unix 秒(字符串),存进 until/锁 value。
func unixAfter(d time.Duration) string {
	return strconv.FormatInt(time.Now().Add(d).Unix(), 10)
}

// retryAfterFrom 从解除时刻字符串算还要等几秒;解析失败退回 fallback;至少 1。
func retryAfterFrom(raw string, fallback time.Duration) int64 {
	if ts, err := strconv.ParseInt(raw, 10, 64); err == nil {
		if remain := ts - time.Now().Unix(); remain > 0 {
			return remain
		}
		return 1
	}
	if fallback > 0 {
		return int64(fallback.Seconds())
	}
	return 1
}
