# Network Intelligence Guide

Complete guide to Gryvia's eBPF-powered network intelligence system for deep observability, security, and performance optimization of GPU clusters.

:::caution Flow sources and eBPF status
Network flows can come from two places. **[Netra](https://github.com/zyvorai/netra)**, the separate standalone eBPF
network observability product: set `apiGateway.netra.url` to an address the gateway pod can reach (its public IP, for
example `https://<netra-public-ip>:30870`; a `*.svc` name only works when Netra runs in the same cluster) and
`apiGateway.netra.tokenSecret` for its API token. Netra has its own license; Gryvia only calls its HTTP API. Or
Gryvia's **own collector** (the `ebpf-collector` DaemonSet of the `network-intelligence` chart, `ebpf.enabled=true`,
off by default).

The 27 eBPF programs in `ebpf/` are now CO-RE (no per-kernel builds; they need a node kernel with BTF, and the
`tcx` programs Linux 6.6+). All 27 compile and pass the kernel verifier on Linux 7.0 x86_64, and on that host the
collector attached 36 of the 83 hooks (the kprobes and tracepoints) and decoded real TCP flows. **Not yet verified:**
arm64 (compiles, never loaded), the XDP/TCX/sockops programs (attach is config-gated: `ebpf.interface`,
`ebpf.cgroupPath`), and everything GPU-related (NCCL/CUDA uprobes, RDMA), which needs GPU or RDMA hardware. RDMA
kprobes attach only where the driver exports those symbols. Attach results per node are at
`GET :9090/api/v1/ebpf/status`. The operator, CRDs and CLI described below work on whichever source you use. With
neither source the network graph, flows and security pages stay empty.
:::

## Table of Contents

