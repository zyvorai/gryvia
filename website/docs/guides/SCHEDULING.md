# Advanced Scheduling Guide

Complete guide to Gryvia's advanced scheduling capabilities for GPU workloads, including gang scheduling, fair-share queuing, elastic training, admission webhooks, and priority preemption.

## Table of Contents

1. [Gang Scheduling](#gang-scheduling)
2. [DRF Fair-Share Queue](#drf-fair-share-queue)
3. [Elastic Training](#elastic-training)
4. [Admission Webhooks](#admission-webhooks)
5. [Priority Preemption](#priority-preemption)

---

## Gang Scheduling

Atomic multi-pod placement guaranteeing that all pods for a distributed training job are scheduled simultaneously or not at all.

### Overview

Traditional Kubernetes scheduling places pods independently, which can cause distributed training jobs to deadlock when some pods are scheduled but others remain pending. Gang scheduling solves this by treating all pods in a job as a single scheduling unit.

Key properties:

- **Atomicity**: All pods are placed together or none are placed.
- **Deadlock Prevention**: Prevents scenarios where multiple jobs each hold partial resources and block each other.
- **Topology Awareness**: Prefers to co-locate pods on the same rack, switch, or NVLink domain for optimal communication performance.
- **Timeout**: If the scheduler cannot place all pods within the configured timeout, the entire gang is released to avoid resource starvation.

### Configuration

Gang scheduling is enabled per-job via the `scheduling` section:

```yaml
apiVersion: gryvia.io/v1
kind: GryviaAIJob
metadata:
  name: distributed-llm-training
spec:
  framework: pytorch
  distributed:
    enabled: true
    strategy: ddp
    nodes: 4
    gpusPerNode: 8

  resources:
    gpuType: H100
    gpuCount: 32

  scheduling:
    gang:
      enabled: true
      # Minimum number of pods required to start (optional)
      # Defaults to total pod count for strict gang semantics
      minMembers: 4
      # Timeout for gang assembly
      timeout: 10m
      # Topology preference
      topology:
        preferred: same-rack
        required: same-zone

  image: nvcr.io/nvidia/pytorch:24.01-py3
  command: ["torchrun", "--nproc_per_node=8", "train.py"]
```

### Topology Preferences

| Topology | Description | Use Case |
|----------|-------------|----------|
| `same-node` | All pods on a single node | Small jobs using NVLink |
| `same-rack` | Pods on nodes in the same rack | Jobs needing low-latency interconnect |
| `same-zone` | Pods in the same availability zone | Cross-rack but zone-local |
| `any` | No topology constraint | Maximum scheduling flexibility |

### How It Works

1. Job submitted with `gang.enabled: true` and 4 pods requested.
2. Scheduler reserves resources but does not bind pods until all 4 can be placed.
3. If all 4 can be placed with topology preferences satisfied, pods are bound atomically.
4. If the gang cannot be assembled within `timeout` (10m), reservations are released and the job is re-queued.
5. The scheduler periodically retries gang assembly using backoff.

---

## DRF Fair-Share Queue

Dominant Resource Fairness (DRF) multi-resource scheduling with hierarchical queues and backfill.

### Overview

DRF ensures fair resource allocation across teams by equalizing each team's dominant resource share. A team whose dominant resource is GPUs will be compared against teams whose dominant resource is CPU or memory, ensuring no single team monopolizes the cluster.

Features:

- **Multi-Resource Fairness**: Considers GPU, CPU, memory, and storage simultaneously.
- **Hierarchical Queues**: Team queues inherit from parent organization queues.
- **Backfill**: Small jobs can jump ahead in the queue if they fit in resource gaps without delaying higher-priority jobs.
- **Borrowing**: Teams can borrow unused quota from other teams (configurable).
- **Guaranteed Minimums**: Each queue has a guaranteed minimum resource allocation.

### Queue Configuration

```yaml
apiVersion: gryvia.io/v1
kind: GryviaQueue
metadata:
  name: ml-research-queue
spec:
  # Queue hierarchy
  parent: organization-queue

  # Team binding
  team: ml-research

  # Resource guarantees and limits
  resources:
    guaranteed:
      gpus: 32
      cpu: 256
      memory: 1Ti
    limit:
      gpus: 64
      cpu: 512
      memory: 2Ti

  # Borrowing policy
  borrowing:
    enabled: true
    maxBorrow:
      gpus: 16        # Can borrow up to 16 extra GPUs
    returnGracePeriod: 5m

  # Backfill configuration
  backfill:
    enabled: true
    maxJobDuration: 4h   # Only backfill jobs shorter than 4h
    maxGPUs: 8           # Max GPUs per backfill job

  # Scheduling weights (higher = more priority in fair share)
  weight: 10

  # Preemption configuration
  preemption:
    enabled: true
    withinQueue: true     # Can preempt within same queue
    crossQueue: false     # Cannot preempt other queues
```

### Queue Hierarchy Example

```
organization-queue (weight: 100, GPUs: 128)
├── ml-research-queue (weight: 40, GPUs: 64)
│   ├── cv-team-queue (weight: 15, GPUs: 24)
│   ├── nlp-team-queue (weight: 15, GPUs: 24)
│   └── rl-team-queue (weight: 10, GPUs: 16)
├── ml-production-queue (weight: 30, GPUs: 32)
└── ml-platform-queue (weight: 30, GPUs: 32)
```

Resources cascade down the hierarchy. Each child queue is guaranteed its share but can borrow from siblings when they have idle capacity.

### Backfill Scheduling

Backfill allows small, short-duration jobs to start immediately by filling resource gaps:

```
Time -->
+-----------+---+--+
| Large Job |   |  |   <- Backfill candidate fits in gap
+-----------+---+--+
| Running Job      |
+------------------+
        ^ Backfill job starts here
```

Requirements for backfill:
- Job must declare a `maxDuration` so the scheduler knows it will complete before the gap closes.
- Job must fit within the `backfill.maxGPUs` limit.
- Job must not delay any higher-priority queued job.

### CLI

```bash
# View queue status
gryvia queue

# View queue hierarchy
gryvia queue --tree

# View queue details
gryvia get queue ml-research-queue

# View fair-share allocations
gryvia queue --fair-share

# Drain a queue (no new jobs, existing finish)
gryvia queue drain ml-research-queue

# Resume a drained queue
gryvia queue resume ml-research-queue
```

---

## Elastic Training

Dynamic worker scaling for distributed training jobs using PyTorch Elastic (torchelastic) integration.

### Overview

Elastic training allows distributed training jobs to scale their worker count up or down without restarting. This enables:

- **Scale-Up**: Add workers when more GPUs become available, accelerating training.
- **Scale-Down**: Remove workers gracefully when GPUs are needed for higher-priority jobs, rather than killing the entire job.
- **Fault Tolerance**: Continue training with reduced workers when nodes fail, rather than failing the entire job.
- **Opportunistic Scaling**: Use spot/preemptible GPUs and automatically adjust when they are reclaimed.

### Configuration

```yaml
apiVersion: gryvia.io/v1
kind: GryviaAIJob
metadata:
  name: elastic-training
spec:
  framework: pytorch
  distributed:
    enabled: true
    strategy: ddp

  # Elastic scaling configuration
  elastic:
    enabled: true
    minWorkers: 2        # Minimum workers to continue training
    maxWorkers: 8        # Maximum workers when resources are available
    initialWorkers: 4    # Starting worker count

    # Scale-up policy
    scaleUp:
      trigger: gpuAvailable
      cooldown: 5m
      increment: 1

    # Scale-down policy
    scaleDown:
      trigger: preemption
      gracePeriod: 2m     # Time for checkpoint before removing worker

    # Checkpoint before scaling
    checkpointOnScale: true

  resources:
    gpuType: A100-80G
    gpuPerWorker: 1
    memoryPerWorker: 64Gi

  image: nvcr.io/nvidia/pytorch:24.01-py3
  command:
    - python
    - -m
    - torch.distributed.run
    - --rdzv_backend=etcd
    - --rdzv_endpoint=etcd:2379
    - --nnodes=2:8
    - --nproc_per_node=1
    - train.py

  checkpointing:
    enabled: true
    frequency: 5m
    path: /checkpoints
```

### How Elastic Scaling Works

1. Job starts with `initialWorkers` (4) workers.
2. If additional GPUs become available and the cluster has capacity, Gryvia adds workers up to `maxWorkers` (8).
3. The rendezvous backend (etcd) coordinates the new worker joining the training group.
4. If a worker is preempted or a node fails, training continues as long as at least `minWorkers` (2) workers remain.
5. Before removing a worker, the job checkpoints its state (if `checkpointOnScale` is true).
6. New workers load the latest checkpoint and join the ongoing training.

### Prerequisites

- PyTorch 1.10+ with torchelastic support
- etcd or C10d rendezvous backend deployed in the cluster
- Checkpointing enabled (required for state consistency during scaling)

### CLI

```bash
# View elastic job status
gryvia status elastic-training

# Manually scale workers
gryvia scale job elastic-training --workers 6

# View scaling events
gryvia get job elastic-training --events
```

---

## Admission Webhooks

Validating and mutating admission webhooks for job submission with automatic NCCL environment injection.

### Overview

Gryvia registers two admission webhooks that intercept GryviaAIJob creation:

1. **Validating Webhook**: Rejects invalid job specifications before they enter the system.
2. **Mutating Webhook**: Automatically injects optimal NCCL environment variables and other runtime configuration.

### Validation Rules

The validating webhook enforces the following rules:

| Rule | Description |
|------|-------------|
| Quota check | Reject if team has insufficient GPU quota |
| Budget check | Reject if team has exceeded budget limit |
| Resource limits | Reject if requested GPU count exceeds per-job maximum |
| Image allowlist | Reject if container image is not in the approved registry list |
| Priority authorization | Reject if user is not authorized for the requested priority level |
| GPU type availability | Reject if requested GPU type does not exist in the cluster |
| Storage validation | Reject if referenced PVCs do not exist |
| Network validation | Reject if referenced network policies are invalid |

### NCCL Environment Injection

The mutating webhook automatically injects optimal NCCL environment variables based on the job's GPU type, node topology, and interconnect:

```yaml
# Automatically injected for H100 jobs with InfiniBand
env:
  - name: NCCL_IB_DISABLE
    value: "0"
  - name: NCCL_IB_HCA
    value: "mlx5"
  - name: NCCL_IB_GID_INDEX
    value: "3"
  - name: NCCL_NET_GDR_LEVEL
    value: "5"
  - name: NCCL_SOCKET_IFNAME
    value: "eth0"
  - name: NCCL_DEBUG
    value: "WARN"
  - name: NCCL_ALGO
    value: "Ring,Tree"
  - name: NCCL_PROTO
    value: "Simple"

# Automatically injected for A100 jobs with NVLink
env:
  - name: NCCL_P2P_LEVEL
    value: "NVL"
  - name: NCCL_SHM_DISABLE
    value: "0"
  - name: NCCL_SOCKET_IFNAME
    value: "eth0"
  - name: NCCL_DEBUG
    value: "WARN"

# Automatically injected for RoCE (RDMA over Converged Ethernet)
env:
  - name: NCCL_IB_DISABLE
    value: "0"
  - name: NCCL_IB_ROCE_VERSION_NUM
    value: "2"
  - name: NCCL_IB_GID_INDEX
    value: "3"
  - name: NCCL_NET_GDR_LEVEL
    value: "2"
```

### Additional Mutations

Beyond NCCL, the mutating webhook also injects:

- **Shared memory**: Sets `/dev/shm` size based on GPU memory requirements.
- **CUDA settings**: Injects `CUDA_VISIBLE_DEVICES` based on allocated GPU indices.
- **Affinity rules**: Adds node affinity for the requested GPU type.
- **Tolerations**: Adds tolerations for GPU node taints.
- **Resource limits**: Sets precise CPU and memory limits based on GPU type defaults.

### Configuration

Webhook behavior is configured via the Gryvia operator ConfigMap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: gryvia-operator-config
  namespace: gryvia-system
data:
  admission.yaml: |
    validation:
      enabled: true
      quotaCheck: true
      budgetCheck: true
      maxGPUsPerJob: 256
      allowedRegistries:
        - nvcr.io
        - docker.io
        - ghcr.io
      imageAllowlist: true

    mutation:
      enabled: true
      ncclInjection: true
      shmSizeAuto: true
      affinityInjection: true
      tolerationInjection: true
```

---

## Priority Preemption

Multi-tier priority system with checkpoint-aware preemption and automatic requeueing.

### Overview

Gryvia extends the [Priority & Preemption](ADVANCED_FEATURES.md#priority--preemption) system with checkpoint-aware preemption that minimizes wasted GPU compute when higher-priority jobs arrive.

### Preemption Flow

```
1. High-priority job submitted
2. Scheduler determines cluster is at capacity
3. Scheduler identifies candidate victims:
   a. Lower priority than the incoming job
   b. Sufficient resources freed if preempted
   c. Prefer jobs with checkpointing enabled
   d. Prefer jobs closest to their next checkpoint
4. Victims are notified (SIGTERM)
5. Victims checkpoint state (up to gracePeriod)
6. Victims are terminated
7. High-priority job is scheduled
8. Preempted jobs are re-queued with their original priority
9. When resources become available, re-queued jobs resume from checkpoint
```

### Preemption Configuration

```yaml
apiVersion: gryvia.io/v1
kind: GryviaAIJob
metadata:
  name: critical-inference
spec:
  # Priority class
  priorityClassName: production

  # Preemption settings
  preemption:
    # Can this job preempt others?
    canPreempt: true
    # Can this job be preempted?
    canBePreempted: false

  # Required for safe preemption
  checkpointing:
    enabled: true
    frequency: 10m
    path: /checkpoints
    # Grace period for final checkpoint on preemption
    gracePeriod: 2m

  resources:
    gpuType: A100-80G
    gpuCount: 4
```

### Priority Classes

Gryvia defines seven priority classes with clear preemption rules:

| Priority Class | Value | Can Preempt | Can Be Preempted | Typical Use |
|---------------|-------|-------------|-------------------|-------------|
| `system-critical` | 1,000,000 | Yes (all) | No | System infrastructure |
| `production` | 100,000 | Yes (high and below) | No | Production serving |
| `high` | 10,000 | Yes (normal and below) | Yes (by production+) | Critical research deadlines |
| `normal` | 1,000 | No | Yes (by high+) | Standard training jobs |
| `low` | 100 | No | Yes (by normal+) | Batch processing |
| `best-effort` | 10 | No | Yes (by any) | Opportunistic workloads |
| `spot` | 50 | No | Yes (by any) | Spot/preemptible instances |

### Victim Selection Strategy

When multiple candidates exist for preemption, the scheduler uses this ranking:

1. **Lowest priority first**: Preempt the lowest-priority jobs first.
2. **Most recently started**: Among equal priority, prefer newer jobs (less wasted compute).
3. **Checkpoint-ready**: Prefer jobs that have recently checkpointed (less lost progress).
4. **Fewest resources**: Prefer preempting fewer jobs to meet the resource requirement.

### CLI

```bash
# View priority classes
gryvia get priorityclasses

# View preemption events
gryvia get events --type preemption

# View re-queued jobs
gryvia queue --requeued

# Check job preemption status
gryvia status my-job --preemption
```

---

## Scheduling Summary

| Feature | Purpose | Key Benefit |
|---------|---------|-------------|
| Gang Scheduling | Atomic multi-pod placement | Prevents deadlock in distributed training |
| DRF Fair-Share | Multi-resource fair allocation | Equitable cluster sharing across teams |
| Elastic Training | Dynamic worker scaling | Maximize GPU utilization, fault tolerance |
| Admission Webhooks | Validation + auto-configuration | Prevent errors, optimal NCCL settings |
| Priority Preemption | Checkpoint-aware preemption | Critical jobs start fast, minimal waste |

---

## Support

- **Documentation**: https://gryvia.io/docs
- **Issues**: https://github.com/zyvorai/gryvia/issues
- **Discussions**: https://github.com/zyvorai/gryvia/discussions
- **Slack**: #gryvia-users

---

*Gryvia - Enterprise GPU Infrastructure Management*
