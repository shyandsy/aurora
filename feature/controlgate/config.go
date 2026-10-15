package controlgate

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/shyandsy/aurora/config"
)

// defaultRenewEvery 续约间隔缺省值(可经 CONTROL_RENEW_EVERY 覆盖)。
const defaultRenewEvery = 60 * time.Second

// Config 是 controlgate feature 的内部配置载体,**不由消费方直接构造**——经 NewControlgateFeature 的
// 函数式 options(WithServiceName/WithMetricNamespace/WithRequireGate/WithRegisterer)设置,其余在 Setup 里
// 自动取:control 坐标(CONTROL_*)经 config.ResolveConfig 从 env 灌(env 名与现网冻结的一致,勿缩写/改名);
// ServiceName 默认 app.Name();DebugLogs 按 app.RunLevel() 推。消费方装配处因此只需(通常)传 metrics namespace:
//
//	app.AddFeature(controlgate.NewControlgateFeature(
//	    controlgate.WithMetricNamespace("myapp"),
//	))
type Config struct {
	// ServiceName 门禁名(admin/customer/schedule…),仅用于日志与 metrics 的 service label。
	// 非 env 字段;经 WithServiceName 覆盖,不设 → Setup 默认取 app.Name()。二者皆空 → Setup fail-startup
	//(不静默裸奔一个无名门禁)。
	ServiceName string

	// ── control 坐标(全空 = 门禁关闭,eng 开发便利;部分缺失 = 误配,Setup 直接 fail-startup)──
	RenewBaseURL  string        `env:"CONTROL_RENEW_BASE_URL,omitempty"` // 如 https://control-eng.deploy789.com
	ProjectUUID   string        `env:"CONTROL_PROJECT_UUID,omitempty"`
	DeploymentID  string        `env:"CONTROL_DEPLOYMENT_ID,omitempty"`
	ProjectPubKey string        `env:"CONTROL_PROJECT_PUBKEY,omitempty"` // base64 ed25519 项目公钥(只验签,非私钥)
	RenewEvery    time.Duration `env:"CONTROL_RENEW_EVERY,omitempty"`    // 续约间隔(可选,默认 60s)
	StateDir      string        `env:"CONTROL_STATE_DIR,omitempty"`      // 持久化目录(可选,跨重启保留剩余授权)

	// StartupGrace 启动期时间门窗口(可选,默认 2min):never-seen / 重启 / 删 state 时,给这么短的窗口
	// 一边服务一边轮询认证时间。首次握手秒级,别调到 1min 以下(给慢冷启动 / control 首拉留余量)。
	// ⚠️ 配了 NTS 源时,此窗口不再是"无条件放行":never-seen 也须先拿 fresh 认证时间才服务(见 gate.faultRate)。
	StartupGrace time.Duration `env:"CONTROL_STARTUP_GRACE,omitempty"`

	// ExemptPaths 在默认基础设施端点(/health /ready /healthz /readyz /livez /metrics)之上**追加**
	// 放行的路径(降级绝不注入 500)。不同服务探针路径不同,写死会在降级时 500 掉探针 → crashloop。
	// env CONTROL_EXEMPT_PATHS 逗号分隔(如 "/ping,/api/v1/health");默认恒 exempt,本项只增不减。
	ExemptPaths []string `env:"CONTROL_EXEMPT_PATHS,omitempty"`

	// MetricNamespace prometheus 指标 namespace(项目前缀,如 "myapp")。空 → 不启用观测(中立库不自造前缀)。
	// 非 env 字段;经 WithMetricNamespace 设。
	MetricNamespace string

	// DebugLogs 是否输出门禁调试/状态日志。非 env 字段;**由 Setup 按 app.RunLevel()==eng 自动设**,
	// 消费方不必管(prd 自动收敛;prd build 时再叠 garble/DCE 收特征)。
	DebugLogs bool

	// RequireGate 逃生阀:置 true 则**任何 RunLevel** 下坐标全空都 fail-startup(不允许 disabled 放行)。
	// 默认 false:仅 prd(production)坐标全空才 fail-startup,local/eng/stage 全空 → disabled(开发便利)。
	// 非 env 字段;经 WithRequireGate 设。
	RequireGate bool

	// Registerer 观测用的 prometheus registerer(可空 → prometheus.DefaultRegisterer)。非 env 字段;
	// 经 WithRegisterer 设(测试可注入独立 registry)。
	Registerer prometheus.Registerer
}

// Key 供 aurora config.ResolveConfig 识别(约定接口)。
func (c *Config) Key() string { return "controlgate" }

// renewEveryOrDefault 取续约间隔,非正 → 缺省 60s。
func (c *Config) renewEveryOrDefault() time.Duration {
	if c.RenewEvery > 0 {
		return c.RenewEvery
	}
	return defaultRenewEvery
}

// startupGraceOrDefault 取启动期时间门窗口,非正 → 缺省 defaultStartupGrace(2min)。
func (c *Config) startupGraceOrDefault() time.Duration {
	if c.StartupGrace > 0 {
		return c.StartupGrace
	}
	return defaultStartupGrace
}

// coordsAllEmpty 坐标是否全空(→ 门禁关闭)。
func (c *Config) coordsAllEmpty() bool {
	return c.RenewBaseURL == "" && c.ProjectUUID == "" && c.DeploymentID == "" && c.ProjectPubKey == ""
}

// coordsComplete 坐标是否齐全。
func (c *Config) coordsComplete() bool {
	return c.RenewBaseURL != "" && c.ProjectUUID != "" && c.DeploymentID != "" && c.ProjectPubKey != ""
}

// failStartupOnEmptyCoords 坐标全空时是否应 fail-startup(而非 disabled 放行):
// RequireGate=true(逃生阀,任何 level)或 RunLevel=production(prd 全空 = 误配)。
func (c *Config) failStartupOnEmptyCoords(runLevel string) bool {
	return c.RequireGate || runLevel == config.RunLevelProduction
}
