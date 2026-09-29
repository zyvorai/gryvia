# Scheduling Guide

What Gryvia does today when a `GryviaAIJob` is submitted, and which of the more advanced scheduling ideas (gang scheduling, DRF fair share, backfill, elastic training, preemption) exist only as library code or design.

## Status at a glance

| Capability | Status |
|------------|--------|
| GPU-aware node selection (filter and score nodes, recorded in `status.nodesAllocated`) | Implemented and run by the GryviaAIJob controller. Advisory: see [What runs today](#what-runs-today) |
| StatefulSet, headless Service and PVC creation, NCCL/`MASTER_ADDR`/`WORLD_SIZE` env for distributed jobs | Implemented |
| Validating admission webhook (job sanity, quota and SKU policy) | Implemented, served by the ai-operator when enabled; fails open by default |
| Gang scheduling | Library code in `operators/ai-operator/pkg/scheduler/gang.go`, not called by the controller |
| DRF fair share and queue ordering | Library code in `operators/ai-operator/pkg/queue`, not called by the controller |
| Backfill | Not implemented as a running behaviour |
| Elastic training | Library code in `operators/ai-operator/pkg/elastic`, not called by the controller |
| Mutating webhook (NCCL injection and defaults) | Code exists in `pkg/webhook/mutator.go`; not registered in `main.go`, so it does not run |
| Priority preemption, `GryviaPriority` classes | CRD only, no controller |
| Fabric-health penalty on node scores | Function exists (`pkg/scheduler/fabric_score.go`), marked NOT WIRED in the code |

The remaining sections describe the running behaviour first, then the designs. Sections marked "Design" are not what a cluster does today. Nothing here has been verified on real GPU hardware; the operator logic is covered by Go unit tests against fake clients.

## What runs today

The GryviaAIJob controller (`operators/ai-operator/controllers/gryviaaijob_controller.go`) reconciles each job through these steps:

1. New jobs get `status.phase: Pending`.
2. **Node selection.** `scheduler.FindOptimalNodes` lists nodes and GPU-consuming pods, then filters and scores nodes:
   - Filters: node is Ready; `gryvia.io/gpu` label matches `spec.gpuType` (unless empty or `any`); `gryvia.io/rdma=true` when `spec.network: rdma`; `gryvia.io/sriov=true` when `spec.network: sriov`; `spec.nodeSelector` matches; enough free GPUs (from the `gryvia.io/gpu-count` label or `nvidia.com/gpu` allocatable, minus GPUs requested by running pods).
   - Score: +50 for a GPU-type match, +30 for RDMA when requested, +40 (NVSwitch) or +30 (NVLink) from the `gryvia.io/interconnect` label for multi-GPU jobs, +5 per free GPU, +1 per 10 GB of `gryvia.io/gpu-memory`, plus a small general node-resource term.
   - The top `distributed.nodes` nodes (1 when not distributed) are written to `status.nodesAllocated`. If no node qualifies, the `Scheduled` condition is set to false with the reason and the job is retried after 30 seconds.
3. Creates a PVC when `spec.storage` is set, a headless Service, and a StatefulSet named `<job>-training` with one `trainer` container.
4. Derives status from the StatefulSet and pods: `Running` once a replica is ready, `Succeeded` when all pods have succeeded, `Failed` when all pods have terminated and at least one failed.

Important limits of the current implementation:

- `status.nodesAllocated` is **not** turned into a node binding. The StatefulSet's pod template carries `nodeSelector` (`gryvia.io/gpu`, `gryvia.io/rdma`, plus your own), tolerations and affinity, and the Kubernetes scheduler places the pods. The operator's node choice is recorded, not enforced. In effect the operator's step is a feasibility check plus advisory status: it fails (job stays Pending, retried every 30 seconds) when no node qualifies, but it does not decide where pods run. The `gryvia.io/gpu`, `gryvia.io/gpu-count`, `gryvia.io/rdma`, `gryvia.io/sriov` and `gryvia.io/interconnect` node labels it relies on are set by the `GryviaGpuNode` controller (from the resource's spec and GPU feature discovery data).
- Pods request `nvidia.com/gpu` (per pod: `distributed.gpusPerNode` when distributed, otherwise `spec.gpus`), so the NVIDIA device plugin is required.
- `spec.priority`, `spec.retryLimit`, `spec.timeout` and `spec.model` are accepted by the schema (priority is range-checked 0 to 100 by the webhook) but the controller does not act on them today.
- A StatefulSet is used, so pods restart in place (`restartPolicy: Always` is the only choice for StatefulSets); run-to-completion semantics are approximated by inspecting pod phases.

### A distributed job that validates against the CRD

