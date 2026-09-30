package controlgate

// ServingState 是门禁对外暴露的服务状态(供健康/状态页读授权状态)。
// 数值与 metrics 的 serving_state gauge 一致:0=serving 1=shedding 2=tripped 3=isolated。
type ServingState int

const (
	// Serving 满速服务(授权有效,或启动宽限窗口内)。
	Serving ServingState = iota
	// Shedding 部分丢弃(过期宽限期内,按 control 下发的 errorRate 随机注入 500)。
	Shedding
	// Tripped 全停(超启动宽限仍无授权 / pending / 过期不允许续用 / 超宽限上限)。
	Tripped
	// Isolated 隔离(已吊销,或有可试时间源却全拿不到 → 判被气隙攻击)。
	Isolated
	// Disabled 门禁关闭(eng 未接入 control,坐标全空)。仅经 Gate.ServingState() 暴露给状态页/告警,
	// **不进 metrics gauge**(disabledGate 不跑观测循环,serving_state 只出 0-3)——让「闸关了」可见,不伪装健康。
	Disabled
)

// String 返回状态名(与 Grafana value-mapping 口径一致)。
func (s ServingState) String() string {
	switch s {
	case Serving:
		return "serving"
	case Shedding:
		return "shedding"
	case Tripped:
		return "tripped"
	case Isolated:
		return "isolated"
	case Disabled:
		return "disabled"
	default:
		return "unknown"
	}
}
