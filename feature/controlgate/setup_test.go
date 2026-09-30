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
}

func newStubApp(runLevel string) *stubApp {
	return &stubApp{Container: di.NewContainer(), runLevel: runLevel}
}

func (*stubApp) AddFeature(contracts.Features)    {}
func (*stubApp) RegisterRoutes([]contracts.Route) {}
func (*stubApp) Run() error                       { return nil }
func (*stubApp) Shutdown() error                  { return nil }
func (*stubApp) Name() string                     { return "test" }
func (s *stubApp) RunLevel() string               { return s.runLevel }
func (s *stubApp) GetContainer() di.Container      { return s.Container }

// TestSetupPartialCoordsFailsStartup 坐标部分缺失(coordsComplete()==false)→ Setup fail-startup。
func TestSetupPartialCoordsFailsStartup(t *testing.T) {
	// 只给 RenewBaseURL,缺 ProjectUUID/DeploymentID/ProjectPubKey → 非全空、非齐全。
	cfg := Config{RenewBaseURL: "https://control.example", ServiceName: "svc"}
	if cfg.coordsAllEmpty() {
		t.Fatal("前置:部分坐标不应判定全空")
	}
	if cfg.coordsComplete() {
		t.Fatal("前置:部分坐标不应判定齐全")
	}
	f := NewControlgateFeature(cfg)
	if err := f.Setup(newStubApp("eng")); err == nil {
		t.Error("坐标部分缺失应 Setup fail-startup(暴露误配)")
	}
}

// TestSetupEmptyCoordsProductionFailsStartup 坐标全空 + production → Setup fail-startup(prd 全空=误配)。
func TestSetupEmptyCoordsProductionFailsStartup(t *testing.T) {
	f := NewControlgateFeature(Config{ServiceName: "svc"})
	if err := f.Setup(newStubApp("production")); err == nil {
		t.Error("production + 坐标全空应 fail-startup(拒绝静默裸奔)")
	}
}

// TestSetupEmptyCoordsEngDisabled 坐标全空 + eng → 门禁关闭(不 fail-startup),ProvideAs 一个 Disabled Gate。
func TestSetupEmptyCoordsEngDisabled(t *testing.T) {
	app := newStubApp("eng")
	f := NewControlgateFeature(Config{ServiceName: "svc"})
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
