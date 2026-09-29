package controlgate

import (
	"github.com/prometheus/client_golang/prometheus"
)

// ── 观测指标(中性命名,伪装成 HTTP 过载/服务健康:甲方看 /metrics 看不出授权语义,
// 可读性只落在我们 Grafana 的 value-mapping 里)────────────────────────────────
//
//	<ns>_http_shed_ratio           0-100 当前丢弃比例(= faultRate)
//	<ns>_http_serving_state        0=serving 1=shedding 2=tripped 3=isolated
//	<ns>_http_serving_ttl_seconds  距彻底停服剩余秒数
//
// namespace 由消费方配(Config.MetricNamespace):aurora 是中立框架,不硬编任何项目名——
// homeserver 传 "homeserver" 保持其既有 Grafana 指标契约,其它项目传自己的中立前缀。
// subsystem/name/label 保持不变(即是"伪装成过载指标"的中性契约)。
type gateMetrics struct {
	shedRatio  *prometheus.GaugeVec
	serving    *prometheus.GaugeVec
	servingTTL *prometheus.GaugeVec
}

// newGateMetrics 在给定 registerer 上注册三个 gauge(namespace 可配)。
// registerer 为 nil 时用 prometheus.DefaultRegisterer。重复注册(同名已存在)会复用已注册者,
// 不 panic —— 便于同进程多 gate / 测试重复构造。
func newGateMetrics(namespace string, reg prometheus.Registerer) *gateMetrics {
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}
	mk := func(name, help string) *prometheus.GaugeVec {
		gv := prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace, Subsystem: "http", Name: name, Help: help,
		}, []string{"service"})
		if err := reg.Register(gv); err != nil {
			if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
				if existing, ok := are.ExistingCollector.(*prometheus.GaugeVec); ok {
					return existing
				}
			}
			// 其它注册错误:退回一个未注册的 gauge(仍可 Set,只是不被抓取),绝不让门禁因观测崩。
		}
		return gv
	}
	return &gateMetrics{
		shedRatio:  mk("shed_ratio", "HTTP request shed ratio 0-100 (overload protection)"),
		serving:    mk("serving_state", "serving state: 0=serving 1=shedding 2=tripped 3=isolated"),
		servingTTL: mk("serving_ttl_seconds", "seconds until serving fully trips"),
	}
}

func (m *gateMetrics) set(service string, shed, state, ttl float64) {
	m.shedRatio.WithLabelValues(service).Set(shed)
	m.serving.WithLabelValues(service).Set(state)
	m.servingTTL.WithLabelValues(service).Set(ttl)
}