```yaml
apiVersion: gryvia.io/v1alpha1
kind: GryviaAIJob
metadata:
  name: distributed-llm-training
spec:
  type: training
  image: nvcr.io/nvidia/pytorch:24.01-py3
  gpus: 8
  gpuType: H100
  network: rdma
  distributed:
    enabled: true
    framework: pytorch
    backend: nccl
    nodes: 4
    gpusPerNode: 8
  command: ["torchrun", "--nproc_per_node=8", "train.py"]
```

For a distributed job the controller creates 4 replicas with 8 GPUs each and injects `MASTER_ADDR`, `MASTER_PORT=29500`, `WORLD_SIZE` (nodes times GPUs per node) and `NCCL_DEBUG=INFO`; with `network: rdma` it also sets `NCCL_IB_DISABLE=0` and `NCCL_NET_GDR_LEVEL=5`, adds the `gryvia.io/rdma` annotation and the `rdma-network` network attachment annotation, and mounts a memory-backed `/dev/shm`. RDMA behaviour has not been verified on hardware.

---

## Gang Scheduling

Status: library code only. `GangScheduler` in `operators/ai-operator/pkg/scheduler/gang.go` implements a `PodGroup` with all-or-nothing reservation of GPUs across nodes, a hold timeout (a constant of 2 minutes in the code) that releases reserved GPUs to break deadlocks, and a deadlock detector. Nothing in the running controller calls it, and there is no `spec.scheduling` field on `GryviaAIJob`.

What that means today: a multi-node job becomes one StatefulSet (default `OrderedReady` pod management, so pods start one after another) and the Kubernetes scheduler places each pod independently. Nothing reserves the whole set of GPUs up front, so two large jobs can each end up holding part of the cluster. If you need real gang semantics now, use a gang-aware scheduler such as Kueue or Volcano in front of the cluster; Gryvia does not integrate with them.

### Design sketch

Design sketch, not accepted by the current CRD schema (there is no `scheduling` field):

```text
spec:
  scheduling:
    gang:
      enabled: true
      minMembers: 4
      timeout: 10m
      topology:
        preferred: same-rack
```

The intended behaviour is: reserve resources without binding until every pod of the gang can be placed, then bind atomically; on timeout release the reservations and re-queue. Topology preference values (`same-node`, `same-rack`, `same-zone`, `any`) are not implemented; the only topology signal in the real scorer is the `gryvia.io/interconnect` node label (NVSwitch, NVLink).

---

## DRF Fair-Share Queue

Status: library code only. `operators/ai-operator/pkg/queue` contains a priority queue (`controller.go`) and a Dominant Resource Fairness calculator (`fairshare.go`: register teams with weights, record allocations, pick the team with the smallest weighted dominant share). The controller does not use either, so there is no fair-share ordering of jobs, no hierarchical queues and no backfill running. There is no `GryviaQueue` kind.

What exists for multi-tenant limits instead:

- `GryviaQuota` (reconciled by the quota-operator) tracks usage per namespace, and the admission webhook enforces per-job GPU limits and allowed GPU types from quotas and the tenant's SKU list on create.
- Kubernetes `ResourceQuota` and namespaces per tenant work as usual.

### The CLI queue view

`gryvia queue` lists jobs whose phase is Pending, Queued or Scheduling, plus a summary of running jobs. It reads `GryviaAIJob` objects through your kubeconfig; the optional name argument filters by job name substring, it is not a queue name.

```bash
gryvia queue
gryvia queue --watch 5
gryvia queue -o json
```

### Design sketch

Design sketch, not accepted by the current CRD schema (no `GryviaQueue` kind exists):

```text
kind: GryviaQueue
spec:
  parent: organization-queue
  team: ml-research
  resources:
    guaranteed: {gpus: 32}
    limit: {gpus: 64}
  borrowing: {enabled: true}
  backfill: {enabled: true, maxJobDuration: 4h}
  weight: 10
```

Intended semantics: weighted DRF ordering across teams, guaranteed minimums with borrowing of idle capacity, and backfill of short small jobs into gaps that do not delay higher-priority jobs. Backfill would also need a way to know a job's maximum duration; `spec.timeout` exists in the schema but is not read by any scheduler today.

---

## Elastic Training

Status: library code only, with no schema field to enable it. `operators/ai-operator/pkg/elastic` has helpers for min/max node annotations and scaling a StatefulSet, but the GryviaAIJob controller never calls them, and `GryviaAIJob` has no `elastic` field. A job's replica count is fixed at `distributed.nodes`.

You can still run a torchelastic-style job by putting the elastic launcher flags in `command` (for example `--nnodes=2:8` with a c10d rendezvous); that is entirely the job's own behaviour, and Gryvia will not add or remove workers.

### Design sketch

