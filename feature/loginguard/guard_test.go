package loginguard

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"testing"
	"time"

	auroraFeature "github.com/shyandsy/aurora/feature"
	"github.com/shyandsy/aurora/feature/ratelimit"
)

// TestStaticPolicy_NilableForProvideAs 锁住:StaticPolicy 必须返回可空(指针)。
// di.ProvideAs 注册实例时 reflect.ValueOf(x).IsNil();结构体值会 panic → 装配即崩(build 不报、上线才炸)。
func TestStaticPolicy_NilableForProvideAs(t *testing.T) {
	v := reflect.ValueOf(StaticPolicy(DefaultPolicy()))
	if v.Kind() != reflect.Ptr {
		t.Fatalf("StaticPolicy 必须返回指针(可空),否则 di.ProvideAs 会 panic;got kind %v", v.Kind())
	}
	if v.IsNil() { // 不该 panic、不该为 nil
		t.Fatal("StaticPolicy 返回不该为 nil")
	}
}

// fakeRedis 内存版假 redis。内嵌 RedisService(契约真源),只实现 loginguard 用到的几个方法;
// 其余方法为 nil 接口,单测不会调到。failAll=true 时所有操作报错,用于验证 fail-open。
type fakeRedis struct {
	auroraFeature.RedisService
	m       map[string]string
	failAll bool
}

func newFake() *fakeRedis { return &fakeRedis{m: map[string]string{}} }

var errDown = errors.New("redis down")

func (f *fakeRedis) Get(_ context.Context, k string) (string, error) {
	if f.failAll {
		return "", errDown
	}
	return f.m[k], nil
}

