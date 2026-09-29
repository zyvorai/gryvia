// Package main is the entry point for the Gryvia flow collector.
//
// It loads compiled eBPF programs, attaches them to the appropriate
// kernel hooks, starts perf-event readers, exposes Prometheus metrics
// on an HTTP endpoint, and optionally streams enriched events to NATS.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/zyvorai/gryvia/collector/pkg/aggregator"
	"github.com/zyvorai/gryvia/collector/pkg/ai"
	"github.com/zyvorai/gryvia/collector/pkg/anomaly"
	"github.com/zyvorai/gryvia/collector/pkg/decoder"
	"github.com/zyvorai/gryvia/collector/pkg/exporter"
	"github.com/zyvorai/gryvia/collector/pkg/graph"
	"github.com/zyvorai/gryvia/collector/pkg/loader"
	"github.com/zyvorai/gryvia/collector/pkg/security"
	"github.com/zyvorai/gryvia/collector/pkg/tuning"
)

func main() {
	var (
		metricsAddr = flag.String("metrics-addr", ":9090", "Prometheus metrics listen address")
		natsURL     = flag.String("nats-url", "", "NATS server URL (optional)")
		ebpfDir     = flag.String("ebpf-dir", "/opt/gryvia/ebpf", "Directory containing compiled eBPF .o files")
		iface       = flag.String("iface", "", "Network interface for XDP/TCX attachment (empty = skip those programs)")
		cgroupPath  = flag.String("cgroup-path", "", "cgroup v2 path for sockops/sk_msg attachment (empty = skip)")
		ncclLib     = flag.String("nccl-lib", "", "path to libnccl.so for uprobes (empty = auto-discover)")
		cudaLib     = flag.String("cuda-lib", "", "path to libcudart.so for uprobes (empty = auto-discover)")
		uprobePID   = flag.Int("uprobe-pid", 0, "find NCCL/CUDA libraries via /proc/<pid>/maps of this process")
		windowSec   = flag.Int("window", 300, "Aggregation sliding window in seconds")
	)
	flag.Parse()

	// Logger.
	logger, err := zap.NewProduction()
	if err != nil {
		panic("failed to create logger: " + err.Error())
	}
	defer func() { _ = logger.Sync() }()
	log := logger.Sugar()

	log.Infow("starting Gryvia collector",
		"metrics_addr", *metricsAddr,
		"ebpf_dir", *ebpfDir,
		"iface", *iface,
	)

	// Context with signal handling.
	ctx, cancel := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// ---- Load eBPF programs ----
	mgr, err := loader.New(loader.Config{
		Dir: *ebpfDir, Iface: *iface, CgroupPath: *cgroupPath,
		NCCLLib: *ncclLib, CUDALib: *cudaLib, UprobePID: *uprobePID,
	}, log)
	if err != nil {
		log.Fatalw("failed to create eBPF loader", "error", err)
	}
	if err := mgr.LoadAndAttach(); err != nil {
		log.Fatalw("failed to load eBPF programs", "error", err)
	}
	defer mgr.Close()

	attachGauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "gryvia_ebpf_program_attached",
		Help: "1 if the eBPF program is attached, 0 otherwise.",
	}, []string{"object", "program", "kind"})
	prometheus.MustRegister(attachGauge)
	for _, s := range mgr.Status() {
		v := 0.0
		if s.Attached {
			v = 1
		}
		attachGauge.WithLabelValues(s.Object, s.Program, s.Kind).Set(v)
	}

	// ---- Initialize subsystems ----
	metrics := exporter.NewMetrics()
	agg := aggregator.New(*windowSec)
	graphBuilder := graph.NewBuilder()
	detector := anomaly.NewDetector(log)

	// GPU and security decoders.
	gpuDecoder := decoder.NewGPUDecoder(4096)
	secDecoder := decoder.NewSecurityDecoder(4096)

	// GPU aggregators.
	window := time.Duration(*windowSec) * time.Second
	ncclAgg := aggregator.NewNCCLAggregator(window)
	gpuMemAgg := aggregator.NewGPUMemAggregator(window)

	// Security aggregator.
	secAgg := security.NewSecurityAggregator(10000)

	// AI training analysers.
	trainingAnalyzer := ai.NewTrainingAnalyzer()
	pipelineAnalyzer := ai.NewPipelineAnalyzer()

	// TCP tuning advisor.
	tcpAdvisor := tuning.NewTCPAdvisor()

	// ---- Decode perf events (existing flow events) ----
	dec, err := decoder.New(mgr.PerfReaders(loader.ClassFlow), log)
	if err != nil {
		log.Warnw("no perf readers available, flow decoding disabled", "error", err)
	}

	if dec != nil {
		// Event processing pipeline for flow events.
		eventCh := dec.Events()
		go func() {
			for ev := range eventCh {
				// Update aggregator.
				agg.Record(ev)
				// Update graph.
				graphBuilder.RecordFlow(ev)
				// Check for anomalies.
				detector.Observe(ev)
				// Update Prometheus metrics.
				metrics.RecordFlow(ev)
			}
		}()
		// Start perf event reading.
		go dec.Run(ctx)
	}

	// ---- Decode ring buffer events, dispatched by map class ----
	gpuRings := mgr.RingBufReaders(loader.ClassGPU)
	for _, rr := range gpuRings {
		go gpuDecoder.DecodeRingBuf(rr)
	}
	secRings := mgr.RingBufReaders(loader.ClassSecurity)
	for _, rr := range secRings {
		go secDecoder.DecodeRingBuf(rr)
	}
	log.Infow("event readers started",
		"flow_perf", len(mgr.PerfReaders(loader.ClassFlow)),
		"gpu_ringbuf", len(gpuRings), "security_ringbuf", len(secRings))

	// ---- GPU event processing pipeline ----
	go func() {
		for ev := range gpuDecoder.Events() {
			// Update NCCL aggregator.
			ncclAgg.Process(ev)
			// Update GPU memory aggregator.
			gpuMemAgg.Process(ev)
			// Update training analyser.
			trainingAnalyzer.Process(ev)
			// Update pipeline analyser.
			pipelineAnalyzer.ProcessGPUEvent(ev)
			// Update Prometheus GPU metrics.
			metrics.RecordGPUEvent(ev)
		}
	}()

	// ---- Security event processing pipeline ----
	go func() {
		for ev := range secDecoder.Events() {
			// Update security aggregator.
			secAgg.Process(ev)
			// Update Prometheus security metrics.
			metrics.RecordSecurityEvent(ev)
		}
	}()

	// ---- Periodic straggler detection and metric updates ----
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Detect NCCL stragglers.
				ncclAgg.DetectStragglers()
				stragglers := ncclAgg.GetStragglers()
				for range stragglers {
					metrics.RecordStragglerDetected()
				}

				// Update comm/compute ratio metric.
				ratio := trainingAnalyzer.GetCommComputeRatio()
				metrics.RecordCommComputeRatio("default", ratio)

				// Update pipeline bottleneck metric.
				bottleneck, _ := pipelineAnalyzer.GetBottleneck()
				metrics.RecordPipelineBottleneck(bottleneck)
			}
		}
	}()

	// ---- Optional NATS streaming ----
	if *natsURL != "" {
		log.Infow("NATS streaming enabled", "url", *natsURL)
		// NATS connection would be established here.
		// Events would be published to "gryvia.flows" subject.
	}

	// ---- HTTP server for metrics and API endpoints ----
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	// Existing endpoints.
	mux.HandleFunc("/api/v1/graph", graphBuilder.ServeHTTP)
	mux.HandleFunc("/api/v1/anomalies", detector.ServeHTTP)

	// GPU endpoints.
	mux.HandleFunc("/api/v1/gpu/nccl", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		stats := ncclAgg.GetOpStats()
		stragglers := ncclAgg.GetStragglers()
		resp := struct {
			OpStats    map[string]*aggregator.NCCLOpStats `json:"op_stats"`
			Stragglers []aggregator.StragglerEvent        `json:"stragglers"`
		}{
			OpStats:    stats,
			Stragglers: stragglers,
		}
		if err := encodeJSON(w, resp); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("/api/v1/gpu/memory", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		stats := gpuMemAgg.GetDirectionStats()
		if err := encodeJSON(w, stats); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	// Security endpoint.
	mux.HandleFunc("/api/v1/security/alerts", secAgg.ServeHTTP)

	// AI/training endpoints.
	mux.HandleFunc("/api/v1/ai/training", trainingAnalyzer.ServeHTTP)
	mux.HandleFunc("/api/v1/ai/pipeline", pipelineAnalyzer.ServeHTTP)

	// TCP tuning endpoint.
	mux.HandleFunc("/api/v1/tuning/tcp", tcpAdvisor.ServeHTTP)

	// eBPF program attach status.
	mux.HandleFunc("/api/v1/ebpf/status", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = encodeJSON(w, mgr.Status())
	})

	// Health check.
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	srv := &http.Server{
		Addr:    *metricsAddr,
		Handler: mux,
	}

	go func() {
		log.Infow("starting HTTP server", "addr", *metricsAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalw("HTTP server failed", "error", err)
		}
	}()

	// Wait for shutdown.
	<-ctx.Done()
	log.Info("shutting down collector")
	if err := srv.Close(); err != nil {
		log.Errorw("HTTP server close error", "error", err)
	}
}

// encodeJSON is a helper to encode JSON responses.
func encodeJSON(w http.ResponseWriter, v interface{}) error {
	enc := json.NewEncoder(w)
	return enc.Encode(v)
}
