# Gryvia Roadmap

Where Gryvia stands and what is still open. This page replaces an earlier month-by-month plan that carried invented dates, effort estimates, targets and budgets. There are no committed dates here. Anything under "Open work" is aspirational, not a promise.

The project is alpha: the API is `gryvia.io/v1alpha1` and can change. For what each release actually contains, the source of truth is [CHANGELOG.md](https://github.com/zyvorai/gryvia/blob/main/CHANGELOG.md). Nothing below has been verified on real GPU, RDMA or InfiniBand hardware unless stated; most checks are unit tests against fake clients, CI on kind or k3s without GPUs, and eBPF load tests on Linux.

## Status by area

### Implemented (with the caveats stated)

| Area | What exists | Caveat |
|------|-------------|--------|
| `GryviaAIJob` operator | Node selection, run-to-completion Indexed Job (training, fine-tuning, evaluation) or StatefulSet (inference), headless Service and PVC per job, distributed-training environment variables, status phases including `Succeeded`/`Failed`, `retryLimit` and `timeout`, cancel, validating admission webhook, opt-in quota/budget admission gate | Node choice is recorded in status, pods are placed by the Kubernetes scheduler; no checkpoint handling in the job controller. Kind e2e (`e2e-jobs.yml`) authored, not yet run; nothing on GPUs. See [Scheduling](SCHEDULING.md) and [AIJob lifecycle](https://github.com/zyvorai/gryvia/blob/main/docs/aijob-lifecycle.md) |
| ML controllers | `GryviaWorkspace`, `GryviaInferenceService`, `GryviaModelRegistry`, `GryviaWorkflow`, `GryviaAutoTuner` in the ai-operator | Unit-tested; kind e2e with tiny CPU images authored, not yet run; no GPU, real model server or real metrics. See [ML Workflows](ML_WORKFLOWS.md) |
| GPU operator | `GryviaGpuNode` registration from NVIDIA GPU feature discovery labels, `GryviaGpuMemoryOptimizer` | No NVML; per-GPU numbers come from the DCGM exporter. Unverified on hardware |
| GPU node preparation | Optional bundled NVIDIA GPU Operator, opt-in Network Operator and NIM Operator sub-charts ([NVIDIA one-click](NVIDIA_ONE_CLICK.md)), `scripts/install-k3s-gpu.sh` | CI runs it on k3s without a GPU; see [GPU nodes](GPU_NODES.md) |
| ai-operator extras | `GryviaCheckpointGuard`, `GryviaTrainingTimeMachine`, `GryviaLiveExperiment`, `GryviaTrainingProfiler`, `GryviaModelLineage` | Unit-tested; not run against real training jobs |
| Quota, tenants, metering | `GryviaQuota`, `GryviaTenant` (namespace `tenant-<name>`, ResourceQuota, LimitRange, optional NetworkPolicy, opt-in RoleBindings), `GryviaUsageRecord`, `GryviaCostPredictor`, `GryviaGpuSku` catalog, `GryviaBudget` controller, opt-in `GryviaReservation` node reservations, estimate invoices and a signed invoice webhook | Estimates only, no payment processing; reservations, tenant RBAC and the admission gate are off by default and unverified on a real cluster. See [GPU as a service](GPU_AS_A_SERVICE.md) |
| Storage and network operators | `GryviaStorage`, `GryviaNetwork` controllers | Real VAST, Weka, DDN or RDMA/SR-IOV hardware not tested |
| Network intelligence | 10 kinds reconciled by the network-intelligence operator (separate chart) | See the eBPF row below |
| API gateway and dashboard | REST API under `/api/...`, API key and session login, OIDC for tenant users, Flight Recorder cluster view, CRUD for several kinds | OIDC not verified against a real identity provider. See [Auth and TLS](AUTH_AND_TLS.md) |
| CLI | Rust `gryvia` talking to the Kubernetes API through your kubeconfig | |
| Helm charts and releases | `helm/gryvia`, `helm/network-intelligence`, observability chart, tag-driven multi-arch image release signed with cosign | |
| Python and Go SDKs | Python REST client, Go Kubernetes client | Install from source |

### Experimental

- **eBPF collector.** 47 CO-RE programs. Compile and pass the kernel verifier on Linux 7.0 x86_64 and in CI; arm64 is compile-only. XDP chaining (`-xdp-mux`) is tested in CI on the loopback interface only. Off by default, runs privileged with host networking, and its image is not part of the release images. GPU, NCCL, RDMA and GPUDirect Storage behaviour is unverified on hardware. Flight Recorder and fabric signals are node-local previews.
- **Fabric signal CRD (`gryviafabricsignals`).** CRD and collector endpoint exist; the collector can patch its status (`-publish-fabric-status`, opt-in) and, with `-fabric-status-per-node` and the ai-operator's `--merge-fabric-signals`, per-node entries are folded into the top-level status. The penalty function in the scheduler package is used only by the opt-in fabric-aware scheduling. None of it has run on real GPU or RDMA hardware.
- **Terraform and Ansible automation** under the repository's infrastructure directories: marked experimental and incomplete; they do not install Kubernetes, a CNI or GPU drivers.
- **Scheduling library code.** The in-tree gang scheduler (`pkg/scheduler/gang.go`), the DRF queue (`pkg/queue`) and elastic scaling exist as Go packages, but nothing calls them. Gang admission, queueing and preemption are done by Kueue instead, opt-in ([Kueue integration](https://github.com/zyvorai/gryvia/blob/main/docs/kueue-integration.md)): unit-tested with fake clients, kind workflow written but not yet run, never run on GPUs.

### CRDs and API only, no controller yet

These have CRDs, and some are creatable from the gateway and dashboard, but no operator reconciles them, so they do nothing: `GryviaChargeback`, `GryviaSLA`, `GryviaPriority`, `GryviaAutoScaler`, `GryviaFederation`, `GryviaRetryPolicy`, `GryviaTemplate`, `GryviaHealthCheck`, `GryviaJobHook`, `GryviaDataset`, `GryviaBenchmark`, `GryviaAudit`, `GryviaDRTest`, `GryviaGPUSharingPolicy`, `GryviaMetric`, `GryviaQuotaPolicy`, and `GryviaGpuSku` (plain catalog data, which needs none). The [CRD reference](../reference/crds.md) lists every kind with its controller status.

### Not implemented

- Checkpoint-aware restarts and checkpoint-before-preemption in the job controller (job retries exist for batch Jobs; preemption exists only through the opt-in Kueue integration and does not checkpoint)
- Backfill, hierarchical fair share, DRF and Kueue's fair-sharing modes (the Kueue integration provides queues, quota, cohort borrowing and priority preemption only, opt-in and unverified on a real cluster until its e2e passes), Kueue topology-aware scheduling and MultiKueue
- Cilium as the default CNI or any cluster-wide CNI replacement
- Multi-cluster control plane and global scheduler
- Verified inference serving (Triton, vLLM, TensorRT-LLM) managed by Gryvia: the controller exists, with a pod-count canary, but opt-in Gateway API weighted routing, custom GPU/RPS HPA metrics and Prometheus SLO gating of canaries are implemented; real-image/data-plane behaviour is not verified (see [inference serving](https://github.com/zyvorai/gryvia/blob/main/docs/inference-serving.md))
- Budget notifications, chargeback
- Automatic GPU health remediation
- Payment processing and billing

## Open work

Listed roughly in the order they would make the platform more useful; no dates.

1. **Verify on real hardware.** Run the operators, the eBPF collector and the RDMA paths on GPU and InfiniBand or RoCE nodes and record measured results. Until then storage, NCCL and training performance are unknown: the earlier targets (for example 20 GB/s storage, 30 percent faster training, 99.9 percent uptime) were goals, never measurements.
2. **Prove the Kueue integration** (run the kind workflow, then a GPU cluster) and wire what is still library code (the fabric penalty; topology-aware placement; elastic training). The decision to integrate Kueue rather than build queueing in-tree is made.
3. **Prove the ML controllers** (run `e2e-ml.yml`, then real Jupyter, vLLM and Triton images on GPUs) and decide the kinds that have no controller: build them or remove them.
4. **Reliability of jobs**: checkpoint resume and checkpoint-before-preemption (retries, `Cancelled` and eviction handling exist for batch Jobs).
5. **Cost controls**: budget notifications and chargeback on top of the opt-in admission gate and the `GryviaBudget` controller (verify both on a real cluster first).
6. **Multi-cluster** and federation, if demand justifies it.
7. **Release quality**: publish the collector image once the eBPF programs are verified on more kernels and on arm64, add end-to-end tests that exercise GPUs, and broaden CI.

## Verification cadence

Ideas for measurements worth running regularly once hardware is available; none is currently automated: LLM training throughput, `nccl-tests` bandwidth, storage I/O with `fio`, job success rate, scheduler and API latency. No targets are set until a baseline exists.

## Contributing and feedback

- **Issues**: https://github.com/zyvorai/gryvia/issues
- **Discussions**: https://github.com/zyvorai/gryvia/discussions
