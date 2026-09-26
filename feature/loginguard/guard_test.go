package loginguard

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"
)

// fakeRedis 内存版 redisOps。failAll=true 时所有操作报错,用于验证 fail-open。
type fakeRedis struct {
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
func (f *fakeRedis) Incr(_ context.Context, k string) (int64, error) {
	if f.failAll {
		return 0, errDown
	}
	n, _ := strconv.ParseInt(f.m[k], 10, 64)
	n++
	f.m[k] = strconv.FormatInt(n, 10)
	return n, nil
}
func (f *fakeRedis) Expire(_ context.Context, _ string, _ time.Duration) error {
	if f.failAll {
		return errDown
	}
	return nil
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

func newG(r redisOps, p LoginPolicy) *guard { return newGuard(r, StaticPolicy(p)) }

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

func TestAccountCountOnly(t *testing.T) {
	ctx := context.Background()
	r := newFake()
	g := newG(r, countOnlyPolicy()) // AcctLockSeconds=0
	for i := 0; i < 5; i++ {
		g.RecordFailure(ctx, ip, acct)
	}
	if d := g.PrecheckAccount(ctx, acct); d.Blocked {
		t.Fatalf("count-only 模式账号绝不该被锁: %+v", d)
	}
	if _, locked := r.m[acctLockKey(acct)]; locked {
		t.Fatal("count-only 模式不该写账号锁 key")
	}
	if _, counted := r.m[acctFailKey(acct)]; !counted {
		t.Fatal("count-only 模式仍应计数(供提示/监控)")
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
	g := newG(r, hardLockPolicy())
	g.RecordFailure(ctx, ip, acct)
	g.RecordSuccess(ctx, ip, acct)
	if _, ok := r.m[ipFailKey(ip)]; ok {
		t.Fatal("成功后应清 IP 失败计数")
	}
	if _, ok := r.m[acctFailKey(acct)]; ok {
		t.Fatal("成功后应清账号失败计数")
	}
}

// TestRecordPending 密码对但未完成(如待 2FA):清失败,但不计每小时成功(登录未真正完成)。
func TestRecordPending(t *testing.T) {
	ctx := context.Background()
	r := newFake()
	g := newG(r, hardLockPolicy())
	g.RecordFailure(ctx, ip, acct)
	g.RecordPending(ctx, ip, acct)
	if _, ok := r.m[ipFailKey(ip)]; ok {
		t.Fatal("pending 应清 IP 失败计数")
	}
	if _, ok := r.m[acctFailKey(acct)]; ok {
		t.Fatal("pending 应清账号失败计数")
	}
	if _, ok := r.m[ipHourKey(ip)]; ok {
		t.Fatal("pending 不该计入每小时成功数(登录未完成)")
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
