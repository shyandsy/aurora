package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	auroraFeature "github.com/shyandsy/aurora/feature"
)

// TestStaticLimits_NilableForProvideAs 锁住:StaticLimits 必须返回可空(指针)。
// di.ProvideAs 注册实例时 reflect.ValueOf(x).IsNil();结构体值会 panic → 装配即崩(build 不报、上线才炸)。
func TestStaticLimits_NilableForProvideAs(t *testing.T) {
	v := reflect.ValueOf(StaticLimits(map[string]Limits{}))
	if v.Kind() != reflect.Ptr {
		t.Fatalf("StaticLimits 必须返回指针(可空),否则 di.ProvideAs 会 panic;got kind %v", v.Kind())
	}
	if v.IsNil() {
		t.Fatal("StaticLimits 返回不该为 nil")
	}
}

// ── 内存假 Redis:内嵌 RedisService(契约真源),只实现本包用到的 Get/Delete/SetNX/Eval;
// 其余方法为 nil 接口,单测不会调到。failMode 下全报错,测 fail-open。──
type fakeRedis struct {
	auroraFeature.RedisService
	m        map[string]string
	failMode bool
}

func newFake() *fakeRedis { return &fakeRedis{m: map[string]string{}} }

var errFake = errors.New("fake redis down")

func (f *fakeRedis) Get(_ context.Context, key string) (string, error) {
	if f.failMode {
		return "", errFake
	}
	return f.m[key], nil
}
func (f *fakeRedis) Delete(_ context.Context, keys ...string) (int64, error) {
	if f.failMode {
		return 0, errFake
	}
	var n int64
	for _, k := range keys {
		if _, ok := f.m[k]; ok {
			delete(f.m, k)
			n++
		}
	}
	return n, nil
}
func (f *fakeRedis) SetNX(_ context.Context, key string, value interface{}, _ time.Duration) (bool, error) {
	if f.failMode {
		return false, errFake
	}
	if _, ok := f.m[key]; ok {
		return false, nil
	}
	f.m[key] = fmt.Sprintf("%v", value)
	return true, nil
}

// Eval 只需模拟本包用到的 INCR+PEXPIRE 脚本:对 keys[0] 自增(PEXPIRE 在假库里 no-op,不追踪 TTL)。
// 真实原子性/TTL 见 limiter_miniredis_test.go(假库测不到)。
func (f *fakeRedis) Eval(_ context.Context, _ string, keys []string, _ ...interface{}) (interface{}, error) {
	if f.failMode {
		return nil, errFake
	}
	var n int64
	fmt.Sscanf(f.m[keys[0]], "%d", &n)
	n++
	f.m[keys[0]] = fmt.Sprintf("%d", n)
	return n, nil
}

// 测试用桶句柄(类型化)。
var (
	bCount    = NewCountBucket("reg_ip_hour", "ip")
	bFailLock = NewFailLockBucket("login_ip_fail", "ip")
	bCooldown = NewCooldownBucket("email_resend", "email")
)

func testService(ns string, r auroraFeature.RedisService) *service {
	p := StaticLimits(map[string]Limits{
		"reg_ip_hour":   {Window: time.Hour, Limit: 2},
		"login_ip_fail": {Window: 5 * time.Minute, Limit: 3, LockSeconds: 900},
		"email_resend":  {Gap: 60 * time.Second},
	})
	return newService(ns, p, newLimiter(r))
}

func ipDims() map[string]string    { return map[string]string{"ip": "1.2.3.4"} }
func emailDims() map[string]string { return map[string]string{"email": "a@x"} }

// ── key 组装 + 维度值转义(防冲突,含 IPv6 冒号)──
func TestKeyBase(t *testing.T) {
	d := NewFailLockBucket("login_ip_fail", "ip").d
	if got := keyBase("user", d, map[string]string{"ip": "1.2.3.4"}); got != "rate_limit:user:login_ip_fail:ip=1.2.3.4" {
		t.Fatalf("key 格式不对: %s", got)
	}
}

func TestKeyBase_EscapesValues(t *testing.T) {
	// IPv6 值自带冒号 → 必须转义(否则冲乱 key 结构)
	single := NewCountBucket("b", "ip").d
	if k := keyBase("user", single, map[string]string{"ip": "2001:db8::1"}); !strings.Contains(k, "%3A") || strings.Contains(k, "db8::1") {
		t.Fatalf("IPv6 冒号应被转义,got %s", k)
	}
	// crafted 单维度值不能和多维度组合撞同一 key
	multi := NewCountBucket("b", "ip", "route").d
	kMulti := keyBase("user", multi, map[string]string{"ip": "1.2.3.4", "route": "/login"})
	kInject := keyBase("user", single, map[string]string{"ip": "1.2.3.4:route=/login"})
	if kMulti == kInject {
		t.Fatalf("crafted 值不转义会与多维度 key 冲突:%s == %s", kMulti, kInject)
	}
}