// Eval 模拟底层 ratelimit 引擎的 incr 脚本(INCR + 首次 PEXPIRE):对 keys[0] 自增;TTL 不追踪
// (单测不验过期,真 TTL 语义见 guard_miniredis_test.go)。loginguard 自己已不再有计数脚本。
func (f *fakeRedis) Eval(_ context.Context, _ string, keys []string, _ ...interface{}) (interface{}, error) {
	if f.failAll {
		return nil, errDown
	}
	k := keys[0]
	n, _ := strconv.ParseInt(f.m[k], 10, 64)
	n++
	f.m[k] = strconv.FormatInt(n, 10)
	return n, nil
}
func (f *fakeRedis) Exists(_ context.Context, k string) (bool, error) {
	if f.failAll {
		return false, errDown
	}
	_, ok := f.m[k]
	return ok, nil
}
func (f *fakeRedis) Delete(_ context.Context, keys ...string) (int64, error) {
	if f.failAll {
		return 0, errDown
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
func (f *fakeRedis) SetNX(_ context.Context, k string, v interface{}, _ time.Duration) (bool, error) {
	if f.failAll {
		return false, errDown
	}
	if _, ok := f.m[k]; ok {
		return false, nil
	}
	f.m[k], _ = v.(string)
	return true, nil
}

const (
	ip   = "1.2.3.4"
	acct = "admin@example.com"
)

func hardLockPolicy() LoginPolicy { // deploy 管理台:账号硬锁
	p := DefaultPolicy()
	p.IPFailLimit = 2
	p.AcctFailLimit = 2
	p.AcctLockSeconds = 900
	return p
}
func countOnlyPolicy() LoginPolicy { // homeserver 公网:账号只计数
	p := hardLockPolicy()
	p.AcctLockSeconds = 0
	return p
}

// newG 用假 redis 建一个 ratelimit 引擎(namespace="test"),再包成 guard —— 走真实接入路径(引擎+策略适配)。
func newG(r auroraFeature.RedisService, p LoginPolicy) *guard {
	prov := StaticPolicy(p)
	return newGuard(ratelimit.NewEngine(r, "test", policyLimits{p: prov}), prov)
}

func TestDefaultPolicyValid(t *testing.T) {
	if !DefaultPolicy().Valid() {
		t.Fatal("DefaultPolicy 应自洽")
	}
	if (LoginPolicy{IPFailLimit: 3}).Valid() { // 开了 IP 锁却没窗口/时长
		t.Fatal("缺窗口/时长应判不自洽")
	}
	if DefaultPolicy().AcctLockSeconds != 0 {
		t.Fatal("安全默认:账号维度应 count-only(AcctLockSeconds=0)")
	}
}

func TestIPFailLock(t *testing.T) {
	ctx := context.Background()
	r := newFake()
	g := newG(r, hardLockPolicy()) // IPFailLimit=2
	for i := 0; i < 3; i++ {       // 3 > 2 → 锁
		g.RecordFailure(ctx, ip, "")
	}
	if d := g.PrecheckIP(ctx, ip); !d.Blocked || d.Reason != "ip_locked" {
		t.Fatalf("IP 应被锁: %+v", d)
	}
}

func TestAccountHardLock(t *testing.T) {
	ctx := context.Background()
	r := newFake()
	g := newG(r, hardLockPolicy()) // AcctFailLimit=2, AcctLockSeconds=900
	for i := 0; i < 3; i++ {
		g.RecordFailure(ctx, ip, acct)
	}
	if d := g.PrecheckAccount(ctx, acct); !d.Blocked || d.Reason != "acct_locked" {
		t.Fatalf("账号应被硬锁: %+v", d)
	}
}

// TestUnlock 锁后强制解锁:PrecheckIP 被锁时带 RetryAfter>0;Unlock 后 IP 与账号都不再锁。
func TestUnlock(t *testing.T) {
	ctx := context.Background()
	r := newFake()
	g := newG(r, hardLockPolicy()) // IPFailLimit=2, AcctFailLimit=2, AcctLockSeconds=900
	for i := 0; i < 3; i++ {       // 3 > 2 → 锁 IP + 账号
		g.RecordFailure(ctx, ip, acct)
	}
	if d := g.PrecheckIP(ctx, ip); !d.Blocked || d.RetryAfter <= 0 {
		t.Fatalf("锁后应 Blocked + RetryAfter>0,got %+v", d)
	}
	g.Unlock(ctx, ip, acct)
	if d := g.PrecheckIP(ctx, ip); d.Blocked {
		t.Fatalf("Unlock 后 IP 不该再锁: %+v", d)
	}
	if d := g.PrecheckAccount(ctx, acct); d.Blocked {
		t.Fatalf("Unlock 后账号不该再锁: %+v", d)
	}
}

func TestAccountCountOnly(t *testing.T) {
	ctx := context.Background()
	r := newFake()
	g := newG(r, countOnlyPolicy()) // AcctLockSeconds=0
	for i := 0; i < 5; i++ {
		g.RecordFailure(ctx, ip, acct)
	}
	// count-only:超阈值也绝不锁(Blocked 恒 false),但仍在计数(remaining 被打到 0)。
	d := g.PrecheckAccount(ctx, acct)
	if d.Blocked {
		t.Fatalf("count-only 模式账号绝不该被锁: %+v", d)
	}
	if d.AcctFailRemaining != 0 {
		t.Fatalf("count-only 模式仍应计数(超阈值后 remaining=0),got %d", d.AcctFailRemaining)
	}
}

func TestIPHourCap(t *testing.T) {
	ctx := context.Background()
	r := newFake()
	p := hardLockPolicy()
	p.IPPerHour = 2
	g := newG(r, p)
	g.RecordSuccess(ctx, ip, acct)
	g.RecordSuccess(ctx, ip, acct)
	if d := g.PrecheckIP(ctx, ip); !d.Blocked || d.Reason != "ip_hour_cap" {
		t.Fatalf("IP 小时上限应拦: %+v", d)
	}
}

func TestRecordSuccessClearsFailures(t *testing.T) {
	ctx := context.Background()
	r := newFake()
	g := newG(r, hardLockPolicy()) // IPFailLimit=2, AcctFailLimit=2
	g.RecordFailure(ctx, ip, acct)
	g.RecordSuccess(ctx, ip, acct)
	// 清干净 → remaining 恢复满额。
	if d := g.PrecheckIP(ctx, ip); d.IPFailRemaining != 2 {
		t.Fatalf("成功后应清 IP 失败计数(remaining 回满 2),got %d", d.IPFailRemaining)
	}
	if d := g.PrecheckAccount(ctx, acct); d.AcctFailRemaining != 2 {
		t.Fatalf("成功后应清账号失败计数(remaining 回满 2),got %d", d.AcctFailRemaining)
	}
}

// TestRecordPending 密码对但未完成(如待 2FA):只清**账号**失败计数,**保留 IP**,且不计每小时成功。
func TestRecordPending(t *testing.T) {
	ctx := context.Background()
	r := newFake()
	p := hardLockPolicy() // IPFailLimit=2, AcctFailLimit=2
	p.IPPerHour = 5       // 开每小时上限,便于断言 pending 不计成功
	g := newG(r, p)
	g.RecordFailure(ctx, ip, acct)
	g.RecordPending(ctx, ip, acct)
	di := g.PrecheckIP(ctx, ip)
	if di.IPFailRemaining != 1 {
		t.Fatalf("pending 必须**保留** IP 失败计数(仍记 1 次,remaining=1),got %d", di.IPFailRemaining)
	}
	if di.IPHourRemaining != 5 {
		t.Fatalf("pending 不该计入每小时成功数(remaining 仍满 5),got %d", di.IPHourRemaining)
	}
	if d := g.PrecheckAccount(ctx, acct); d.AcctFailRemaining != 2 {
		t.Fatalf("pending 应清账号失败计数(remaining 回满 2),got %d", d.AcctFailRemaining)
	}
}

// TestPendingCannotResetIPLock 回归:攻击者持有账号 A 的有效密码,反复触发 pending,
// 不得借此把 IP 失败计数归零、绕过 IP 锁去喷射爆破其它账号。pending 前的 IP 锁必须岿然不动。
func TestPendingCannotResetIPLock(t *testing.T) {
	ctx := context.Background()
	r := newFake()
	g := newG(r, hardLockPolicy()) // IPFailLimit=2
	// 攻击者从同一 IP 喷射爆破(对不同账号或空账号),触到 IP 锁:
	g.RecordFailure(ctx, ip, "victim1")
	g.RecordFailure(ctx, ip, "victim2")
	g.RecordFailure(ctx, ip, "victim3") // 3 > 2 → 锁 IP
	if d := g.PrecheckIP(ctx, ip); !d.Blocked {
		t.Fatal("前置条件:IP 应已被锁")
	}
	// 攻击者用自己账号 A 的正确密码反复触发 pending(不计每小时成功、可无限次):
	for i := 0; i < 5; i++ {
		g.RecordPending(ctx, ip, "attacker-A")
	}
	if d := g.PrecheckIP(ctx, ip); !d.Blocked || d.Reason != "ip_locked" {
		t.Fatalf("pending 绝不能重置 IP 锁,IP 仍须锁定: %+v", d)
	}
}

func TestPrecheckRemaining(t *testing.T) {
	ctx := context.Background()
	r := newFake()
	g := newG(r, hardLockPolicy()) // IPFailLimit=2, AcctFailLimit=2
	g.RecordFailure(ctx, ip, acct) // 各失败 1 次
	if d := g.PrecheckIP(ctx, ip); d.IPFailRemaining != 1 {
		t.Fatalf("IPFailRemaining want 1 got %d", d.IPFailRemaining)
	}
	if d := g.PrecheckAccount(ctx, acct); d.AcctFailRemaining != 1 {
		t.Fatalf("AcctFailRemaining want 1 got %d", d.AcctFailRemaining)
	}
}

// TestFailOpen redis 全挂 / redis 为 nil:预检一律放行、记录不 panic、绝不上锁(可用性优先)。
func TestFailOpen(t *testing.T) {
	ctx := context.Background()

	t.Run("nil redis", func(t *testing.T) {
		g := newGuard(nil, StaticPolicy(hardLockPolicy()))
		if g.PrecheckIP(ctx, ip).Blocked || g.PrecheckAccount(ctx, acct).Blocked {
			t.Fatal("nil redis 应 fail-open 放行")
		}
		g.RecordFailure(ctx, ip, acct) // 不 panic
		g.RecordSuccess(ctx, ip, acct)
		g.RecordPending(ctx, ip, acct)
	})

	t.Run("redis 全挂", func(t *testing.T) {
		r := &fakeRedis{m: map[string]string{}, failAll: true}
		g := newG(r, hardLockPolicy())
		for i := 0; i < 10; i++ {
			g.RecordFailure(ctx, ip, acct) // incr 报错 → 返回 0 → 永不到阈值 → 不上锁
		}
		if g.PrecheckIP(ctx, ip).Blocked || g.PrecheckAccount(ctx, acct).Blocked {
			t.Fatal("redis 挂时应 fail-open 放行(不因限流误锁全员)")
		}
	})
}
