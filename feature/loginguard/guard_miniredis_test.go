package loginguard

// 用 miniredis(进程内真 redis 语义:EVAL/PEXPIRE/TTL)做端到端集成测试:验证 loginguard 建在
// ratelimit 引擎上后,失败累加→上锁、成功→清计数 走真 redis 都对;并守住"失败键带 TTL、非永久键"
// 这条反自我-DoS 回归(假库不跑 Lua、不追踪 TTL,测不到)。原子性本身在 ratelimit 包的 miniredis 测里。

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"

	auroraFeature "github.com/shyandsy/aurora/feature"
)

// mrOps 把 go-redis client 适配成 RedisService(内嵌接口只实现引擎用到的方法,连 miniredis)。
type mrOps struct {
	auroraFeature.RedisService
	c *redis.Client
}

func (m mrOps) Get(ctx context.Context, key string) (string, error) {
	return m.c.Get(ctx, key).Result() // 缺失返回 redis.Nil(err),与真 RedisService 一致
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

// ── 走真 redis:失败累加→IP 锁,且失败键原子带 TTL(非永久键);另一 IP 上成功能清失败 ──
func TestGuard_Integration_MiniRedis(t *testing.T) {
	ctx := context.Background()
	mr, ops := newMiniredis(t)
	g := newG(ops, hardLockPolicy()) // IPFailLimit=2

	for i := 0; i < 3; i++ { // 3 > 2 → 锁
		g.RecordFailure(ctx, ip, "")
	}
	if d := g.PrecheckIP(ctx, ip); !d.Blocked || d.Reason != "ip_locked" {
		t.Fatalf("超阈值后 IP 应被锁: %+v", d)
	}
	// 反自我-DoS 回归:失败计数键必须带正 TTL,不能是永久键(否则永不衰减 = 永久锁死)。
	assertFailKeyHasTTL(t, mr)

	// 另一个干净 IP:失败一次 → remaining 减少;成功 → 清回满额(走真 redis 的 clear)。
	const ip2 = "5.6.7.8"
	g.RecordFailure(ctx, ip2, "")
	if d := g.PrecheckIP(ctx, ip2); d.IPFailRemaining != 1 {
		t.Fatalf("ip2 失败一次后 remaining 应=1,got %d", d.IPFailRemaining)
	}
	g.RecordSuccess(ctx, ip2, "")
	if d := g.PrecheckIP(ctx, ip2); d.Blocked || d.IPFailRemaining != 2 {
		t.Fatalf("ip2 成功后应清失败(remaining 回满 2、不锁): %+v", d)
	}
}

// assertFailKeyHasTTL:扫 miniredis 里的 login_ip_fail 计数键,断言其带正 TTL(非永久)。
func assertFailKeyHasTTL(t *testing.T, mr *miniredis.Miniredis) {
	t.Helper()
	for _, k := range mr.Keys() {
		if strings.Contains(k, "login_ip_fail") && strings.HasSuffix(k, ":fail") {
			if ttl := mr.TTL(k); ttl <= 0 {
				t.Fatalf("失败计数键 %q 应带正 TTL(非永久键),got %v", k, ttl)
			}
			return
		}
	}
	t.Fatal("未找到 login_ip_fail 计数键(失败未被记录?)")
}