1. [Architecture](#architecture)
2. [eBPF Programs](#ebpf-programs)
3. [CRDs](#crds)
4. [CLI Commands](#cli-commands)
5. [Deployment](#deployment)
6. [Best Practices](#best-practices)

---

## Architecture

Gryvia's network intelligence stack uses eBPF programs attached to kernel hooks to collect fine-grained telemetry without application modification. The data flows through four layers:

```
+--------------------+     +------------------+     +-------------------+     +--------+
|   eBPF Programs    | --> |  Flow Collector  | --> |     Operator      | --> |  CRDs  |
| (kernel-attached)  |     | (per-node agent) |     | (control plane)   |     |        |
+--------------------+     +------------------+     +-------------------+     +--------+
        |                         |                         |                     |
  Kernel hooks              Aggregates &              Reconciles CRDs,      User-facing
  (TC, XDP, kprobe,        enriches flows            generates policies,    resources &
   tracepoint, cgroup)     with pod metadata          detects anomalies     status
```

**eBPF Programs** run in the kernel on every node and capture packet, socket, syscall, and GPU-level events with low overhead by design. Overhead has not yet been measured; see the benchmark suite.

**Flow Collector** is a per-node DaemonSet that reads eBPF map data, enriches flows with Kubernetes pod/service metadata, and exports to the operator.

**Operator** runs as a Deployment in the control plane. It reconciles network intelligence CRDs, correlates cross-node flows, detects anomalies, and generates or enforces network policies.

**CRDs** are the user-facing interface for configuring network policies, viewing traffic insights, running trace sessions, and managing security rules.

---

## eBPF Programs

Gryvia ships 27 eBPF programs organized into seven categories. All programs are loaded and managed by the Flow Collector DaemonSet.

### GPU Programs

| Program | Hook Type | Description |
|---------|-----------|-------------|
| `nccl_trace` | uprobe | Traces NCCL collective operations (AllReduce, AllGather, Broadcast) with timing, message size, and ring/tree algorithm details. Identifies communication bottlenecks in distributed training. |
| `gpu_mem_trace` | kprobe | Monitors GPU memory allocations and deallocations via the NVIDIA kernel driver. Detects memory leaks, fragmentation, and OOM patterns before they cause job failures. |
| `rdma_trace` | tracepoint | Traces RDMA/InfiniBand verbs (post_send, post_recv, poll_cq) for RoCE and IB gryvias. Measures RDMA latency, throughput, and error rates per queue pair. |

### Fabric Signal Programs

Ten programs are listed here. Nine feed the scheduler-facing fabric signals and are observe-only (nothing is dropped or modified); `quota_pace` is the one exception, is off by default and is described last. Those that use a ring buffer
emit `struct fabric_signal` on their own `fabric_events` ring buffer instead of extending the frozen 72-byte `gpu_event`.
The collector folds the signals per job over a 5-minute window and serves them, with a `[0,1]` score penalty, at
`GET :9090/api/v1/fabric` and as `gryvia_fabric_*` Prometheus gauges. The `GryviaFabricSignal` CRD (short name `gfs`)
carries the same fields in its status; no controller fills it yet and the score is not wired into the scheduler.

| Program | Hook Type | Description |
|---------|-----------|-------------|
| `straggler` | uprobe (`ncclAllReduce`) | Flags a rank whose collective takes more than 2x the fastest recent span of the same payload size (and more than 5 ms) on the same node. |
| `rdma_health` | kprobe (`ib_post_send`, `mlx5_ib_post_send`, `ib_poll_cq`, `mlx5_ib_poll_cq`) | Counts sends per QP and retry-exceeded / RNR-exceeded work completions; emits a signal above thresholds set in the `rdma_thresh` map. Every probe is optional. |
| `gds_trace` | uprobe (`cuFileRead`, `cuFileWrite`), kprobe (`nvidia_fs_read`) | Counts bytes and times cuFile calls; a call is "direct" only when the nvidia-fs kernel hook is also seen, otherwise it counts as bounce/unknown. Only calls slower than 2 ms are emitted. |
| `overlap` | uprobe/uretprobe (`ncclAllReduce`, `cudaDeviceSynchronize`) | Reports a `cudaDeviceSynchronize` of at least 1 ms that runs nested inside an in-flight `ncclAllReduce` on the same thread (GPU idle while communicating). Folded as `overlapIdleRatio`, the share of the window spent in such syncs. |
| `roce_cnp` | XDP | Counts RoCEv2 congestion notification packets (UDP 4791, BTH opcode 0x81) in per-CPU counters; always returns `XDP_PASS`. Attached only with `-iface`; the collector polls the counters and folds them as `cnpRate` (packets per second, job `_node/cnp`) and `gryvia_roce_cnp_packets_total`. |
| `infer_latency` | kretprobe (`inet_csk_accept`), kprobe (`tcp_recvmsg`) | Time from accept to the first read on connections to the inference ports (`-infer-ports`, default `8000,8001`: vLLM and Triton HTTP; skipped when the list is empty). Folded as `inferWaitP99ms`, informational only. |
| `ucx_gloo` | uprobe/uretprobe (`ucp_tag_send_nb`, `ucp_tag_send_nbx`) | UCX tag-send calls that blocked for at least 5 ms (the span of the posting call, not of the transfer). Skipped when `libucp.so` is not found (`-ucx-lib`, `-uprobe-pid`). Gloo is **not** probed: its allreduce symbols are C++ mangled, vary by version and are normally linked statically into `libtorch_cpu.so`, so a fixed probe could never attach. Folded as `ucxSlowP99ms`, informational only. |
| `pfc_pause` | XDP | Counts 802.1Qbb priority-flow-control pause frames (EtherType 0x8808, opcode 0x0101) in per-CPU counters, with a count per paused priority, plus 802.3x pause frames; always `XDP_PASS`. Attached only with `-iface`; folded as `pfcRate` (frames per second, job `_node/pfc`) and `gryvia_pfc_*_total` counters. Only one XDP program can own an interface, so it conflicts with `roce_cnp`, `packet_filter` and `dns_tracker`: the collector attaches the first one and skips the others with a logged reason instead of replacing it. Many NICs handle pause frames in the MAC and never pass them to XDP, in which case the counters stay 0. |
| `weight_exfil` | kprobe (`vfs_read`, `tcp_v4_connect`) | Observe only. A read of at least 8 MiB from a model-weight file (by name: `.safetensors`, `.gguf`, `.ckpt`, `.onnx`, `.pt`, `.pth`, `.bin`, `.h5`), followed within 30 s by a `connect` from the same process to a destination outside loopback, RFC1918 and link-local ranges, produces one signal (also logged as a warning). A read alone never does. It is a heuristic (IPv4 only, `read()` only, name-based) and a model server that legitimately calls an external API after loading weights will trip it. Folded as `exfilEvents`; it never changes `scoreDelta`. |
| `quota_pace` | sockops (cgroup v2) | **The only program that changes anything, and it is off by default.** When a process in a cgroup that has an entry in the `pace_rate` map opens an outbound TCP connection, the socket's `SO_MAX_PACING_RATE` is lowered to that rate (never raised, and never below 1 Mbit/s). No entry, no change. Attached only with `-quota-pace` **and** `-cgroup-path`. Entries are lease-gated (15 minutes, `Pacer` in `collector/pkg/fabric`): expiry deletes the entry and the collector deletes every entry it wrote when it shuts down. Sockets that are already paced keep their rate until they close; only new connections are unpaced. **Nothing grants leases yet:** there is no `GryviaQuota` watch and no API, so enabling the flag only attaches the program and paces nothing until an operator or a future controller writes an entry. |

The fabric score penalty (`scoreDelta`, capped at 1) adds 0.15 when `overlapIdleRatio` exceeds 0.3, 0.15 when
`cnpRate` exceeds 100 packets per second and 0.10 when `pfcRate` exceeds 1000 pause frames per second, on top of the
straggler, RDMA and GDS terms. `exfilEvents`, `ucxSlowP99ms` and `inferWaitP99ms` are informational. The thresholds are heuristics
that have not been calibrated on real fabrics. `FabricPenalty` in the ai-operator scheduler package turns `scoreDelta`
into up to 25 points to subtract, but nothing calls it yet.

:::caution Unverified on hardware
These programs compile and pass the kernel verifier (Linux 7.0 x86_64; arm64 compiles only). `roce_cnp` and
`pfc_pause` were run on crafted packets with `BPF_PROG_TEST_RUN`, `overlap` and `ucx_gloo` on stand-in libraries,
`weight_exfil` briefly on the live hooks of a test process, and `quota_pace` on a throwaway cgroup on that host; none of it has run on real UCX,
PFC or model-serving workloads or on arm64 hardware. The GPU, RDMA
and GDS paths, real RoCE traffic and `infer_latency` attached to a live server have not been exercised on GPU, RDMA or GPUDirect Storage hardware. On the test host none of
`ib_post_send`, `ib_poll_cq`, `mlx5_ib_*` or `nvidia_fs_read` exists, and the collector skips those hooks with a log
line (they show as `skipped: symbol not found` in `/api/v1/ebpf/status`). `ib_post_send` and `ib_poll_cq` are inline
wrappers, so kernel-side hooks only see in-kernel RDMA users; user-space verbs (NCCL) bypass them. The straggler span
is the host-side duration of the NCCL call, which is the enqueue time for asynchronous collectives. Inside pods the
uprobes need the library resolved through `/proc/<pid>/root` (`-uprobe-pid`); the per-container mount-namespace
resolver is not built yet. Signals are attributed to a job by pid; until the cgroup-to-pod resolver binds pids, they
are grouped under `_unattributed` by process name.
:::

### Security Programs

| Program | Hook Type | Description |
|---------|-----------|-------------|
| `container_escape` | kprobe | Detects container escape attempts by monitoring namespace changes, capability escalation, and suspicious mount operations. |
| `crypto_detect` | kprobe | Identifies cryptocurrency mining by detecting specific instruction patterns and connections to known mining pools. |
| `exfil_detect` | TC | Detects data exfiltration patterns including large outbound transfers, DNS tunneling, and connections to suspicious external destinations. |
| `privesc_monitor` | kprobe | Monitors privilege escalation attempts including setuid calls, capability changes, and kernel module loading from containers. |
| `driver_fim` | kprobe | File integrity monitoring for GPU driver files and kernel modules. Alerts on unauthorized modifications to critical driver components. |

### Performance Programs

| Program | Hook Type | Description |
|---------|-----------|-------------|
| `tcp_tuning` | sockops | Dynamically tunes TCP parameters (buffer sizes, congestion control, window scaling) per connection based on observed traffic patterns. |
| `connpool_analyze` | kprobe | Analyzes connection pooling behavior to detect connection storms, idle connection waste, and suboptimal pool sizing. |
| `numa_path` | tracepoint | Tracks NUMA-aware memory access patterns and identifies cross-NUMA traffic that degrades GPU-to-GPU communication. |
| `sockops_optimize` | sockops | Accelerates local pod-to-pod communication by short-circuiting the TCP stack for connections within the same node. |

### Observability Programs

| Program | Hook Type | Description |
|---------|-----------|-------------|
| `trace_correlator` | kprobe | Correlates distributed traces across pods by extracting and propagating trace context (W3C Trace Context, B3) from network packets. |
| `latency_breakdown` | kprobe/TC | Decomposes end-to-end latency into kernel, network, and application components. Identifies which layer contributes most to tail latency. |
| `cost_tracker` | TC | Tracks per-flow byte counts and maps them to cost attribution. Enables accurate network cost allocation per team, job, and service. |
| `fingerprint` | TC | Generates traffic fingerprints for services based on packet size distributions, timing patterns, and protocol usage. Used for anomaly detection baselines. |

### AI-Specific Programs

| Program | Hook Type | Description |
|---------|-----------|-------------|
| `training_pattern` | uprobe/TC | Analyzes distributed training communication patterns to identify synchronization barriers, straggler nodes, and gradient aggregation inefficiencies. |
| `datapipe_bottleneck` | kprobe | Detects data pipeline bottlenecks by monitoring data loader throughput, storage I/O patterns, and prefetch queue depths. |
| `gradient_compress` | TC | Monitors gradient compression ratios and communication volume in distributed training. Identifies opportunities for gradient compression optimization. |

### Core Programs

| Program | Hook Type | Description |
|---------|-----------|-------------|
| `tcp_trace` | kprobe | Core TCP flow tracing with connection state tracking, retransmit monitoring, and per-flow byte/packet counters. |
| `packet_filter` | XDP | High-performance packet filtering at the XDP layer for early-drop of unauthorized traffic. Used by GryviaFlowPolicy enforcement. |
| `latency_probe` | TC | Measures per-packet network latency using kernel timestamps. Provides P50/P90/P99 latency distributions per flow. |
| `syscall_monitor` | tracepoint | Monitors network-related syscalls (connect, accept, sendmsg, recvmsg) with per-container attribution. |
| `dns_tracker` | TC | Tracks DNS queries and responses with latency. Detects DNS-based service discovery issues and resolution failures. |

---

## CRDs

### GryviaFlowPolicy

Intent-based network policies that describe allowed traffic using high-level semantics rather than raw IP/port rules.

```yaml
apiVersion: gryvia.io/v1alpha1
kind: GryviaFlowPolicy
metadata:
  name: training-data-access
  namespace: ml-research
spec:
  # Intent-based policy definition
  intent: allow
  description: "Allow training jobs to access data lake and model registry"

  # Source selector
  source:
    matchLabels:
      app: training-job
      team: ml-research

  # Destination rules
  destination:
    - service: data-lake
      namespace: storage
      ports: [8080, 443]
      protocol: TCP

    - service: model-registry
      namespace: ml-platform
      ports: [443]
      protocol: TCP

  # Traffic shaping
  rateLimit:
    requestsPerSecond: 10000
    burstSize: 5000

  # Logging
  logging:
    enabled: true
    sampleRate: 100   # Log 1 in 100 flows

  # Enforcement mode
  enforcement: enforce   # audit | enforce
```

### GryviaTrafficInsight

Per-service traffic metrics aggregated from eBPF flow data.

```yaml
apiVersion: gryvia.io/v1alpha1
kind: GryviaTrafficInsight
metadata:
  name: training-cluster-insight
  namespace: ml-research
spec:
  # Target service or workload
  target:
    kind: Deployment
    name: training-coordinator
    namespace: ml-research

  # Metrics collection interval
  interval: 30s

  # Metrics to collect
  metrics:
    - bytesIn
    - bytesOut
    - packetsIn
    - packetsOut
    - connections
    - latencyP50
    - latencyP99
    - retransmits
    - dnsLatency

  # Retention period
  retention: 7d

status:
  lastUpdated: "2024-01-15T12:30:00Z"
  metrics:
    bytesIn: 1.2Ti
    bytesOut: 856Gi
    connections: 45230
    latencyP50ms: 0.8
    latencyP99ms: 12.5
    retransmitRate: 0.02%
  topSources:
    - service: data-loader
      bytesOut: 800Gi
    - service: gradient-aggregator
      bytesOut: 56Gi
  topDestinations:
    - service: parameter-server
      bytesIn: 1.1Ti
```

### GryviaAutoPolicy

Self-healing firewall that learns traffic patterns and generates or enforces network policies automatically.

```yaml
apiVersion: gryvia.io/v1alpha1
kind: GryviaAutoPolicy
metadata:
  name: ml-namespace-autopolicy
  namespace: ml-research
spec:
  # Operating mode
  # learn: Observe traffic and build baseline (no enforcement)
  # suggest: Generate policy recommendations for review
  # enforce: Automatically apply learned policies
  mode: suggest

  # Scope
  scope:
    namespaces: [ml-research, ml-staging]

  # Learning configuration
  learning:
    duration: 7d
    minConfidence: 0.95
    excludePorts: [53, 443]   # Don't restrict DNS and HTTPS

  # Policy generation
  policyGeneration:
    defaultDeny: true
    granularity: service      # service | pod | namespace
    mergeThreshold: 0.8       # Merge similar rules above this similarity

  # Notifications
  notifications:
    slack: "#ml-security"
    onNewPolicy: true
    onBlockedTraffic: true

status:
  phase: Suggesting
  learnedFlows: 12450
  generatedPolicies: 23
  lastSuggestion: "2024-01-15T12:00:00Z"
  suggestions:
    - name: allow-training-to-datastore
      confidence: 0.98
      description: "Training pods regularly access datastore on port 6379"
```

### GryviaTraceSession

On-demand network debugging sessions for troubleshooting connectivity and performance issues.

```yaml
apiVersion: gryvia.io/v1alpha1
kind: GryviaTraceSession
metadata:
  name: debug-training-latency
  namespace: ml-research
spec:
  # Trace target
  target:
    pod: training-worker-0
    namespace: ml-research

  # Capture filters
  filters:
    - protocol: TCP
      destPort: 29500      # PyTorch distributed port
    - protocol: TCP
      destPort: 2049       # NFS

  # Capture duration
  duration: 5m

  # Packet capture settings
  capture:
    maxPackets: 100000
    snapLength: 256        # Bytes per packet to capture
    includePayload: false

  # Analysis options
  analysis:
    latencyBreakdown: true
    retransmitAnalysis: true
    flowCorrelation: true

status:
  phase: Completed
  startTime: "2024-01-15T12:00:00Z"
  endTime: "2024-01-15T12:05:00Z"
  capturedPackets: 45230
  results:
    avgLatency: 2.3ms
    p99Latency: 15.8ms
    retransmitRate: 0.5%
    findings:
      - severity: warning
        message: "High retransmit rate on NFS connections from training-worker-0 to nfs-server"
        recommendation: "Check NFS server disk I/O and network MTU settings"
```

### GryviaServiceGraph

Service dependency visualization generated from observed network traffic.

```yaml
apiVersion: gryvia.io/v1alpha1
kind: GryviaServiceGraph
metadata:
  name: ml-platform-graph
spec:
  # Scope
  namespaces: [ml-research, ml-platform, storage]

  # Discovery settings
  discovery:
    includeExternal: true
    protocol: true          # Include protocol-level details
    refreshInterval: 5m

  # Display options
  layout: hierarchical
  groupBy: namespace

status:
  lastUpdated: "2024-01-15T12:30:00Z"
  services: 24
  edges: 67
  externalEndpoints: 5
  graph:
    nodes:
      - name: training-coordinator
        namespace: ml-research
        type: Deployment
        replicas: 1
      - name: data-lake
        namespace: storage
        type: StatefulSet
        replicas: 3
    edges:
      - source: training-coordinator
        destination: data-lake
        protocol: TCP
        port: 8080
        bytesPerSecond: 125000000
        requestsPerSecond: 450
```

### GryviaNetworkAnomaly

Anomaly detection rules and alerts for network traffic patterns.

```yaml
apiVersion: gryvia.io/v1alpha1
kind: GryviaNetworkAnomaly
metadata:
  name: training-anomaly-detector
  namespace: ml-research
spec:
  # Detection scope
  scope:
    namespaces: [ml-research]

  # Detection rules
  rules:
    - name: traffic-spike
      type: volumeAnomaly
      baseline: 7d
      threshold: 3.0         # Standard deviations from baseline
      severity: warning

    - name: new-external-connection
      type: newDestination
      scope: external
      severity: critical
      action: alert

    - name: latency-degradation
      type: latencyAnomaly
      metric: p99
      baseline: 24h
      threshold: 2.0
      severity: warning

    - name: connection-storm
      type: connectionRate
      maxNewConnections: 1000
      window: 1m
      severity: critical

  # Alerting
  alerts:
    slack: "#ml-security"
    email: ml-team@example.com
    webhook: https://pagerduty.example.com/webhook

status:
  activeAnomalies: 2
  anomalies:
    - rule: latency-degradation
      detected: "2024-01-15T11:45:00Z"
      severity: warning
      description: "P99 latency for training-coordinator increased from 5ms to 18ms"
      affectedPods: [training-worker-0, training-worker-3]
```

### GryviaSecurityPolicy

Security detection rules for identifying threats and policy violations.

```yaml
apiVersion: gryvia.io/v1alpha1
kind: GryviaSecurityPolicy
metadata:
  name: gpu-cluster-security
  namespace: gryvia-system
spec:
  # Detection rules
  rules:
    - name: crypto-mining-detection
      program: crypto_detect
      enabled: true
      action: block
      severity: critical
      alert: true

    - name: container-escape-detection
      program: container_escape
      enabled: true
      action: alert
      severity: critical

    - name: data-exfiltration
      program: exfil_detect
      enabled: true
      action: alert
      severity: high
      config:
        maxOutboundMB: 1000    # Alert on large outbound transfers
        suspiciousDomains: true
        dnsExfiltration: true

    - name: privilege-escalation
      program: privesc_monitor
      enabled: true
      action: block
      severity: critical

    - name: driver-integrity
      program: driver_fim
      enabled: true
      action: alert
      severity: high
      config:
        paths:
          - /usr/lib/x86_64-linux-gnu/libnvidia-*
          - /usr/lib/modules/*/nvidia*

  # Global alert configuration
  alerting:
    slack: "#security-alerts"
    pagerduty:
      serviceKey: "abc123"
      severity: critical
```

### GryviaNetworkCost

Network cost attribution per team, job, and service.

```yaml
apiVersion: gryvia.io/v1alpha1
kind: GryviaNetworkCost
metadata:
  name: monthly-network-costs
spec:
  # Reporting period
  period:
    type: monthly

  # Cost rates
  rates:
    intraNode: 0.00          # Free within a node
    intraCluster: 0.01       # $0.01/GB within cluster
    crossZone: 0.02          # $0.02/GB cross-zone
    internet: 0.09           # $0.09/GB to internet
    rdma: 0.005              # $0.005/GB RDMA traffic

  # Scope
  scope:
    groupBy: [team, job, namespace]

status:
  lastCalculated: "2024-01-15T00:00:00Z"
  totalCost: 1234.56
  breakdown:
    byTeam:
      - team: ml-research
        cost: 890.12
        trafficGB: 45230
      - team: ml-production
        cost: 344.44
        trafficGB: 12340
    byType:
      intraCluster: 452.30
      crossZone: 246.80
      internet: 111.06
      rdma: 424.40
```

### GryviaTrainingInsight

AI training-specific network analysis for distributed training jobs.

```yaml
apiVersion: gryvia.io/v1alpha1
kind: GryviaTrainingInsight
metadata:
  name: llm-training-insight
  namespace: ml-research
spec:
  # Target training job
  jobRef:
    name: llm-distributed-training
    namespace: ml-research

  # Analysis options
  analysis:
    ncclProfiling: true
    stragglerDetection: true
    gradientAnalysis: true
    communicationPattern: true

status:
  lastUpdated: "2024-01-15T12:30:00Z"
  workers: 8
  communicationPattern: ring-allreduce
  ncclMetrics:
    allReduceTimeMs: 12.5
    allGatherTimeMs: 8.3
    broadcastTimeMs: 2.1
    totalCommTimePercent: 18.5
  stragglers:
    - worker: training-worker-3
      avgIterationMs: 245
      clusterAvgMs: 220
      delta: 11.4%
      cause: "Cross-NUMA GPU memory access"
  gradientMetrics:
    avgGradientSizeMB: 125
    compressionRatio: 1.0
    recommendation: "Enable gradient compression for 2.3x communication speedup"
  bottleneck:
    type: communication
    component: allreduce
    recommendation: "Switch to hierarchical allreduce for 8+ node jobs"
```

### GryviaInferenceInsight

Serving latency breakdown and optimization analysis for inference services.

```yaml
apiVersion: gryvia.io/v1alpha1
kind: GryviaInferenceInsight
metadata:
  name: llm-serving-insight
  namespace: ml-production
spec:
  # Target inference service
  serviceRef:
    name: llama-3-serving
    namespace: ml-production

  # Analysis options
  analysis:
    latencyBreakdown: true
    batchingAnalysis: true
    cacheAnalysis: true
    throughputProfiling: true

status:
  lastUpdated: "2024-01-15T12:30:00Z"
  requestsPerSecond: 450
  latency:
    total:
      p50ms: 35
      p90ms: 62
      p99ms: 95
    breakdown:
      networkIngress: 1.2ms
      queueWait: 5.3ms
      tokenization: 2.1ms
      inference: 24.5ms
      detokenization: 0.8ms
      networkEgress: 1.1ms
  batching:
    avgBatchSize: 12
    maxBatchSize: 64
    batchUtilization: 18.7%
    recommendation: "Increase max_batch_wait to 10ms to improve batch utilization to ~45%"
  kvCache:
    hitRate: 78.5%
    memoryUsedGB: 24.3
    evictions: 1250
  throughput:
    tokensPerSecond: 12500
    peakTokensPerSecond: 18200
```

---

## CLI Commands

### Network Tracing

Traces target a service. Use `--level` to choose the capture depth (`l3`, `l4`, `l7`; default `l7`).

```bash
# Start a trace of a service for 5 minutes
gryvia network trace training-worker --duration 5m --trace-namespace ml-research

# Trace at L4 only (TCP/UDP, no payload parsing) for 2 minutes
gryvia network trace training-worker --level l4 --duration 2m --trace-namespace ml-research

# Follow a live trace
gryvia network trace training-worker --follow --trace-namespace ml-research

# View trace sessions and their results
kubectl get gryviatracesessions -n ml-research
kubectl get gryviatracesession debug-training-latency -n ml-research -o yaml
```

### Flow Analysis

```bash
# View recent flows in a namespace
gryvia network flows --flow-namespace ml-research

# Flows for a specific service over the last hour
gryvia network flows --service training-worker --flow-namespace ml-research --last 1h

# Export flows as JSON
gryvia network flows --flow-namespace ml-research --output json
```

### Service Graph

```bash
# View service dependency graph (ASCII)
gryvia network graph --graph-namespace ml-research

# Export the graph as JSON
gryvia network graph --graph-namespace ml-research --format json > graph.json
```

### Policy Management

```bash
# List active flow policies
gryvia network policy list --policy-namespace ml-research

# Apply a flow policy from a manifest
kubectl apply -f flow-policy.yaml

# View autopolicy suggestions
gryvia network policy suggest --policy-namespace ml-research

# Accept (apply) an autopolicy suggestion
gryvia network policy apply suggestion-name --policy-namespace ml-research
```

To evaluate a policy without enforcing it, set the policy's mode in its manifest (for a
GryviaAutoPolicy, `learn` or `suggest`) instead of `enforce`.

### Anomaly Detection

```bash
# View active anomalies
gryvia network anomalies --anomaly-namespace ml-research

# Filter by severity
gryvia network anomalies --severity critical --anomaly-namespace ml-research

# Filter by service
gryvia network anomalies --service training-worker --anomaly-namespace ml-research

# View the full detail of one anomaly
kubectl get gryvianetworkanomaly latency-degradation -n ml-research -o yaml
```

### Security

```bash
# View security alerts
gryvia security alerts --security-namespace ml-research

# Filter by severity
gryvia security alerts --severity critical --security-namespace ml-research

# Filter by alert type (escape, mining, exfiltration, privesc)
gryvia security alerts --alert-type mining --security-namespace ml-research

# View security status
gryvia security status

# List security policies
gryvia security policy list --security-namespace ml-research

# Create a security policy
gryvia security policy create gpu-hardening \
  --namespaces ml-research \
  --rules escape,mining,exfiltration,privesc \
  --auto-block

# Or apply a security policy from a manifest
kubectl apply -f security-policy.yaml
```

### GPU Network Analysis

```bash
# View NCCL communication metrics
gryvia gpu nccl --job llm-distributed-training

# View GPU memory transfer stats for a node
gryvia gpu memory --node gpu-node-01

# View RDMA statistics
gryvia gpu rdma --node gpu-node-01

# Training communication analysis
gryvia gpu training --job llm-distributed-training
```

Straggler detection and gradient compression analysis are reported in the GryviaTrainingInsight
resource for the job:

```bash
kubectl get gryviatraininginsights -n ml-research
```

---

## Deployment

### Prerequisites

- Linux kernel 5.10+ (for full eBPF feature support)
- BTF (BPF Type Format) enabled in kernel (`CONFIG_DEBUG_INFO_BTF=y`)
- CAP_BPF and CAP_SYS_ADMIN for the Flow Collector DaemonSet
- NVIDIA GPU drivers for GPU-specific eBPF programs

### Install Flow Collector

The Flow Collector is deployed as a DaemonSet on every node. The `gryvia` CLI has no installer for it,
and no collector image is published yet (see the note at the top of this page). Once a collector is
deployed, verify it with:

```bash
kubectl get daemonset -n gryvia-system flow-collector
kubectl get pods -n gryvia-system -l app=flow-collector
```

### Verify eBPF Programs

```bash
# Check network health, including the collector
gryvia network status --status-namespace ml-research

# Check the collector pods for eBPF program load errors
kubectl logs -n gryvia-system -l app=flow-collector --tail 100
```

---

## Best Practices

### Performance

- Start with core programs (`tcp_trace`, `latency_probe`, `dns_tracker`) and add specialized programs as needed.
- Use `sampleRate` in GryviaFlowPolicy logging to reduce overhead on high-throughput flows.
- Set appropriate `retention` periods on GryviaTrafficInsight to control storage usage.

### Security

- Begin with GryviaAutoPolicy in `learn` mode for at least 7 days before switching to `suggest` or `enforce`.
- Enable `container_escape` and `privesc_monitor` on all GPU nodes.
- Review auto-policy suggestions before accepting, especially in multi-tenant clusters.

### Troubleshooting

- Use GryviaTraceSession for targeted debugging rather than enabling cluster-wide capture.
- Check GryviaTrainingInsight for straggler detection when distributed training performance degrades.
- Review GryviaInferenceInsight latency breakdown to identify which layer (network, queue, compute) is causing tail latency.

---

## Support

- **Documentation**: https://gryvia.io/docs
- **Issues**: https://github.com/zyvorai/gryvia/issues
- **Discussions**: https://github.com/zyvorai/gryvia/discussions
- **Slack**: #gryvia-users

---

*Gryvia - Enterprise GPU Infrastructure Management*
