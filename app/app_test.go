package app

import (
	"sync/atomic"
	"testing"

	"github.com/shyandsy/aurora/contracts"
)

// fakeFeature 记录 Close 被调次数。
type fakeFeature struct {
	name   string
	closed int32
}

func (f *fakeFeature) Name() string                 { return f.name }
func (f *fakeFeature) Setup(app contracts.App) error { return nil }
func (f *fakeFeature) Close() error {
	atomic.AddInt32(&f.closed, 1)
	return nil
}

// Shutdown 必须幂等:多次调用(含 Run 的 defer + 业务 main 显式调)下每个 feature 只 Close 一次。
func TestShutdownIdempotent(t *testing.T) {
	f1 := &fakeFeature{name: "f1"}
	f2 := &fakeFeature{name: "f2"}
	a := &app{features: []contracts.Features{f1, f2}}

	if err := a.Shutdown(); err != nil {
		t.Fatalf("Shutdown 第一次返错: %v", err)
	}
	if err := a.Shutdown(); err != nil {
		t.Fatalf("Shutdown 第二次返错: %v", err)
	}

	if got := atomic.LoadInt32(&f1.closed); got != 1 {
		t.Fatalf("f1.Close 应只调一次, got %d", got)
	}
	if got := atomic.LoadInt32(&f2.closed); got != 1 {
		t.Fatalf("f2.Close 应只调一次, got %d", got)
	}
}

// Close 按注册逆序调用(后注册的先关)。
func TestShutdownReverseOrder(t *testing.T) {
	var order []string
	mk := func(name string) *orderedFeature { return &orderedFeature{name: name, rec: &order} }
	a := &app{features: []contracts.Features{mk("a"), mk("b"), mk("c")}}

	_ = a.Shutdown()

	want := []string{"c", "b", "a"}
	if len(order) != len(want) {
		t.Fatalf("关闭数量不符: got %v", order)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("关闭顺序应逆序 %v, got %v", want, order)
		}
	}
}

type orderedFeature struct {
	name string
	rec  *[]string
}

func (f *orderedFeature) Name() string                 { return f.name }
func (f *orderedFeature) Setup(app contracts.App) error { return nil }
func (f *orderedFeature) Close() error {
	*f.rec = append(*f.rec, f.name)
	return nil
}
