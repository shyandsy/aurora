package controlgate

import (
	"testing"

	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/di"
)

// stubApp 是最小 contracts.App:内嵌真 di.Container(ProvideAs/Resolve 真跑),其余方法空实现。
// 只用于测 Setup 的**早返回分支**(坐标全空/部分缺失),不进完整路径(那需真 *gin.Engine)。
type stubApp struct {
	di.Container
	runLevel string
	name     string
}

func newStubApp(runLevel string) *stubApp {
	return &stubApp{Container: di.NewContainer(), runLevel: runLevel, name: "test"}
}

func (*stubApp) AddFeature(contracts.Features)    {}
func (*stubApp) RegisterRoutes([]contracts.Route) {}
func (*stubApp) Run() error                       { return nil }
func (*stubApp) Shutdown() error                  { return nil }
func (s *stubApp) Name() string                   { return s.name }
func (s *stubApp) RunLevel() string               { return s.runLevel }
func (s *stubApp) GetContainer() di.Container      { return s.Container }

// setEmptyCoords 把 4 个 CONTROL_* 坐标显式置空(hermetic:挡住开发机 / CI 里残留的同名 env 干扰,
// 保证「坐标全空」判定稳定)。坐标现在走 env(Setup 内部 ResolveConfig),故测试经 env 注入。
func setEmptyCoords(t *testing.T) {
	t.Helper()
	t.Setenv("CONTROL_RENEW_BASE_URL", "")
	t.Setenv("CONTROL_PROJECT_UUID", "")
	t.Setenv("CONTROL_DEPLOYMENT_ID", "")
	t.Setenv("CONTROL_PROJECT_PUBKEY", "")
}

// TestSetupPartialCoordsFailsStartup 坐标部分缺失(只配 RENEW_BASE_URL)→ Setup fail-startup。
func TestSetupPartialCoordsFailsStartup(t *testing.T) {
	setEmptyCoords(t)
	t.Setenv("CONTROL_RENEW_BASE_URL", "https://control.example") // 只给这一个 → 非全空、非齐全
	f := NewControlgateFeature(WithServiceName("svc"))
	if err := f.Setup(newStubApp("eng")); err == nil {
		t.Error("坐标部分缺失应 Setup fail-startup(暴露误配)")
	}
}

// TestSetupEmptyCoordsProductionFailsStartup 坐标全空 + production → Setup fail-startup(prd 全空=误配)。
func TestSetupEmptyCoordsProductionFailsStartup(t *testing.T) {
	setEmptyCoords(t)
	f := NewControlgateFeature(WithServiceName("svc"))
	if err := f.Setup(newStubApp("production")); err == nil {
		t.Error("production + 坐标全空应 fail-startup(拒绝静默裸奔)")
	}
}

// TestSetupEmptyCoordsEngDisabled 坐标全空 + eng → 门禁关闭(不 fail-startup),ProvideAs 一个 Disabled Gate。
func TestSetupEmptyCoordsEngDisabled(t *testing.T) {
	setEmptyCoords(t)
	app := newStubApp("eng")
	f := NewControlgateFeature(WithServiceName("svc"))
	if err := f.Setup(app); err != nil {
		t.Fatalf("eng + 坐标全空应 disabled 放行(不报错), got %v", err)
	}
	// 验证注入的 Gate 是 disabled 态(ServingState=Disabled),而非漏 provide。
	var holder struct {
		Gate Gate `inject:""`
	}
	if err := app.Resolve(&holder); err != nil {
		t.Fatalf("Setup 应 ProvideAs 一个 Gate(inject 解析失败): %v", err)
	}
	if holder.Gate == nil {
		t.Fatal("Gate 未被 provide")
	}
	if holder.Gate.ServingState() != Disabled {
		t.Errorf("门禁关闭时 Gate.ServingState 应为 Disabled, got %s", holder.Gate.ServingState())
	}
}

// TestSetupEmptyServiceNameFailsStartup 未传 WithServiceName 且 app.Name() 空 → Setup fail-startup
//(拒绝无名门禁;正常 aurora app 的 SERVICE_NAME 必填 + envDefault,到不了这,是防非法 App 实现)。
func TestSetupEmptyServiceNameFailsStartup(t *testing.T) {
	setEmptyCoords(t)
	app := newStubApp("eng")
	app.name = "" // 模拟 App 实现没给名
	f := NewControlgateFeature()
	if err := f.Setup(app); err == nil {
		t.Error("ServiceName 无法确定应 fail-startup")
	}
}
