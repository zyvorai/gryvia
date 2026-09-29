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
	"strings"
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
	"github.com/zyvorai/gryvia/collector/pkg/fabric"
	"github.com/zyvorai/gryvia/collector/pkg/flight"
	"github.com/zyvorai/gryvia/collector/pkg/graph"
	"github.com/zyvorai/gryvia/collector/pkg/kube"
	"github.com/zyvorai/gryvia/collector/pkg/loader"
	"github.com/zyvorai/gryvia/collector/pkg/security"
	"github.com/zyvorai/gryvia/collector/pkg/tuning"
)

func main() {
	var (
		metricsAddr     = flag.String("metrics-addr", ":9090", "Prometheus metrics listen address")
		natsURL         = flag.String("nats-url", "", "NATS server URL (optional)")
		ebpfDir         = flag.String("ebpf-dir", "/opt/gryvia/ebpf", "Directory containing compiled eBPF .o files")
		iface           = flag.String("iface", "", "Network interface for XDP/TCX attachment (empty = skip those programs)")
		cgroupPath      = flag.String("cgroup-path", "", "cgroup v2 path for sockops/sk_msg attachment (empty = skip)")
		ncclLib         = flag.String("nccl-lib", "", "path to libnccl.so for uprobes (empty = auto-discover)")
		cudaLib         = flag.String("cuda-lib", "", "path to libcudart.so for uprobes (empty = auto-discover)")
		cufileLib       = flag.String("cufile-lib", "", "path to libcufile.so for GPUDirect Storage uprobes (empty = auto-discover)")
		ucxLib          = flag.String("ucx-lib", "", "path to libucp.so (UCX) for ucx_gloo uprobes (empty = auto-discover)")
		uprobePID       = flag.Int("uprobe-pid", 0, "find NCCL/CUDA/cuFile/UCX libraries via /proc/<pid>/maps of this process")
		inferPorts      = flag.String("infer-ports", "8000,8001", "local TCP ports of inference servers for infer_latency (vLLM 8000, Triton HTTP 8001; at most 8, empty disables it)")
		flightTokenFile = flag.String("flight-token-file", "", "file containing the Flight Recorder API token; empty disables the endpoint")
		quotaPace       = flag.Bool("quota-pace", false, "attach quota_pace (the only program that changes sockets: caps SO_MAX_PACING_RATE of cgroups that hold a lease). Off by default; needs -cgroup-path. Without -quota-pace-sync nothing grants leases and pacing stays inert")
		quotaPaceSync   = flag.Bool("quota-pace-sync", false, "MUTATING, off by default: every 30 s grant a pace lease to the cgroups of pods on this node whose namespace is listed by a GryviaQuota with spec.network.maxEgressMbps (needs -quota-pace, -cgroup-path, running in a cluster and NODE_NAME). Fails open: an API error changes nothing and leases expire after 2 minutes")
		quotaPaceDry    = flag.Bool("quota-pace-dry-run", false, "with -quota-pace-sync: log what would be granted or revoked and write nothing to the pace map")
		publishNodeFab  = flag.Bool("publish-node-fabric", false, "every 30 s write this node's fabric health (worst scoreDelta + top reasons, ttl 5 min) to the cluster-scoped GryviaNodeFabric named after NODE_NAME, for the ai-operator's opt-in fabric-aware scheduling; creates only that one object; needs a cluster, NODE_NAME and RBAC (chart value ebpf.publishNodeFabric)")
		publishFabric   = flag.Bool("publish-fabric-status", false, "every 30 s patch the status of an existing GryviaFabricSignal (spec.jobRef = job) with the folded fabric signals; needs a cluster (KUBERNETES_SERVICE_HOST) and RBAC (chart value ebpf.publishFabricStatus)")
		tlsCertFile     = flag.String("tls-cert-file", "", "serve the HTTP listener over TLS (>= 1.2) with this certificate; re-read when the file changes (rotation). Needs -tls-key-file")
		tlsKeyFile      = flag.String("tls-key-file", "", "private key for -tls-cert-file")
		tlsClientCA     = flag.String("tls-client-ca-file", "", "enable mTLS: require and verify client certificates against this CA bundle (needs TLS); a verified client certificate authenticates every endpoint")
		apiTokenFile    = flag.String("api-token-file", "", "file with a shared token (>= 32 chars): every endpoint except /healthz, /readyz and the flight endpoint then needs an HMAC request signature (X-Gryvia-Time / X-Gryvia-Signature)")
		metricsToken    = flag.String("metrics-token-file", "", "file with a bearer token (>= 32 chars) that Prometheus may present on /metrics only")
		insecureListen  = flag.Bool("insecure-listener", false, "explicitly serve every endpoint without authentication (acknowledges the risk; contradicts the auth flags)")
		requireAuth     = flag.Bool("require-auth", false, "refuse to start unless -api-token-file, -metrics-token-file or -tls-client-ca-file is set")
		windowSec       = flag.Int("window", 300, "Aggregation sliding window in seconds")
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

	sec := securityConfig{TLSCertFile: *tlsCertFile, TLSKeyFile: *tlsKeyFile, TLSClientCAFile: *tlsClientCA,
		APITokenFile: *apiTokenFile, MetricsTokenFile: *metricsToken, Insecure: *insecureListen, RequireAuth: *requireAuth}
	if err := sec.validate(); err != nil {
		log.Fatalw("invalid listener security flags", "error", err)
	}
	for _, w := range sec.warnings() {
		log.Warnw(w)
	}

	// Context with signal handling.
	ctx, cancel := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if *quotaPace && *cgroupPath == "" {
		log.Fatalw("-quota-pace needs -cgroup-path: pacing is only ever attached to an explicit cgroup")
	}

	if (*quotaPaceSync || *quotaPaceDry) && !*quotaPace {
		log.Fatalw("-quota-pace-sync and -quota-pace-dry-run need -quota-pace (and -cgroup-path)")
	}
	if *quotaPaceDry && !*quotaPaceSync {
		log.Fatalw("-quota-pace-dry-run needs -quota-pace-sync")
	}

	ports, err := loader.ParsePorts(*inferPorts)
	if err != nil {
		log.Fatalw("invalid -infer-ports", "error", err)
	}

	// ---- Load eBPF programs ----
	mgr, err := loader.New(loader.Config{
		InferPorts: ports,
		Dir:        *ebpfDir, Iface: *iface, CgroupPath: *cgroupPath,
		NCCLLib: *ncclLib, CUDALib: *cudaLib, CuFileLib: *cufileLib, UCXLib: *ucxLib, UprobePID: *uprobePID,
		QuotaPace: *quotaPace,
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
	node := os.Getenv("NODE_NAME")
	identity := flight.NewResolver(node, "/host/proc")
	if node != "" {
		go identity.Run(ctx, func(err error) { log.Warnw("pod identity refresh failed", "error", err) })
	}
	recorder := flight.New(node)
	agg := aggregator.New(*windowSec)
	graphBuilder := graph.NewBuilder()
	detector := anomaly.NewDetector(log)

	// GPU and security decoders.
	gpuDecoder := decoder.NewGPUDecoder(4096)
	secDecoder := decoder.NewSecurityDecoder(4096)
	fabricDecoder := fabric.NewDecoder(4096)
	fabricFolder := fabric.NewFolder(fabric.DefaultWindow)
	// Attribute signals to (namespace, job) through the Flight Recorder's resolver: host PID ->
	// pod UID -> pod -> gryvia.io/job label. It fails closed, so anything it cannot resolve stays
	// under "_unattributed".
	fabricBinder := fabric.NewBinder(fabricFolder, func(pid uint32) (string, string, bool) {
		id, ok := identity.Resolve(pid)
		return id.Namespace, id.Job, ok
	})
	// GDS direct/bounce is only meaningful when the nvidia-fs kernel hook is attached.
	for _, s := range mgr.Status() {
		if s.Object == "gds_trace.o" && s.Target == "nvidia_fs_read" && s.Attached {
			fabricFolder.SetGDSDirectVisible(true)
		}
	}

	// roce_cnp keeps counters only (no ring): poll them and fold the CNP delta.
	roceAttached := false
	for _, s := range mgr.Status() {
		if s.Object == "roce_cnp.o" && s.Attached {
			roceAttached = true
		}
	}
	var lastCNP, lastRoCE uint64
	pollRoCE := func() {
		cnp, err1 := mgr.ReadCounter("roce_cnp.o", "cnp_count", fabric.CNPSlotCNP)
		roce, err2 := mgr.ReadCounter("roce_cnp.o", "cnp_count", fabric.CNPSlotRoCE)
		if err1 != nil || err2 != nil {
			log.Warnw("reading roce_cnp counters", "cnp_error", err1, "roce_error", err2)
			return
		}
		dCNP, dRoCE := cnp-lastCNP, roce-lastRoCE
		if cnp < lastCNP { // counter reset (program reloaded)
			dCNP = cnp
		}
		if roce < lastRoCE {
			dRoCE = roce
		}
		lastCNP, lastRoCE = cnp, roce
		fabricFolder.AddCNP(dCNP)
		metrics.RecordRoCE(dCNP, dRoCE)
	}

	// pfc_pause keeps per-CPU counters only (no ring): poll them, fold the
	// frame delta and export the per-priority counts.
	pfcAttached := false
	for _, s := range mgr.Status() {
		if s.Object == "pfc_pause.o" && s.Attached {
			pfcAttached = true
		}
	}
	var lastPFC [fabric.PFCSlots]uint64
	pollPFC := func() {
		var cur, delta [fabric.PFCSlots]uint64
		for slot := uint32(0); slot < fabric.PFCSlots; slot++ {
			v, err := mgr.ReadCounter("pfc_pause.o", "pause_count", slot)
			if err != nil {
				log.Warnw("reading pfc_pause counters", "slot", slot, "error", err)
				return
			}
			cur[slot] = v
			delta[slot] = v - lastPFC[slot]
			if v < lastPFC[slot] { // counter reset (program reloaded)
				delta[slot] = v
			}
		}
		lastPFC = cur
		var perPrio [fabric.PFCPriorities]uint64
		copy(perPrio[:], delta[fabric.PFCSlotPrio0:])
		fabricFolder.AddPFC(delta[fabric.PFCSlotFrames])
		metrics.RecordPFC(delta[fabric.PFCSlotFrames], delta[fabric.PFCSlotLegacy], perPrio)
	}

	// quota_pace is the only program that mutates sockets. With -quota-pace the
	// Pacer owns its pace_rate map: it deletes lapsed leases and, at shutdown,
	// every entry it wrote. Without -quota-pace-sync nothing calls Pacer.Grant, so
	// no cgroup is paced until an operator inserts an entry. With it, the
	// QuotaSyncer (below) is the only caller, and only for pods on this node in
	// namespaces listed by a GryviaQuota that sets spec.network.maxEgressMbps.
	var pacer *fabric.Pacer
	if *quotaPace {
		if pm := mgr.Map("quota_pace.o", loader.PaceRateMap); pm != nil {
			ttl := fabric.DefaultLeaseTTL
			if *quotaPaceSync {
				ttl = fabric.QuotaSyncLeaseTTL // renewed every sync; an API outage ends pacing within minutes
			}
			pacer = fabric.NewPacer(fabric.NewEBPFPaceMap(pm), ttl, log)
			go pacer.Run(ctx, 10*time.Second)
			defer func() { _ = pacer.Shutdown() }()
			log.Warnw("quota pacing enabled: the pace_rate map is empty until a lease is granted",
				"cgroup_path", *cgroupPath, "lease_ttl", ttl.String(), "quota_pace_sync", *quotaPaceSync, "dry_run", *quotaPaceDry)
			if *quotaPaceSync {
				startQuotaSync(ctx, log, pacer, node, *cgroupPath, *quotaPaceDry)
			}
		} else {
			log.Warnw("-quota-pace set but quota_pace.o is not loaded (missing object or verifier rejection); pacing stays off")
		}
	}

	// Publishing fabric status writes to the cluster, so it needs an explicit flag and a cluster.
	if *publishFabric {
		client, err := kube.NewInCluster()
		if err != nil {
			log.Warnw("-publish-fabric-status ignored", "error", err)
		} else {
			go (&fabric.Publisher{API: client, Folder: fabricFolder, Log: log}).Run(ctx)
			log.Infow("publishing fabric status to GryviaFabricSignal (existing objects only)", "interval", fabric.PublishInterval.String())
		}
	}

	// Per-node fabric health for fabric-aware scheduling: opt-in, writes one object named after this node.
	if *publishNodeFab {
		client, err := kube.NewInCluster()
		switch {
		case err != nil:
			log.Warnw("-publish-node-fabric ignored", "error", err)
		case node == "":
			log.Warnw("-publish-node-fabric ignored: NODE_NAME is not set")
		default:
			go (&fabric.NodePublisher{API: client, Folder: fabricFolder, Node: node, Log: log}).Run(ctx)
			log.Infow("publishing node fabric health to GryviaNodeFabric", "node", node, "interval", fabric.NodePublishInterval.String())
		}
	}

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
				if id, ok := identity.Resolve(ev.PID); ok {
					ev.SrcPod, ev.Namespace = id.Pod, id.Namespace
					kind := "flow"
					if ev.Verdict == 2 {
						kind = "tcp_retransmit"
					}
					observation := flight.Event{Identity: id, Source: "ebpf", Kind: kind, Bytes: uint64(ev.Bytes), DurationNs: ev.LatencyNs}
					if ev.Verdict == 2 {
						observation.Retransmits, observation.Bytes = ev.Bytes, 0
					}
					recorder.Record(observation)
				}
				if ev.Verdict == 2 {
					continue // observation, not traffic or a drop
				}
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
	fabricRings := mgr.RingBufReaders(loader.ClassFabric)
	for _, rr := range fabricRings {
		go fabricDecoder.DecodeRingBuf(rr)
	}
	log.Infow("event readers started",
		"flow_perf", len(mgr.PerfReaders(loader.ClassFlow)),
		"gpu_ringbuf", len(gpuRings), "security_ringbuf", len(secRings),
		"fabric_ringbuf", len(fabricRings))

	// ---- GPU event processing pipeline ----
	go func() {
		for ev := range gpuDecoder.Events() {
			if id, ok := identity.Resolve(ev.PID); ok {
				kind := "gpu_api"
				switch ev.EventType {
				case decoder.GPUEvtNCCLOp:
					kind = "nccl_collective"
				case decoder.GPUEvtRDMASend, decoder.GPUEvtRDMARecv:
					kind = "rdma_kernel"
				case decoder.GPUEvtPipeStall:
					kind = "pipeline_stall"
				case decoder.GPUEvtMemTransfer:
					kind = "cuda_memcpy"
				}
				observation := flight.Event{Identity: id, Source: "ebpf", Kind: kind, Bytes: ev.Bytes, DurationNs: ev.LatencyNs}
				if ev.EventType == decoder.GPUEvtNCCLOp {
					observation.Operation = decoder.NCCLOpName(ev.NCCLOp)
				}
				recorder.Record(observation)
			}
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

	// ---- Fabric signal pipeline (straggler, RDMA health, GDS, overlap, inference wait, UCX, weight exfil) ----
	go func() {
		for sig := range fabricDecoder.Events() {
			fabricBinder.Add(sig)
			if sig.Type == fabric.SigExfil {
				// Observe only: the probe emits one signal per read burst. Never enforced here.
				log.Warnw("possible model-weight exfiltration: large model-file read followed by a connect to a non-internal address",
					"pid", sig.PID, "comm", sig.Comm, "dest", sig.ExfilDest(),
					"bytes_read", sig.Bytes, "read_to_connect_ms", sig.LatencyNS/1_000_000)
			}
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

				if roceAttached {
					pollRoCE()
				}
				if pfcAttached {
					pollPFC()
				}

				// Publish per-job fabric status.
				metrics.RecordFabric(fabricFolder.List())
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

	// Fabric signals: per-job straggler / RDMA / GDS status and score penalty.
	mux.HandleFunc("/api/v1/fabric", fabricFolder.ServeHTTP)

	// Security endpoint.
	mux.HandleFunc("/api/v1/security/alerts", secAgg.ServeHTTP)

	// AI/training endpoints.
	mux.HandleFunc("/api/v1/ai/training", trainingAnalyzer.ServeHTTP)
	mux.HandleFunc("/api/v1/ai/pipeline", pipelineAnalyzer.ServeHTTP)
	mux.Handle("/api/v1/flight/diagnose", flightAuth(recorder, *flightTokenFile))

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

	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	srv := &http.Server{
		Addr:              *metricsAddr,
		Handler:           sec.authMiddleware(mux, time.Now),
		ReadHeaderTimeout: 10 * time.Second,
	}
	if sec.tlsEnabled() {
		rl, err := newReloader(sec.TLSCertFile, sec.TLSKeyFile, sec.TLSClientCAFile,
			func(err error) { log.Warnw("TLS reload failed", "error", err) })
		if err != nil {
			log.Fatalw("cannot load TLS material", "error", err)
		}
		srv.TLSConfig = rl.tlsConfig()
	}

	go func() {
		log.Infow("starting HTTP server", "addr", *metricsAddr, "tls", sec.tlsEnabled(),
			"mtls", sec.TLSClientCAFile != "", "auth_enforced", sec.enforcing())
		var err error
		if sec.tlsEnabled() {
			err = srv.ListenAndServeTLS("", "")
		} else {
			err = srv.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			log.Fatalw("HTTP server failed", "error", err)
		}
	}()

	// Wait for shutdown.
	<-ctx.Done()
	log.Info("shutting down collector")
	if pacer != nil {
		// Fail open first: remove every pace entry this collector wrote.
		if err := pacer.Shutdown(); err != nil {
			log.Errorw("removing pace entries at shutdown", "error", err)
		}
	}
	if err := srv.Close(); err != nil {
		log.Errorw("HTTP server close error", "error", err)
	}
}

// encodeJSON is a helper to encode JSON responses.
func encodeJSON(w http.ResponseWriter, v interface{}) error {
	enc := json.NewEncoder(w)
	return enc.Encode(v)
}

// startQuotaSync starts the GryviaQuota -> pace lease reconcile loop. Every
// prerequisite is checked and a missing one leaves pacing inert (with a log
// line): running in a cluster, NODE_NAME, the collector's own namespace (its
// pods are never paced) and a cgroup v2 root.
func startQuotaSync(ctx context.Context, log *zap.SugaredLogger, pacer *fabric.Pacer, node, cgroupPath string, dryRun bool) {
	client, err := kube.NewInCluster()
	if err != nil {
		log.Warnw("-quota-pace-sync ignored", "error", err)
		return
	}
	if node == "" {
		log.Warnw("-quota-pace-sync ignored: NODE_NAME is not set")
		return
	}
	own, err := os.ReadFile(kube.SADir + "/namespace")
	if err != nil || strings.TrimSpace(string(own)) == "" {
		log.Warnw("-quota-pace-sync ignored: cannot determine the collector's own namespace", "error", err)
		return
	}
	src := &fabric.KubeSources{API: client, Node: node}
	s := fabric.NewQuotaSyncer(src, src, &fabric.HostCgroups{Root: cgroupPath}, pacer, strings.TrimSpace(string(own)), dryRun, log)
	go s.Run(ctx, fabric.QuotaSyncInterval)
	log.Warnw("quota pace sync running", "interval", fabric.QuotaSyncInterval.String(), "dry_run", dryRun, "node", node,
		"own_namespace", strings.TrimSpace(string(own)), "cgroup_root", cgroupPath)
}
