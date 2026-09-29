# Gryvia Roadmap

Where Gryvia stands and what is still open. This page replaces an earlier month-by-month plan that carried invented dates, effort estimates, targets and budgets. There are no committed dates here. Anything under "Open work" is aspirational, not a promise.

The project is alpha: the API is `gryvia.io/v1alpha1` and can change. For what each release actually contains, the source of truth is [CHANGELOG.md](https://github.com/zyvorai/gryvia/blob/main/CHANGELOG.md). Nothing below has been verified on real GPU, RDMA or InfiniBand hardware unless stated; most checks are unit tests against fake clients, CI on kind or k3s without GPUs, and eBPF load tests on Linux.

## Status by area

### Implemented (with the caveats stated)

| Area | What exists | Caveat |
|------|-------------|--------|
| `GryviaAIJob` operator | Node selection, StatefulSet, headless Service and PVC per job, distributed-training environment variables, status phases, validating admission webhook | Node choice is recorded in status, pods are placed by the Kubernetes scheduler; no retries, priority handling or checkpoint handling in the job controller. See [Scheduling](SCHEDULING.md) |
| GPU operator | `GryviaGpuNode` registration from NVIDIA GPU feature discovery labels, `GryviaGpuMemoryOptimizer` | No NVML; per-GPU numbers come from the DCGM exporter. Unverified on hardware |
| GPU node preparation | Optional bundled NVIDIA GPU Operator, `scripts/install-k3s-gpu.sh` | CI runs it on k3s without a GPU; see [GPU nodes](GPU_NODES.md) |
| ai-operator extras | `GryviaCheckpointGuard`, `GryviaTrainingTimeMachine`, `GryviaLiveExperiment`, `GryviaTrainingProfiler`, `GryviaModelLineage` | Unit-tested; not run against real training jobs |
| Quota, tenants, metering | `GryviaQuota`, `GryviaTenant` (namespace `tenant-<name>`, ResourceQuota, LimitRange, optional NetworkPolicy), `GryviaUsageRecord`, `GryviaCostPredictor`, `GryviaGpuSku` catalog, estimate invoices | Estimates only, no payment processing. See [GPU as a service](GPU_AS_A_SERVICE.md) |
| Storage and network operators | `GryviaStorage`, `GryviaNetwork` controllers | Real VAST, Weka, DDN or RDMA/SR-IOV hardware not tested |
| Network intelligence | 10 kinds reconciled by the network-intelligence operator (separate chart) | See the eBPF row below |
| API gateway and dashboard | REST API under `/api/...`, API key and session login, OIDC for tenant users, Flight Recorder cluster view, CRUD for several kinds | OIDC not verified against a real identity provider. See [Auth and TLS](AUTH_AND_TLS.md) |
| CLI | Rust `gryvia` talking to the Kubernetes API through your kubeconfig | |
| Helm charts and releases | `helm/gryvia`, `helm/network-intelligence`, observability chart, tag-driven multi-arch image release signed with cosign | |
| Python and Go SDKs | Python REST client, Go Kubernetes client | Install from source |

### Experimental

- **eBPF collector.** 30 CO-RE programs. Compile and pass the kernel verifier on Linux 7.0 x86_64; arm64 is compile-only. Off by default, runs privileged with host networking, and its image is not part of the release images. GPU, NCCL, RDMA and GPUDirect Storage behaviour is unverified on hardware. Flight Recorder and fabric signals are node-local previews.
- **Fabric signal CRD (`gryviafabricsignals`).** CRD and collector endpoint exist; no controller fills the CRD, and the penalty function in the scheduler package is not called.
- **Terraform and Ansible automation** under the repository's infrastructure directories: marked experimental and incomplete; they do not install Kubernetes, a CNI or GPU drivers.
- **Scheduling library code.** Gang scheduling, DRF fair share and queue ordering, and elastic scaling exist as Go packages with tests, but the running job controller does not call them.

### CRDs and API only, no controller yet

These have CRDs, and some are creatable from the gateway and dashboard, but no operator reconciles them, so they do nothing: `GryviaAutoTuner`, `GryviaWorkflow`, `GryviaWorkspace`, `GryviaInferenceService`, `GryviaModelRegistry` (the gateway only patches `spec.stage`), `GryviaBudget`, `GryviaChargeback`, `GryviaSLA`, `GryviaPriority`, `GryviaAutoScaler`, `GryviaFederation`, `GryviaReservation`, `GryviaRetryPolicy`, `GryviaTemplate`, `GryviaHealthCheck`, `GryviaJobHook`, `GryviaDataset`, `GryviaBenchmark`, `GryviaAudit`, `GryviaDRTest`, `GryviaGPUSharingPolicy`, `GryviaMetric`, `GryviaQuotaPolicy`, `GryviaWorkspace`, the fabric-signal kind, and `GryviaGpuSku` (plain catalog data, which needs none). The [CRD reference](../reference/crds.md) lists every kind with its controller status.

### Not implemented

- Job retries, checkpoint-aware restarts and preemption in the job controller
- Priority queues, backfill, hierarchical fair share, Kueue integration
- Cilium as the default CNI or any cluster-wide CNI replacement
- Multi-cluster control plane and global scheduler
- Inference serving (Triton, vLLM, TensorRT-LLM) managed by Gryvia, with canary rollouts
- Budget alerts and enforcement, chargeback
- Automatic GPU health remediation
- Payment processing and billing

## Open work

Listed roughly in the order they would make the platform more useful; no dates.

1. **Verify on real hardware.** Run the operators, the eBPF collector and the RDMA paths on GPU and InfiniBand or RoCE nodes and record measured results. Until then storage, NCCL and training performance are unknown: the earlier targets (for example 20 GB/s storage, 30 percent faster training, 99.9 percent uptime) were goals, never measurements.
2. **Wire the scheduling library code** into the job controller (gang placement, fair share, the fabric penalty) and decide between building in-tree and integrating an existing queueing system.
3. **Reconcile the ML kinds**: register controllers for the tuner, workflow, model registry, inference service and workspace kinds, or remove the kinds that will not be built.
4. **Reliability of jobs**: retries, checkpoint resume, and handling of `Cancelled` and node failure in the job controller.
5. **Enforce cost controls**: budgets and alerts on top of the existing metering and estimates.
6. **Multi-cluster** and federation, if demand justifies it.
7. **Release quality**: publish the collector image once the eBPF programs are verified on more kernels and on arm64, add end-to-end tests that exercise GPUs, and broaden CI.

## Verification cadence

Ideas for measurements worth running regularly once hardware is available; none is currently automated: LLM training throughput, `nccl-tests` bandwidth, storage I/O with `fio`, job success rate, scheduler and API latency. No targets are set until a baseline exists.

## Contributing and feedback

- **Issues**: https://github.com/zyvorai/gryvia/issues
- **Discussions**: https://github.com/zyvorai/gryvia/discussions
