// Package exporter provides Prometheus metrics derived from eBPF flow
// events, GPU/NCCL events, security events, and AI training analysis.
package exporter

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/zyvorai/gryvia/collector/pkg/decoder"
)

// Metrics holds all Prometheus metric handles.
type Metrics struct {
	// Network metrics.
	flowBytes      *prometheus.CounterVec
	latency        *prometheus.HistogramVec
	connectionsAct *prometheus.GaugeVec
	drops          *prometheus.CounterVec
	dnsLatency     *prometheus.HistogramVec

	// GPU / NCCL metrics.
	ncclOpDuration        *prometheus.HistogramVec
	ncclBytesTotal        *prometheus.CounterVec
	ncclStragglerTotal    prometheus.Counter
	gpuMemcpyBytes        *prometheus.CounterVec
	gpuMemcpyDuration     *prometheus.HistogramVec
	rdmaSendBytesTotal    prometheus.Counter
	rdmaRecvBytesTotal    prometheus.Counter
	rdmaCompletionLatency prometheus.Histogram

	// Security metrics.
	securityAlertsTotal      *prometheus.CounterVec
	securityEscapeAttempts   prometheus.Counter
	securityMiningDetections prometheus.Counter

	// AI training metrics.
	trainingCommComputeRatio *prometheus.GaugeVec
	trainingStragglerEvents  prometheus.Counter
	pipelineBottleneck       *prometheus.GaugeVec

	// Performance / TCP metrics.
	tcpCwndHistogram prometheus.Histogram
	tcpRTTHistogram  prometheus.Histogram
	networkCostBytes *prometheus.CounterVec
}

// NewMetrics registers and returns all collector metrics.
func NewMetrics() *Metrics {
	labels := []string{"src_service", "dst_service", "protocol", "namespace"}

	return &Metrics{
		// ---- Network metrics ----
		flowBytes: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "gryvia_network_flow_bytes_total",
			Help: "Total bytes transferred between services.",
		}, labels),

		latency: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "gryvia_network_latency_seconds",
			Help:    "Network latency between services in seconds.",
			Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0, 5.0},
		}, labels),

		connectionsAct: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gryvia_network_connections_active",
			Help: "Number of currently active connections between services.",
		}, labels),

		drops: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "gryvia_network_drops_total",
			Help: "Total dropped packets/connections between services.",
		}, labels),

		dnsLatency: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "gryvia_network_dns_latency_seconds",
			Help:    "DNS resolution latency in seconds.",
			Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0},
		}, []string{"namespace"}),

		// ---- GPU / NCCL metrics ----
		ncclOpDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "gryvia_nccl_operation_duration_seconds",
			Help:    "Duration of NCCL collective operations in seconds.",
			Buckets: []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0},
		}, []string{"op_type", "rank"}),

		ncclBytesTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "gryvia_nccl_bytes_total",
			Help: "Total bytes transferred by NCCL operations.",
		}, []string{"op_type"}),

		ncclStragglerTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "gryvia_nccl_stragglers_detected_total",
			Help: "Total number of NCCL straggler events detected.",
		}),

		gpuMemcpyBytes: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "gryvia_gpu_memcpy_bytes_total",
			Help: "Total bytes transferred by GPU memcpy operations.",
		}, []string{"direction"}),

		gpuMemcpyDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "gryvia_gpu_memcpy_duration_seconds",
			Help:    "Duration of GPU memcpy operations in seconds.",
			Buckets: []float64{0.00001, 0.0001, 0.001, 0.01, 0.1, 1.0},
		}, []string{"direction"}),

		rdmaSendBytesTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "gryvia_rdma_send_bytes_total",
			Help: "Total bytes sent via RDMA.",
		}),

		rdmaRecvBytesTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "gryvia_rdma_recv_bytes_total",
			Help: "Total bytes received via RDMA.",
		}),

		rdmaCompletionLatency: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "gryvia_rdma_completion_latency_seconds",
			Help:    "RDMA completion latency in seconds.",
			Buckets: []float64{0.000001, 0.00001, 0.0001, 0.001, 0.01},
		}),

		// ---- Security metrics ----
		securityAlertsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "gryvia_security_alerts_total",
			Help: "Total security alerts by event type and severity.",
		}, []string{"event_type", "severity"}),

		securityEscapeAttempts: promauto.NewCounter(prometheus.CounterOpts{
			Name: "gryvia_security_escape_attempts_total",
			Help: "Total container escape attempts detected.",
		}),

		securityMiningDetections: promauto.NewCounter(prometheus.CounterOpts{
			Name: "gryvia_security_mining_detections_total",
			Help: "Total crypto mining activity detections.",
		}),

		// ---- AI training metrics ----
		trainingCommComputeRatio: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gryvia_training_comm_compute_ratio",
			Help: "Ratio of communication time to compute time for training jobs.",
		}, []string{"job"}),

		trainingStragglerEvents: promauto.NewCounter(prometheus.CounterOpts{
			Name: "gryvia_training_straggler_events_total",
			Help: "Total straggler events detected during training.",
		}),

		pipelineBottleneck: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gryvia_pipeline_bottleneck",
			Help: "Current pipeline bottleneck indicator (1.0 = bottleneck).",
		}, []string{"phase"}),

		// ---- Performance / TCP metrics ----
		tcpCwndHistogram: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "gryvia_tcp_cwnd_histogram",
			Help:    "Distribution of TCP congestion window sizes.",
			Buckets: []float64{1, 5, 10, 20, 50, 100, 200, 500, 1000},
		}),

		tcpRTTHistogram: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "gryvia_tcp_rtt_seconds",
			Help:    "Distribution of TCP round-trip times in seconds.",
			Buckets: []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05, 0.1, 0.5},
		}),

		networkCostBytes: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "gryvia_network_cost_bytes_total",
			Help: "Total network bytes by zone type and namespace for cost analysis.",
		}, []string{"zone_type", "namespace"}),
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

