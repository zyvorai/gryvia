// Package exporter provides Prometheus metrics derived from eBPF flow
// events.  All metrics are registered under the "tensorreaper_network_"
// namespace and labelled by source/destination service, protocol, and
// namespace.
package exporter

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/ssahani/TensorReaper/collector/pkg/decoder"
)

// Metrics holds all Prometheus metric handles.
type Metrics struct {
	flowBytes      *prometheus.CounterVec
	latency        *prometheus.HistogramVec
	connectionsAct *prometheus.GaugeVec
	drops          *prometheus.CounterVec
	dnsLatency     *prometheus.HistogramVec
}

// NewMetrics registers and returns all collector metrics.
func NewMetrics() *Metrics {
	labels := []string{"src_service", "dst_service", "protocol", "namespace"}

	return &Metrics{
		flowBytes: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "tensorreaper_network_flow_bytes_total",
			Help: "Total bytes transferred between services.",
		}, labels),

		latency: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "tensorreaper_network_latency_seconds",
			Help:    "Network latency between services in seconds.",
			Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0, 5.0},
		}, labels),

		connectionsAct: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "tensorreaper_network_connections_active",
			Help: "Number of currently active connections between services.",
		}, labels),

		drops: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "tensorreaper_network_drops_total",
			Help: "Total dropped packets/connections between services.",
		}, labels),

		dnsLatency: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "tensorreaper_network_dns_latency_seconds",
			Help:    "DNS resolution latency in seconds.",
			Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0},
		}, []string{"namespace"}),
	}
}

// RecordFlow updates Prometheus metrics from a decoded flow event.
func (m *Metrics) RecordFlow(ev decoder.FlowEvent) {
	src := ev.SrcService
	if src == "" {
		src = decoder.IPToString(ev.SrcIP)
	}
	dst := ev.DstService
	if dst == "" {
		dst = decoder.IPToString(ev.DstIP)
	}
	proto := protoLabel(ev.Protocol)
	ns := ev.Namespace

	labelValues := []string{src, dst, proto, ns}

	// Bytes transferred.
	if ev.Bytes > 0 {
		m.flowBytes.WithLabelValues(labelValues...).Add(float64(ev.Bytes))
	}

	// Latency.
	if ev.LatencyNs > 0 {
		m.latency.WithLabelValues(labelValues...).Observe(
			float64(ev.LatencyNs) / 1e9)
	}

	// Drops (verdict == 1).
	if ev.Verdict == 1 {
		m.drops.WithLabelValues(labelValues...).Inc()
	}

	// Active connections (increment on new, would decrement on close
	// with a more stateful approach -- for now, bump on every event).
	if ev.Verdict == 0 && ev.Protocol == 6 {
		m.connectionsAct.WithLabelValues(labelValues...).Inc()
	}
}

// RecordDNSLatency records a DNS resolution latency observation.
func (m *Metrics) RecordDNSLatency(ns string, latencyNs uint64) {
	if latencyNs > 0 {
		m.dnsLatency.WithLabelValues(ns).Observe(float64(latencyNs) / 1e9)
	}
}

func protoLabel(proto uint8) string {
	switch proto {
	case 6:
		return "tcp"
	case 17:
		return "udp"
	default:
		return "other"
	}
}
