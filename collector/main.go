// Package main is the entry point for the TensorReaper flow collector.
//
// It loads compiled eBPF programs, attaches them to the appropriate
// kernel hooks, starts perf-event readers, exposes Prometheus metrics
// on an HTTP endpoint, and optionally streams enriched events to NATS.
package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/ssahani/TensorReaper/collector/pkg/aggregator"
	"github.com/ssahani/TensorReaper/collector/pkg/anomaly"
	"github.com/ssahani/TensorReaper/collector/pkg/decoder"
	"github.com/ssahani/TensorReaper/collector/pkg/exporter"
	"github.com/ssahani/TensorReaper/collector/pkg/graph"
	"github.com/ssahani/TensorReaper/collector/pkg/loader"
)

func main() {
	var (
		metricsAddr = flag.String("metrics-addr", ":9090", "Prometheus metrics listen address")
		natsURL     = flag.String("nats-url", "", "NATS server URL (optional)")
		ebpfDir     = flag.String("ebpf-dir", "/opt/tensorreaper/ebpf", "Directory containing compiled eBPF .o files")
		iface       = flag.String("iface", "eth0", "Network interface for XDP attachment")
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

	log.Infow("starting TensorReaper collector",
		"metrics_addr", *metricsAddr,
		"ebpf_dir", *ebpfDir,
		"iface", *iface,
	)

	// Context with signal handling.
	ctx, cancel := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// ---- Load eBPF programs ----
	mgr, err := loader.New(*ebpfDir, *iface, log)
	if err != nil {
		log.Fatalw("failed to create eBPF loader", "error", err)
	}
	if err := mgr.LoadAndAttach(); err != nil {
		log.Fatalw("failed to load eBPF programs", "error", err)
	}
	defer mgr.Close()

	// ---- Initialize subsystems ----
	metrics := exporter.NewMetrics()
	agg := aggregator.New(*windowSec)
	graphBuilder := graph.NewBuilder()
	detector := anomaly.NewDetector(log)

	// ---- Decode perf events ----
	dec, err := decoder.New(mgr.PerfReaders(), log)
	if err != nil {
		log.Fatalw("failed to create decoder", "error", err)
	}

	// Event processing pipeline.
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

	// ---- Optional NATS streaming ----
	if *natsURL != "" {
		log.Infow("NATS streaming enabled", "url", *natsURL)
		// NATS connection would be established here.
		// Events would be published to "tensorreaper.flows" subject.
	}

	// ---- HTTP server for metrics and graph API ----
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/api/v1/graph", graphBuilder.ServeHTTP)
	mux.HandleFunc("/api/v1/anomalies", detector.ServeHTTP)
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