// ── 计数桶:超限 + remaining ──
func TestService_HitCount(t *testing.T) {
	s := testService("user", newFake())
	if o := s.Hit(context.Background(), bCount, ipDims()); o.Over || o.Remaining != 1 {
		t.Fatalf("第1次不该超限、剩1,got %+v", o)
	}
	if o := s.Hit(context.Background(), bCount, ipDims()); o.Over || o.Remaining != 0 {
		t.Fatalf("第2次(=上限2)不超限、剩0,got %+v", o)
	}
	if o := s.Hit(context.Background(), bCount, ipDims()); !o.Over {
		t.Fatalf("第3次应超限,got %+v", o)
	}
}

// ── 失败锁桶:超阈值上锁 + Locked + ClearFail ──
func TestService_FailLock(t *testing.T) {
	ctx := context.Background()
	s := testService("user", newFake())
	for i := 0; i < 3; i++ {
		if o := s.Fail(ctx, bFailLock, ipDims()); o.Over {
			t.Fatalf("第%d次不该 over", i+1)
		}
	}
	if locked, _ := s.Locked(ctx, bFailLock, ipDims()); locked {
		t.Fatal("未超阈值不该锁")
	}
	if o := s.Fail(ctx, bFailLock, ipDims()); !o.Over {
		t.Fatal("第4次(>3)应 over")
	}
	if locked, _ := s.Locked(ctx, bFailLock, ipDims()); !locked {
		t.Fatal("应已锁")
	}
	s.ClearFail(ctx, bFailLock, ipDims())
	if o := s.Fail(ctx, bFailLock, ipDims()); o.Count != 1 {
		t.Fatalf("清后重新从1起,got %+v", o)
	}
}

// ── 冷却桶:gap 内重复被拦 ──
func TestService_Cooldown(t *testing.T) {
	ctx := context.Background()
	s := testService("user", newFake())
	if ok, _ := s.Cooldown(ctx, bCooldown, emailDims()); !ok {
		t.Fatal("首次应放行")
	}
	if ok, ra := s.Cooldown(ctx, bCooldown, emailDims()); ok || ra <= 0 {
		t.Fatalf("gap 内重复应被拦 + RetryAfter>0,got ok=%v ra=%d", ok, ra)
	}
}

// ── 命名空间隔离:两服务共用同一 Redis、同桶同维度,互不影响 ──
func TestService_NamespaceIsolation(t *testing.T) {
	ctx := context.Background()
	shared := newFake()
	user := testService("user", shared)
	cust := testService("customer", shared)

	for i := 0; i < 4; i++ {
		user.Fail(ctx, bFailLock, ipDims())
	}
	if locked, _ := user.Locked(ctx, bFailLock, ipDims()); !locked {
		t.Fatal("user 侧应已锁")
	}
	if locked, _ := cust.Locked(ctx, bFailLock, ipDims()); locked {
		t.Fatal("customer 侧不该被 user 的计数带锁(命名空间未隔离!)")
	}
	if o := cust.Fail(ctx, bFailLock, ipDims()); o.Count != 1 {
		t.Fatalf("customer 侧应从1起独立计数,got %+v", o)
	}
}

// ── Redis 抖动:一律 fail-open(放行)──
func TestService_FailOpen(t *testing.T) {
	ctx := context.Background()
	r := newFake()
	r.failMode = true
	s := testService("user", r)
	if o := s.Hit(ctx, bCount, ipDims()); o.Over {
		t.Fatal("Redis 挂时计数桶应放行")
	}
	if o := s.Fail(ctx, bFailLock, ipDims()); o.Over {
		t.Fatal("Redis 挂时失败锁桶应放行")
	}
	if locked, _ := s.Locked(ctx, bFailLock, ipDims()); locked {
		t.Fatal("Redis 挂时不该判为锁")
	}
	if ok, _ := s.Cooldown(ctx, bCooldown, emailDims()); !ok {
		t.Fatal("Redis 挂时冷却桶应放行")
	}
}

// TestEngine_Namespace_Exposed 锁住:引擎暴露自己的 namespace,供附属结构(如登录被锁列表索引)
// 用同一前缀拼 key、不脱钩。namespace 语义 = 哪个服务。
func TestEngine_Namespace_Exposed(t *testing.T) {
	_, r := newMiniredis(t)
	eng := NewEngine(r, "svc-x", StaticLimits(map[string]Limits{}))
	if got := eng.Namespace(); got != "svc-x" {
		t.Fatalf("Namespace() 应回显构造时的 ns,want svc-x got %q", got)
	}
}
