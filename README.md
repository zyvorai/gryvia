# Gryvia

> **GPU is the new CPU. Gryvia is its scheduler.**

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)](https://go.dev/)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-1.32+-326CE5?logo=kubernetes)](https://kubernetes.io/)
[![NVIDIA](https://img.shields.io/badge/NVIDIA-GPU-76B900?logo=nvidia)](https://nvidia.com)

**From cluster to fabric.** Gryvia turns your GPU infrastructure into a high-performance compute fabric — optimizing not just GPU allocation, but GPU-to-GPU communication. Like DGX SuperPOD, but open-source and self-hosted.

---

## TL;DR

Gryvia is a Kubernetes-native GPU platform that:

- Schedules AI workloads with **topology-aware GPU placement** and per-team quotas
- Aims to configure **RDMA + NVLink** for you, so less manual NCCL tuning
- Provisions **parallel-filesystem storage** per job
- Targets **bare metal**, and is designed to run on managed Kubernetes (EKS, GKE, AKS)

> **Status:** early and under active development. Performance figures in this repository are design targets, not measured results.

Built for teams training large models, not running containers.

---

## The Problem

Modern AI infrastructure is broken:

- **GPUs sit idle** — poor scheduling and fragmentation waste expensive GPU hours
- **Distributed training is slow** — network bottlenecks kill multi-node scaling efficiency
- **Storage can't keep up** — data loading becomes the training bottleneck at scale
- **Teams waste weeks** tuning NCCL, RDMA, drivers, and topology before training starts
- **No single platform** handles GPU scheduling, networking, storage, and observability together

## The Solution

Gryvia turns your infrastructure into a **high-performance GPU fabric**:

- **GPUs communicate at hardware speed** — automatic RDMA, NVLink, and NCCL optimization
- **Jobs are always optimally placed** — topology-aware scheduling with GPU affinity
- **Storage and network are automatically tuned** — parallel filesystems + SR-IOV out of the box
- **No manual cluster tuning required** — submit a job, get maximum performance

---

## Gryvia vs Vanilla Kubernetes

| Capability | Vanilla K8s + GPU Operator | Gryvia |
|--------|---------------------------|------------|
| **Multi-node distributed training setup** | Manual configuration | Automated by operators |
| **RDMA/NVLink tuning** | Manual | Configured by the GPU and network operators |
| **Job placement** | Default scheduler (first-fit) | Topology-aware scoring |
| **Storage for training** | Standard CSI | Parallel filesystem integrations (Weka, DDN, Lustre, CephFS) |
| **GPU failure handling** | Manual intervention | Health checks and automated remediation |
| **Cost tracking** | Not built-in | Per-team budgets and chargeback |

---

## 5-Minute Demo

```bash
# Deploy Gryvia
gryvia init --bare-metal

# Submit a distributed training job — that's it
gryvia create job llama-70b \
  --gpus 64 \
  --gpu-type H100 \
  --distributed \
  --nodes 8

# Watch it run
gryvia status llama-70b --follow
```

No NCCL tuning. No topology config. No storage provisioning. **It just works.**

---

## What Makes Gryvia Different?

| **Platform** | **Weakness** | **Gryvia Advantage** |
|---|---|---|
| **EKS/GKE/AKS** | Generic, cloud-only | GPU-first, multi-cloud + bare metal |
| **OpenShift AI** | Heavy, complex | Lightweight, purpose-built |
| **Databricks** | Expensive, closed | Open-source, self-hosted |
| **CoreWeave** | Cloud-only | On-prem + edge + multi-cloud |
| **DGX SuperPOD** | Proprietary, rigid | Open, flexible, Kubernetes-native |

**Stop managing nodes. Start managing compute fabric.**

---

## Features

### 1. **Gang Scheduling + Elastic Training — No Wasted GPUs**

Advanced scheduling that goes far beyond basic Kubernetes:
- **Gang scheduling** — all pods for a distributed job are placed atomically, or none are. No deadlocks
- **Fair-share queuing** — DRF (Dominant Resource Fairness) ensures no team starves another
- **Backfill scheduling** — small jobs fill gaps while large jobs wait
- **Elastic training** — running jobs scale up/down at runtime with PyTorch Elastic (torchrun)
- **Admission webhooks** — validate jobs at creation time, auto-inject NCCL env vars and SR-IOV annotations
- **Priority preemption** — high-priority jobs can evict lower-priority ones with grace period for checkpointing

---

### 2. **Never Waste a GPU Again — Deep NVIDIA Integration**

#### Native NVIDIA Stack
Built-in support for the complete NVIDIA ecosystem:
- **NVIDIA GPU Operator** - Automated lifecycle management
- **NVIDIA Container Toolkit** - Seamless GPU container integration
- **nvidia-smi auto-management** - Automatic monitoring and tuning
- **Automatic driver installation** - Zero-touch driver deployment
- **Kernel module tuning** - Performance optimization out of the box
- **MIG (Multi-Instance GPU) support** - Slice GPUs for multi-tenancy

**Example:** Register an H100 GPU node:
```yaml
apiVersion: gryvia.io/v1
kind: FabricGpuNode
spec:
  nodeName: gpu-worker-01
  gpuType: H100
  gpuCount: 8
  rdma: true
  sriov: true
```

#### 📊 **NVIDIA Observability (Enterprise Grade)**
Automatic integration with:
- **NVIDIA DCGM (Data Center GPU Manager)**
- **DCGM Exporter → Prometheus** - Real-time metrics
- Pre-built Grafana dashboards for:
  - GPU utilization & memory usage
  - Temperature & throttling events
  - Power draw & efficiency
  - NVLink bandwidth & topology
  - XID errors & health alerts

**Real-time GPU health monitoring out of the box** — no configuration needed.

#### 🧠 **AI-Aware GPU Scheduling**
Custom scheduler that understands:
- GPU model (A100 vs H100 vs L40)
- NUMA locality & CPU pinning
- NVLink topology & GPU-GPU affinity
- RDMA proximity & network bandwidth
- Storage latency & I/O patterns

**Result:** No "bad placements" that slow training — jobs always land on optimal nodes.

---

### 3. **Fix the #1 AI Training Bottleneck — GPU Communication**

#### Automatic RDMA + SR-IOV
Gryvia automatically:
- Configures **SR-IOV Virtual Functions (VFs)**
- Enables **RDMA for pod-to-pod traffic**
- Tunes MTU, buffer sizes, congestion control
- Supports **InfiniBand** and **RoCE v2**

**Perfect for:**
- Distributed training (PyTorch DDP, Horovod, Megatron)
- Large model sharding (FSDP, Tensor Parallelism)
- Multi-node LLM training

**Performance:**
- **200-400Gbps** bandwidth per node
- **<2µs** latency for GPU-GPU communication
- **Zero packet loss** with PFC/ECN tuning

#### 🛡️ **AI-Native Network Security**
- Cilium Network Policies with eBPF
- Zero-trust pod networking
- GPU pod isolation per team
- Microsegmentation between jobs

---

### 4. **Stop Waiting for Data — Ultra-Fast Storage Fabric**

Native support for parallel filesystems:
- **VAST Data** - NVMe-optimized, 40GB/s+ throughput
- **Weka** - Distributed parallel filesystem
- **DDN** - Enterprise HPC storage
- **Lustre** - Traditional parallel FS
- **CephFS** - High-performance mode

#### Features:
- **Auto-provision** per-job parallel filesystems
- **RDMA storage access** for parallel filesystems (throughput depends on your hardware and filesystem)

**Optimized for:**
- Training data loading (ImageNet, LAION-5B)
- Model checkpointing (every N steps)
- Distributed inference (model weights)

**One command gives you:**
```yaml
apiVersion: gryvia.io/v1
kind: FabricStorage
spec:
  backend: vast
  rdma: true
  throughput: "40GB/s"
```

---

### 5. **Submit a Job, Not a Cluster Config — AI Workload Automation**

#### Auto-Distributed Training
You submit:
```yaml
kind: FabricAIJob
spec:
  model: llama-70b
  gpus: 8
  distributed:
    enabled: true
    nodes: 2
    gpusPerNode: 8
```

Gryvia automatically:
- Creates multi-node StatefulSet topology
- Sets `WORLD_SIZE`, `MASTER_ADDR`, `MASTER_PORT` environment variables
- Configures NCCL for optimal communication (RDMA when available)
- Sets up:
  - Tensor parallelism
  - Data parallelism (DDP)
  - Pipeline parallelism
  - Automatic checkpointing
- Handles node failures & restarts

#### 🚀 **One-Click Inference Deployment**
Submit a model, get auto-deployed:
- **NVIDIA Triton Inference Server**
- **vLLM** (high-throughput LLM serving)
- **TensorRT-LLM** (optimized kernels)
- **TorchServe** (PyTorch serving)

**With:**
- Horizontal Pod Autoscaling (HPA)
- Load balancing across GPUs
- Low-latency request routing

---

### 6. **Your Jobs Keep Running — Self-Healing Platform**

If something breaks, Gryvia auto-recovers:

| Issue                  | Auto-Action             |
|------------------------|-------------------------|
| GPU crash              | Restart container       |
| Node failure           | Migrate job to new node |
| Slow storage           | Move workload           |
| Network issue          | Reassign SR-IOV         |
| Driver error           | Reload kernel module    |
| Temperature spike      | Throttle job            |

**Your jobs keep running without human intervention.**

---

### 7. **Know Where Every GPU Dollar Goes — Cost + Quota + Chargeback**

Built-in metering tracks:
- GPU hours per team/namespace
- Storage usage (TB/month)
- Network bandwidth consumed

**Example report:**
```
Team A: 1,200 H100 hours @ $2.50/hr = $3,000
Team B: 800 A100 hours @ $1.50/hr = $1,200
```

Perfect for:
- Internal chargebacks
- Budget enforcement
- Cost attribution

---

### 8. **Security & Compliance**

Security features:
- **OIDC/SSO authentication** — SAML, OAuth2, LDAP with PKCE flow
- **Multi-tenant isolation** — per-tenant namespaces, NetworkPolicies, ResourceQuotas
- **Secure boot** for GPU nodes
- **Encrypted storage** (at-rest & in-transit)
- **Workload isolation** (cgroups, namespaces)
- **RBAC + IAM** integration
- **Audit controller** — real-time compliance tracking with retention policies
- **SLA monitoring** — queue time tracking, breach detection, compliance reporting
- **SOC-2 friendly** design

---

### 9. **Run Anywhere, Same Experience — Multi-Cloud + Bare Metal**

Gryvia runs everywhere:
- **AWS** (EKS with GPU nodes)
- **GCP** (GKE with T4/A100)
- **Azure** (AKS with NDv4)
- **On-prem bare metal** (your own GPU servers)
- **Edge sites** (L40/T4 inference)

**Same experience, same APIs, everywhere.**

---

### 10. **Prove It Before Production — Built-in AI Benchmarks**

Pre-installed benchmarks to prove performance:
- **LLM throughput** (tokens/sec)
- **Training latency** (step time)
- **GPU stress tests** (burn-in)
- **Storage I/O** (dd, fio)
- **Network bandwidth** (iperf, ib_write_bw)

**Use these to validate your infra before production.**

---

### 11. **From Training to Serving — ML Workflow Engine**

Complete ML lifecycle management:
- **Hyperparameter tuning** — Grid, Random, Bayesian (TPE), ASHA early stopping with configurable parallelism
- **Workflow/DAG engine** — multi-step pipelines with dependencies, fan-out/fan-in, conditional execution
- **Model Registry** — version management with dev→staging→production promotion
- **Inference Service** — deploy models to Triton/vLLM/TensorRT-LLM with canary rollouts and auto-rollback
- **Workspaces** — managed Jupyter/VS Code environments with GPU allocation, idle timeout, pause/resume
- **Job templates** — parameterized, reusable job definitions with validation

---

### 12. **See Everything — Network Intelligence (NetPredator)**

eBPF-powered network visibility and control built on Cilium:
- **Service dependency graph** — real-time topology visualization with latency and throughput per edge
- **Traffic insights** — per-service p50/p95/p99 latency, top talkers, anomaly detection
- **Auto-policy generation** — learn traffic patterns, suggest network policies, enforce with approval
- **Trace sessions** — time-limited L3/L4/L7 network debugging
- **Anomaly detection** — statistical baseline deviation with auto-mitigation
- **eBPF collectors** — kernel-level TCP tracing, DNS tracking, syscall monitoring, latency probes

```bash
# Debug network issues in real-time
gryvia network trace payment-service --duration 2m --level l7

# Auto-generate security policies from observed traffic
gryvia network policy suggest --namespace production
```

---

## Architecture

```
┌──────────────────────────────────────────────────────────┐
│               Gryvia Control Plane                   │
│    (Multi-Cluster Management + Global Scheduling)        │
└─────────────────────┬────────────────────────────────────┘
                      │
      ┌───────────────┼────────────────┐
      │               │                │
┌─────▼─────┐   ┌────▼─────┐   ┌─────▼──────┐
│ Cluster A │   │ Cluster B│   │ Cluster C  │
│  H100 GPU │   │  A100 GPU│   │ L40 (Edge) │
│  InfiniBand│   │  RoCE v2 │   │  Standard  │
│  VAST Stor│   │  Weka    │   │  Local SSD │
└───────────┘   └──────────┘   └────────────┘
```

**Each cluster runs:**
1. GPU-aware scheduler with gang scheduling, DRF fair-share, and elastic scaling
2. 6 specialized operators (GPU, AI, Storage, Network, Quota, Network Intelligence)
3. eBPF flow collector + NVIDIA DCGM + Prometheus + Grafana observability
4. High-performance storage fabric (VAST/Weka/DDN)
5. Dark-themed Web UI with OIDC auth + REST API gateway
6. Rust CLI + Python SDK + Go SDK

---

## Quick Start (Bare Metal)

### Install on Physical GPU Servers

**Step 1: Configure your inventory**
```bash
cd terraform/bare-metal
cp terraform.tfvars.example terraform.tfvars
# Edit with your server IPs and GPU types
```

**Step 2: Deploy**
```bash
terraform init
terraform apply
cd generated && ./deploy.sh
```

**Step 3: Verify**
```bash
export KUBECONFIG=./generated/kubeconfig
kubectl get fabricgpunodes
```

**Step 4: Submit a job**
```bash
kubectl apply -f ../../examples/training/simple-pytorch-training.yaml
kubectl get fabricaijob
```

**Full deployment time:** 30-45 minutes (automated)

---

## Core Components

### Custom Resource Definitions (30+ CRDs)

**Core Resources:**

| CRD | Purpose |
|-----|---------|
| **FabricGpuNode** | GPU node registration, health, driver management |
| **FabricAIJob** | Training/inference job definition with GPU allocation |
| **FabricStorage** | Parallel filesystem (VAST/Weka/DDN) configuration |
| **FabricNetwork** | RDMA/SR-IOV network setup |
| **FabricQuota** | Per-team GPU limits and budgets |

**ML Workflow:**

| CRD | Purpose |
|-----|---------|
| **FabricAutoTuner** | Hyperparameter tuning with Grid/Random/Bayesian/ASHA |
| **FabricWorkflow** | DAG-based multi-step ML pipelines |
| **FabricModelRegistry** | Model versioning and promotion (dev→staging→prod) |
| **FabricInferenceService** | Model serving with canary and auto-rollback |
| **FabricWorkspace** | Managed Jupyter/VS Code GPU environments |
| **FabricTemplate** | Reusable parameterized job templates |

**Operations & Enterprise:**

| CRD | Purpose |
|-----|---------|
| **FabricAutoScaler** | GPU-aware cluster autoscaling |
| **FabricBudget** | Team budget tracking with burn rate forecasting |
| **FabricChargeback** | Per-team cost reports with configurable pricing |
| **FabricTenant** | Multi-tenant isolation with RBAC |
| **FabricSLA** | SLA monitoring and breach detection |
| **FabricAudit** | Compliance audit trail with retention policies |
| **FabricReservation** | Time-windowed GPU capacity reservations |
| **FabricPriority** | Priority classes with preemption policies |

**Network Intelligence:**

| CRD | Purpose |
|-----|---------|
| **FabricFlowPolicy** | Intent-based network policies (→ CiliumNetworkPolicy) |
| **FabricTrafficInsight** | Real-time per-service traffic analysis |
| **FabricAutoPolicy** | Self-healing firewall (learn → suggest → enforce) |
| **FabricTraceSession** | Time-limited network debugging sessions |
| **FabricServiceGraph** | Live service dependency graph |
| **FabricNetworkAnomaly** | Anomaly detection with auto-mitigation |

### Operators (6 Kubernetes Controllers)

1. **GPU Operator** — GPU lifecycle, drivers, health checks, benchmarks, GPU sharing (MIG/time-slice), custom metrics
2. **AI Workload Operator** — Scheduling (gang, elastic, fair-share), distributed training, HPO tuning, workflows, model registry, inference serving, workspaces, admission webhooks
3. **Storage Operator** — CSI drivers (VAST/Weka/DDN/Lustre/Ceph), dataset management with caching
4. **Network Operator** — SR-IOV, RDMA, Multus NetworkAttachmentDefinitions
5. **Quota Operator** — Quota enforcement, budgets, chargeback, SLA monitoring, audit, tenants, reservations
6. **Network Intelligence Operator** — eBPF-powered flow policies, traffic insights, auto-policy, trace sessions, service graph, anomaly detection

### Web UI

Dark-themed React dashboard with metallic design language:
- **Dashboard** — real-time cluster stats, GPU utilization charts, job pipeline view
- **Jobs** — submission wizard, monitoring, logs
- **Quotas & Costs** — team quota management, cost analysis
- **Nodes** — GPU node health and metrics
- **Network** — service graph visualization, flow monitoring, policy management
- **Login** — OIDC/SSO + API key authentication with PKCE flow
- API gateway with OIDC JWT validation, rate limiting, tenant-aware filtering

### CLI

Rust-based CLI (`gryvia`) for:
- Job management: submit, list, status, cancel, logs, queue monitoring
- Interactive wizards: `gryvia create job`, `gryvia create quota`
- Cluster overview with `--detailed` and `--watch`
- Network intelligence: `gryvia network trace`, `flows`, `graph`, `policy`, `anomalies`
- GPU, storage, and network health checks

### SDKs

**Python:**
```python
from gryvia import Gryvia

async with Gryvia(api_url="https://...", token="...") as tr:
    job = await tr.jobs.create({"spec": {"model": "llama-70b", "gpus": 8}})
    await tr.jobs.wait_for_completion(job["metadata"]["name"])
```

**Go:**
```go
client := sdk.NewGryviaClient(mgr.GetClient())
job, err := client.CreateJob(ctx, &v1.FabricAIJob{...})
```

### GPU-Aware Scheduler

Three-stage scheduling: filter, score, select.

**Filtering:** Eliminates nodes that are unhealthy, wrong GPU type, lack RDMA/SR-IOV, or have insufficient free GPUs (calculated from actual pod GPU usage across the cluster). When no nodes match, the error includes the exact requirements that failed (GPU type, count, network mode, nodes evaluated).

**Scoring algorithm:**
- GPU type match: +50 points
- RDMA availability: +30 points
- NVSwitch interconnect: +40 points
- Available GPU count: +5 points per free GPU
- GPU memory: +1 point per 10GB
- Node resources: memory and CPU capacity

**Topology optimizer** uses named scoring weights (NVLink 40%, NUMA 30%, utilization 20%) with full-node bonuses and fragmentation penalties.

**Result:** Optimal job placement every time.

---

## Performance Metrics

### Targets

No benchmark results are published yet. The figures below are design targets to
be validated with `benchmarks/suite.yaml` on real hardware before they are
quoted as results.

| Metric | Target |
|--------|--------|
| **Job scheduling latency** | < 500 ms (8-GPU job) |
| **Fault recovery time** | < 60 s (node failure) |
| **Concurrent queued jobs** | 10,000+ |

Storage and network throughput depend on the hardware, filesystem and fabric you deploy on.

---

## Documentation

- [Quick Start Guide](website/docs/getting-started/quickstart.md)
- [Bare Metal Deployment](website/docs/guides/DEPLOYMENT_GUIDE.md)
- [Complete Deployment Guide](website/docs/guides/COMPLETE_DEPLOYMENT_GUIDE.md)
- [CLI Guide](website/docs/guides/CLI_GUIDE.md)
- [API Reference](website/docs/developer-guide/api-reference.md)
- [Storage & Network Operators](website/docs/guides/STORAGE_NETWORK_OPERATORS.md)
- [Advanced Features](website/docs/guides/ADVANCED_FEATURES.md)
- [Examples & Tutorials](examples/README.md)
- [FAQ](website/docs/guides/FAQ.md)

## Security

Gryvia follows security best practices:
- **OIDC/SSO authentication** - JWT validation with JWKS caching, PKCE flow, tenant-aware filtering
- **Multi-tenant isolation** - Per-tenant namespaces, NetworkPolicies, ResourceQuotas via FabricTenant CRD
- **Least-privilege RBAC** - Operators have scoped ClusterRoles per CRD
- **Non-root containers** - All operator pods run as UID 65532 with read-only root filesystem
- **No privileged containers** - GPU device plugins use targeted capabilities instead of blanket `privileged: true`
- **Network policies** - Default-deny ingress; auto-generated policies from observed traffic (FabricAutoPolicy)
- **Admission webhooks** - Validate and mutate FabricAIJob resources at creation time
- **Audit trail** - FabricAudit controller captures all GPU resource events with configurable retention
- **Input validation** - Budget pricing rejects negative/NaN/Inf rates; CLI enforces DNS-1123 job names; quota enforcement is atomic
- **CRD validation** - Required fields, enum constraints, min/max validation on all custom resources
- **Image pinning** - All container images use specific version tags, never `:latest`

---

## Real-World Use Cases

### Large Language Model Training
```yaml
apiVersion: gryvia.io/v1
kind: FabricAIJob
spec:
  model: llama-70b
  gpus: 8
  gpuType: H100
  network: rdma
  storage: vast-fast
  distributed:
    enabled: true
    framework: pytorch
    backend: nccl
    nodes: 8          # 8 nodes
    gpusPerNode: 8    # 8 GPUs each = 64 total
```

### Multi-Tenant GPU Sharing
```yaml
apiVersion: gryvia.io/v1
kind: FabricQuota
spec:
  team: data-science
  namespaces: [ds-training, ds-inference]
  gpuQuota:
    maxGPUs: 16
    allowedGPUTypes: [A100, L40]
  budget:
    monthlyBudget: 10000.00  # USD
```

### Inference at Scale
```yaml
apiVersion: gryvia.io/v1
kind: FabricAIJob
spec:
  type: inference
  model: stable-diffusion-xl
  gpus: 4
  replicas: 10  # 40 GPUs total
```

---

## Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md).

---

## License

Apache License 2.0 - see [LICENSE](LICENSE).

---

## Why Teams Choose Gryvia

1. **GPU Utilization** — topology-aware, quota-aware scheduling to reduce idle and fragmented GPUs
2. **Bare Metal Performance** — No cloud overhead, full hardware access
3. **Cost Visibility** — per-team budgets and chargeback
4. **Flexibility** — Run anywhere (on-prem, cloud, edge)
5. **Built for platform teams** — Kubernetes-native operators and CRDs
6. **Open Source** — No vendor lock-in, full visibility

---

**AI infra without bottlenecks.**

[Get Started](website/docs/getting-started/quickstart.md) | [View Examples](examples/) | [Documentation](website/docs/intro.md)