Design sketch, not accepted by the current CRD schema:

```text
spec:
  elastic:
    enabled: true
    minWorkers: 2
    maxWorkers: 8
    checkpointOnScale: true
```

Intended behaviour: scale workers up when GPUs are free, scale down instead of killing the job when GPUs are needed elsewhere, and checkpoint before removing a worker. This would need the checkpoint machinery (see `GryviaCheckpointGuard` in the ML workflows guide) and a rendezvous backend in the cluster.

---

## Admission Webhooks

The ai-operator serves a **validating** webhook for `GryviaAIJob` when started with `--enable-webhooks` (the Helm chart's `webhook.enabled`, which defaults to true, with a self-signed certificate). The chart's `webhook.failurePolicy` defaults to `Ignore`, so if the webhook is unreachable job creation still succeeds (fail open). Set it to `Fail` to reject jobs while the webhook is down.

### Validation rules that exist

| Rule | Behaviour |
|------|-----------|
| `spec.type` | Must be one of `training`, `inference`, `fine-tuning`, `evaluation` |
| `spec.gpus` | Must be greater than 0 |
| `spec.network` | If set, one of `standard`, `rdma`, `sriov` |
| `spec.priority` | Between 0 and 100 |
| `spec.distributed` | When enabled: `nodes` > 0, `gpusPerNode` not negative, `framework` one of pytorch, tensorflow, horovod, deepspeed, megatron, `backend` one of nccl, gloo, mpi, and nodes times GPUs per node at most 1024 |
| Resource requests, image | Basic well-formedness checks |
| Quota and tenant policy (create only) | Denies GPU types not allowed by the namespace's `GryviaQuota` objects, GPU counts above the smallest per-job limit, and GPU types outside the tenant's allowed SKUs (from the `GryviaGpuSku` catalog) |
| Cluster warnings | Non-blocking warnings when the request exceeds the largest node or names a GPU type no node carries |

Rules that earlier versions of this guide listed but that are **not** implemented: budget check, image allowlist, priority authorization, PVC existence, network policy validation. There is no `gryvia-operator-config` ConfigMap driving webhook behaviour.

### Mutating webhook

`operators/ai-operator/pkg/webhook/mutator.go` contains a mutator that would inject NCCL variables (defaults such as `NCCL_DEBUG=WARN`, `NCCL_IB_DISABLE`, `NCCL_NET_GDR_LEVEL`, `NCCL_SOCKET_IFNAME`), topology and RDMA/SR-IOV annotations, default CPU and memory limits and a default image pull policy. It is not registered in the operator's `main.go` and no `MutatingWebhookConfiguration` is shipped, so none of these mutations happen. The only NCCL/distributed environment that is set today is the small set the controller itself adds (see above).

---

## Priority Preemption

Status: not implemented. `spec.priority` (0 to 100) is validated but the controller neither orders jobs by it nor preempts anything. A `GryviaPriority` CRD exists (priority classes such as production or best-effort in the design), and a controller file exists under `operators/ai-operator/controllers/`, but it is not registered in `main.go`, so nothing reconciles it. There is no `priorityClassName`, `preemption` or `checkpointing` field on `GryviaAIJob`.

For preemption today, use Kubernetes `PriorityClass` and the default scheduler's preemption on the pods. That requires a `priorityClassName` on the pod template, which `GryviaAIJob` does not expose either, so in practice it is not reachable through this API yet.

### Design

The intended flow is: a high-priority job arrives at a full cluster, the scheduler picks lower-priority victims (preferring jobs that checkpoint, lowest priority first), victims get SIGTERM and a grace period to checkpoint, then are re-queued and resume from the checkpoint. None of this exists in running code. The priority class table (values, who can preempt whom) in earlier versions of this guide was a proposal, not shipped behaviour, and has been removed.

```bash
# The CRD exists; nothing reconciles it yet
kubectl get gryviapriorities

# Real commands
gryvia queue
gryvia status my-job
kubectl describe gryviaaijob my-job
```

---

## Scheduling Summary

| Feature | Real state |
|---------|-----------|
| Node selection | Runs; recorded in status, pods are placed by the Kubernetes scheduler using selectors |
| Gang scheduling | Library code, not wired |
| DRF fair share, hierarchical queues, backfill | Library code (DRF and queue), backfill absent; not wired |
| Elastic training | Library code, not wired, no CRD field |
| Validating webhook | Runs when enabled; quota and SKU policy; fails open by default |
| Mutating webhook (NCCL injection) | Code only, not registered |
| Priority preemption | Not implemented |

See also [GPU nodes](GPU_NODES.md), [GPU as a service](GPU_AS_A_SERVICE.md) for quota and tenancy, and [Operations](OPERATIONS.md).