// RecordGPUEvent updates GPU/NCCL Prometheus metrics from a decoded GPU event.
func (m *Metrics) RecordGPUEvent(ev decoder.GPUEvent) {
	switch ev.EventType {
	case decoder.GPUEvtNCCLOp:
		opType := decoder.NCCLOpName(ev.NCCLOp)
		rank := fmt.Sprintf("%d", ev.SrcRank)
		m.ncclOpDuration.WithLabelValues(opType, rank).Observe(float64(ev.LatencyNs) / 1e9)
		m.ncclBytesTotal.WithLabelValues(opType).Add(float64(ev.Bytes))

	case decoder.GPUEvtMemTransfer:
		dir := decoder.MemDirectionName(ev.Direction)
		m.gpuMemcpyBytes.WithLabelValues(dir).Add(float64(ev.Bytes))
		m.gpuMemcpyDuration.WithLabelValues(dir).Observe(float64(ev.LatencyNs) / 1e9)

	case decoder.GPUEvtRDMASend:
		m.rdmaSendBytesTotal.Add(float64(ev.Bytes))
		m.rdmaCompletionLatency.Observe(float64(ev.LatencyNs) / 1e9)

	case decoder.GPUEvtRDMARecv:
		m.rdmaRecvBytesTotal.Add(float64(ev.Bytes))
		m.rdmaCompletionLatency.Observe(float64(ev.LatencyNs) / 1e9)
	}
}

// RecordSecurityEvent updates security Prometheus metrics.
func (m *Metrics) RecordSecurityEvent(ev decoder.SecurityEvent) {
	eventType := decoder.SecurityEventTypeName(ev.EventType)
	severity := decoder.SeverityName(ev.Severity)
	m.securityAlertsTotal.WithLabelValues(eventType, severity).Inc()

	switch ev.EventType {
	case decoder.SecEvtContainerEscape:
		m.securityEscapeAttempts.Inc()
	case decoder.SecEvtCryptoMining:
		m.securityMiningDetections.Inc()
	}
}

// RecordStragglerDetected increments the straggler counter.
func (m *Metrics) RecordStragglerDetected() {
	m.ncclStragglerTotal.Inc()
	m.trainingStragglerEvents.Inc()
}

// RecordCommComputeRatio sets the communication/compute ratio gauge.
func (m *Metrics) RecordCommComputeRatio(job string, ratio float64) {
	m.trainingCommComputeRatio.WithLabelValues(job).Set(ratio)
}

// RecordPipelineBottleneck sets the bottleneck gauge for a given phase.
func (m *Metrics) RecordPipelineBottleneck(bottleneck string) {
	// Reset all phases.
	for _, phase := range []string{"data_loading", "preprocessing", "compute", "communication"} {
		if phase == bottleneck {
			m.pipelineBottleneck.WithLabelValues(phase).Set(1.0)
		} else {
			m.pipelineBottleneck.WithLabelValues(phase).Set(0.0)
		}
	}
}

// RecordTCPStats records TCP connection statistics for Prometheus.
func (m *Metrics) RecordTCPStats(cwnd float64, rttUs float64) {
	m.tcpCwndHistogram.Observe(cwnd)
	m.tcpRTTHistogram.Observe(rttUs / 1e6) // convert us to seconds
}

// RecordNetworkCost records network cost bytes by zone type and namespace.
func (m *Metrics) RecordNetworkCost(zoneType, namespace string, bytes float64) {
	m.networkCostBytes.WithLabelValues(zoneType, namespace).Add(bytes)
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
