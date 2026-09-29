# Changelog

All notable changes to Gryvia are recorded here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
and the project aims to follow [Semantic Versioning](https://semver.org/) once it leaves alpha.

## [Unreleased]

### Added
- Flight Recorder cluster view: `GET /api/flight/jobs/{job}?namespace=` on the gateway signs a request to every Running collector pod in `apiGateway.flightCollectorNamespace` (HMAC-SHA256 over method, request URI and timestamp with a shared token, 30 s window, replay inside it possible) and merges the node-local reports with a `coverage` field (`total`, `reachable`, `reporting`, `complete`) and a `truncated` flag; tenant callers are limited to their tenant namespaces. The collector's `GET /api/v1/flight/diagnose` now requires that token (`-flight-token-file`, chart values `ebpf.flightTokenSecret` and `apiGateway.flightTokenSecret`) and answers 503 when it is unset; the rest of the collector listener stays unauthenticated. NCCL events carry their operation name and the gateway flags ranks whose observed API duration is an outlier (an API timing, not a GPU or network cause). A Go test and a Python test pin the same signature vector. Unit-tested only; not run against a real cluster.
- Fabric signals, second part: `overlap` (a `cudaDeviceSynchronize` nested inside an in-flight `ncclAllReduce`), `roce_cnp` (XDP counter of RoCEv2 congestion notifications, always `XDP_PASS`, attached only with `-iface`) and `infer_latency` (accept to first read on inference ports, new `-infer-ports` flag, default `8000,8001`). The fabric folder adds `overlapIdleRatio`, `cnpRate` and `inferWaitP99ms` (also in the `GryviaFabricSignal` status and `gryvia_fabric_*` gauges); the first two add up to 0.15 each to `scoreDelta`, which stays in `[0,1]`. `FabricPenalty` (up to 25 points) is in the ai-operator scheduler package but is not called yet. Load-verified on Linux 7.0 (x86_64, arm64 compiles); `roce_cnp` and `overlap` were also run (test-run packets, stand-in library); real RoCE traffic, GPUs and a live inference server are untested.
- Flight Recorder (node-local preview): the collector keeps a bounded in-memory timeline (2,048 events per job, 128 jobs) of eBPF observations attributed to a `GryviaAIJob` and serves it at `GET /api/v1/flight/diagnose?namespace=&job=`. Identity comes from the host PID's cgroup pod UID matched against the node's pod list (`gryvia.io/job` label) and is never guessed: unresolved or stale (>1 min without a pod-list refresh) PIDs are dropped. `tcp_trace` now emits the retransmission count at connection close (verdict 2) with the PID captured at connect; it is excluded from flow, byte and drop accounting. Fixes: the TrainingInsight controller now reads the collector's real JSON, the NCCL and training analyzers no longer report rank 0 or a topology when the probe has no rank, `GetOpStats` no longer writes under a read lock, NCCL latency samples are bounded, and the Helm collector gets `NODE_NAME`. The endpoint is unauthenticated like the rest of the collector listener (see `docs/flight-recorder.md`); the eBPF change is verified on Linux 7.0 x86_64 only and no cluster-wide aggregation, CLI or GPU/RDMA validation exists.
- Fabric signals: three new eBPF programs (`straggler`, `rdma_health`, `gds_trace`) report NCCL rank skew, RDMA retry/RNR completions and GPUDirect Storage direct-vs-bounce reads on a separate `fabric_signal` ring buffer (`gpu_event` stays 72 bytes; observe-only). The collector decodes them (`collector/pkg/fabric`), folds them per job, and serves them with a `[0,1]` score penalty at `GET /api/v1/fabric` and as `gryvia_fabric_*` gauges; missing optional symbols (`mlx5_ib_*`, `nvidia_fs_read`) are skipped with a log line. New `GryviaFabricSignal` CRD (`gfs`) and `-cufile-lib` flag. Compiled and verifier-checked on Linux 7.0 x86_64 (arm64 compiles only); the GPU, RDMA and GDS paths are unverified on hardware, no controller fills the CRD yet, and the score is not wired into the scheduler.
- The eBPF programs are ported to CO-RE and work: all 24 in `ebpf/` compile (clang 18 and 21, x86_64 and arm64) and pass the kernel verifier on Linux 7.0; no generated `vmlinux.h` (`ebpf/headers/gryvia_core.h`), `make check` loads every object. Many programs had real bugs fixed (wrong syscall-id and register reads, hard-coded struct offsets, connection keys that never matched, unbounded loops, x86-only syscall numbers). The collector image builds again.
- The collector attaches programs by their section name (kprobe, kretprobe, uprobe, tracepoint, raw_tracepoint, tp_btf, XDP, TCX, sockops, sk_msg), tolerates individual failures, dispatches events to the flow, GPU and security decoders, and reports per-program attach results at `/api/v1/ebpf/status`. Verified live on Linux 7.0 (36 of 83 hooks attached, real TCP flows decoded); GPU, RDMA and arm64 paths are not yet verified on hardware.
- Estimate invoices: `/api/invoices`, a dashboard Invoices page and `gryvia invoice` build a monthly per-tenant statement from usage records (JSON or CSV). No payment processing.
- The admission webhook now denies jobs whose GPU type is not allowed by the namespace's quota or the tenant's catalog SKUs, or that exceed the per-job GPU limit; it fails open when the quotas cannot be read.
- GPU as a Service: tenants (`GryviaTenant`, now run by the quota operator) get an isolated `tenant-<name>` namespace; a price catalog (`GryviaGpuSku`); per-job metering (`GryviaUsageRecord`); gateway routes `/api/skus`, `/api/tenants`, `/api/usage` (+ CSV/JSON export); dashboard Catalog, Usage and Tenants pages; and `gryvia catalog`, `gryvia tenant`, `gryvia usage`. See `website/docs/guides/GPU_AS_A_SERVICE.md`.
- Roles in the gateway: the API key and sessions are the provider admin; OIDC users are tenant users limited to their tenant's namespaces (`GRYVIA_OIDC_ADMIN_GROUPS` promotes a group). Cluster-wide routes (nodes, network, security, GPU metrics) are admin-only.
- Network flows can come from [Netra](https://github.com/zyvorai/netra): set `apiGateway.netra.url` and `/api/network/flows` returns Netra's recent flows (falls back to service-graph edges when it is unset or unreachable).
- Workflows and auto tuners can be deleted from the dashboard and the gateway (`DELETE /api/workflows/{name}`, `DELETE /api/tuners/{name}`).
- GPU nodes prepare themselves: the chart can bundle NVIDIA's GPU Operator (`nvidia.enabled=true`: driver, container toolkit, device plugin, DCGM, node feature discovery), and `scripts/install-k3s-gpu.sh` takes a fresh Ubuntu server to k3s + Gryvia + GPU support (`agent` mode joins more nodes). CI runs the script on a real k3s (without a GPU) and dry-run tests for the GPU branches. See `website/docs/guides/GPU_NODES.md` and `docs/gpu-validation.md`.
- The GPU operator registers a `GryviaGpuNode` for every node labelled by NVIDIA GPU feature discovery (`--auto-register`, chart value `gpuOperator.autoRegister`) and reads readiness, driver and CUDA versions from the node's labels and allocatable GPUs; per-GPU numbers come from the DCGM exporter on that node.
- CLI in the style of `cilium`: colored `--help` with commands in groups, examples on every command, aliases, `--no-color`/`NO_COLOR`, shell completions (`gryvia completion`), `gryvia version`, concise errors with hints, and one output look (aligned tables with OK/Warning/Error markers) across all commands.
- `gryvia status` reports the whole platform like `cilium status`: a component banner, workload readiness, cluster totals, image versions, errors and a per-node table (GPU health, driver, DaemonSet pods, maintenance). `--brief`, `--wait`, `--node`, `-o json|yaml`; exits 1 when a required component fails. `-o json|yaml` is also available on `cluster`, `quota`, `queue` and `maintenance list`.
- Helm chart `helm/gryvia` installs the operators, API gateway and dashboard in one release, with optional
  Ingress, cert-manager TLS, PodDisruptionBudgets and a NetworkPolicy.
- Tag-driven release workflow: multi-arch images on ghcr.io (mirrored to Docker Hub), signed with cosign,
  OCI Helm chart and CLI binaries.
- `scripts/install.sh` and `scripts/kind-demo.sh` (a GPU-free demo with fictional data on kind).
- `POST /api/auth/login` on the gateway with rate limiting, a `usingDefaultKey` flag on `/api/auth/me`, and a
  dashboard warning while the default lab key is in use. `auth.apiKey=""` generates a random key.
- Dashboard: job logs, events and pods; searchable, sortable, paginated tables with URL state; create forms for
  traces and security policies; structured editors for tuner parameter spaces and workflow steps.
- Accessibility pass: skip link, focus management, modal focus trapping, table captions, text alternatives for
  the service graph, heat map and charts.
- Open tabs recover automatically after a redeploy.

### Changed
- Fixed struct-layout mismatches between the eBPF programs and the collector's Go decoders (implicit padding made every field after the gap misread; security and GPU enums disagreed).
- The `network-intelligence` chart now runs the collector with the flags it actually has (`ebpf.interface`, `ebpf.cgroupPath`, `ebpf.ncclLib`, `ebpf.cudaLib`); the old `--enable-program` list is gone.
- **Breaking for OIDC installs:** an OIDC token must now match a `GryviaTenant` (by `org` or `groups`); previously the claim was trusted as a namespace and every user could read cluster-wide data. Set `apiGateway.oidc.legacyNamespaces=true` only while no tenants exist.
- Quota enforcement rejects jobs whose GPU type is not allowed; GPU prices come from the SKU catalog (one shared default table replaces two hardcoded copies).
- The GPU operator no longer uses NVML: it runs without cgo on a distroless static image. It previously read GPU data inside its own pod, so it reported the wrong node and normally could not load the driver. `spec.drivers` is informational; nothing consumes it.
- The bundled NVIDIA device plugin and DCGM exporter DaemonSets only run on nodes labelled `nvidia.com/gpu.present=true` (configurable) and are skipped when the GPU Operator sub-chart is enabled. Previously they ran, and crashed, on every node.
- Docs no longer claim the Terraform/Ansible flow installs Kubernetes, Calico, Multus or GPU drivers; those directories are marked experimental and incomplete.
- **Breaking:** the API version is now `gryvia.io/v1alpha1` (it was `v1`), matching the project's alpha status. Objects created under `v1` are not converted: export them, change `apiVersion`, and re-apply. `scripts/rename-version.sh` documents the change; `deploy-remote.sh` recreates CRDs that still store `v1` when they hold no objects.
- **Breaking:** API kinds renamed from `Fabric*` to `Gryvia*` (for example `FabricAIJob` is now `GryviaAIJob`).
  Recreate existing objects under the new kinds; `scripts/rename-kinds.sh` documents the mapping.
- The dashboard follows netra's design system and defaults to the system light or dark setting.
- Status vocabulary unified across pages (`Succeeded` counts as completed, `Scheduling` as pending).

### Fixed
- The release rehearsal showed the eBPF collector image has never built (its kernel programs do not compile). It is no longer published and is off by default in the network-intelligence chart; documented as experimental.
- Costs are labelled as spend to date; empty states name the operator or collector that feeds the data instead
  of showing reassuring zeros.
- Rate limits count per client address instead of per proxy pod.

### Security
- Dashboard sessions are signed, expiring tokens issued by `/api/auth/login`; the API key is no longer stored in the browser.
- The dashboard bundle no longer contains a login credential; the gateway validates sign-in.
- The chart runs no root containers: the TLS Secret is mounted with `fsGroup` instead of a root init container.
- Login attempts are constant-time compared, delayed on failure and rate limited per client address.
