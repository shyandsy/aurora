package ratelimit

// 用 miniredis(进程内、支持 EVAL/PEXPIRE/TTL)做集成测试:验证原子 Lua incr 的真实语义
// —— 普通假 Redis 不跑 Lua、不追踪 TTL,测不到"首次 INCR 落 TTL、不会出现无 TTL 永久 key"这个修复点。

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"

	auroraFeature "github.com/shyandsy/aurora/feature"
)

// mrOps 把 go-redis client 适配成 RedisService(内嵌接口只实现 ratelimit 用到的方法,连 miniredis)。
type mrOps struct {
	auroraFeature.RedisService
	c *redis.Client
}

func (m mrOps) Get(ctx context.Context, key string) (string, error) {
	return m.c.Get(ctx, key).Result() // 缺失返回 redis.Nil(err),与真 RedisService 一致;limiter 按 err 处理
}
func (m mrOps) Delete(ctx context.Context, keys ...string) (int64, error) {
	return m.c.Del(ctx, keys...).Result()
}
func (m mrOps) SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	return m.c.SetNX(ctx, key, value, ttl).Result()
}
func (m mrOps) Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error) {
	return m.c.Eval(ctx, script, keys, args...).Result()
}

func newMiniredis(t *testing.T) (*miniredis.Miniredis, auroraFeature.RedisService) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis 启动失败: %v", err)
	}
	t.Cleanup(mr.Close)
	c := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = c.Close() })
	return mr, mrOps{c: c}
}

// ── 原子 Lua incr:首次 INCR 就落 TTL(修复点)、后续递增仍带 TTL、TTL 过期后从 1 重来 ──
func TestIncr_AtomicTTL_MiniRedis(t *testing.T) {
	ctx := context.Background()
	mr, ops := newMiniredis(t)
	l := newLimiter(ops)
	const key = "rate_limit:user:login_ip_fail:ip=1.2.3.4:fail"

	if c := l.incr(ctx, key, 5*time.Second); c != 1 {
		t.Fatalf("首次 incr 应=1,got %d", c)
	}
	// 关键断言:首次 INCR 原子地落上了 TTL —— 不会出现"无 TTL 永久 key → 永久锁死"的自我 DoS。
	if ttl := mr.TTL(key); ttl <= 0 || ttl > 5*time.Second {
		t.Fatalf("首次 incr 应落 ~5s TTL(证明 INCR+PEXPIRE 原子),got %v", ttl)
	}
	if c := l.incr(ctx, key, 5*time.Second); c != 2 {
		t.Fatalf("再次 incr 应=2,got %d", c)
	}
	if ttl := mr.TTL(key); ttl <= 0 {
		t.Fatal("再次 incr 后 key 仍应有 TTL(不该被变成永久 key)")
	}

	mr.FastForward(6 * time.Second) // 越过 TTL,key 过期消失
	if c := l.incr(ctx, key, 5*time.Second); c != 1 {
		t.Fatalf("TTL 过期后应重新从 1 起,got %d", c)
	}
}

// ── 亚秒窗口(<1s):PEXPIRE 用毫秒,key 不被立即删、计数能累加到超限 ──
// 回归此前的整秒截断坑:int64(ttl.Seconds()) 把 500ms 截成 0 → EXPIRE key 0 = 立即删键 →
// 计数永远卡 1、Over 永不触发 = 该桶静默失效。现改 PEXPIRE + Milliseconds 后不再中招。
func TestIncr_SubSecondWindow_MiniRedis(t *testing.T) {
	ctx := context.Background()
	mr, ops := newMiniredis(t)
	l := newLimiter(ops)
	const key = "rate_limit:svc:burst:ip=1.2.3.4"

	if c := l.incr(ctx, key, 500*time.Millisecond); c != 1 {
		t.Fatalf("首次 incr 应=1,got %d", c)
	}
	// 关键:亚秒窗口的 key 必须仍带正 TTL、没被 PEXPIRE 0 当场删掉。
	if ttl := mr.TTL(key); ttl <= 0 || ttl > 500*time.Millisecond {
		t.Fatalf("500ms 窗口应落 ~500ms TTL(不被立即删),got %v", ttl)
	}
	// 计数必须能累加(整秒截断坑下会永远卡在 1)。
	if c := l.incr(ctx, key, 500*time.Millisecond); c != 2 {
		t.Fatalf("再次 incr 应=2(计数能累加,不卡 1),got %d", c)
	}
	mr.FastForward(600 * time.Millisecond) // 越过亚秒窗口
	if c := l.incr(ctx, key, 500*time.Millisecond); c != 1 {
		t.Fatalf("亚秒窗口过期后应重新从 1 起,got %d", c)
	}
}

// ── 失败锁桶走真 redis 语义:失败超阈值上锁、Locked 命中带 RetryAfter、ClearFail 清计数 ──
func TestService_FailLock_MiniRedis(t *testing.T) {
	ctx := context.Background()
	_, ops := newMiniredis(t)
	b := NewFailLockBucket("login_ip_fail", "ip")
	p := StaticLimits(map[string]Limits{"login_ip_fail": {Window: 5 * time.Minute, Limit: 3, LockSeconds: 900}})
	s := newService("user", p, newLimiter(ops))
	dims := map[string]string{"ip": "1.2.3.4"}

	for i := 0; i < 3; i++ {
		if o := s.Fail(ctx, b, dims); o.Over {
			t.Fatalf("第%d次不该 over", i+1)
		}
	}
	if o := s.Fail(ctx, b, dims); !o.Over {
		t.Fatal("第4次(>3)应 over")
	}
	if locked, ra := s.Locked(ctx, b, dims); !locked || ra <= 0 {
		t.Fatalf("应已锁 + RetryAfter>0,got %v %d", locked, ra)
	}
	s.ClearFail(ctx, b, dims)
	if o := s.Fail(ctx, b, dims); o.Count != 1 {
		t.Fatalf("清失败计数后应从 1 起,got %+v", o)
	}
}

// ── 冷却桶走真 redis:gap 内重复被拦 ──
func TestService_Cooldown_MiniRedis(t *testing.T) {
	ctx := context.Background()
	_, ops := newMiniredis(t)
	b := NewCooldownBucket("email_resend", "email")
	p := StaticLimits(map[string]Limits{"email_resend": {Gap: 60 * time.Second}})
	s := newService("user", p, newLimiter(ops))
	dims := map[string]string{"email": "a@x"}

	if ok, _ := s.Cooldown(ctx, b, dims); !ok {
		t.Fatal("首次应放行")
	}
	if ok, ra := s.Cooldown(ctx, b, dims); ok || ra <= 0 {
		t.Fatalf("gap 内重复应被拦 + RetryAfter>0,got ok=%v ra=%d", ok, ra)
	}
}
