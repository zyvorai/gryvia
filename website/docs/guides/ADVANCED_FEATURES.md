# Advanced Features Guide

Complete guide to Gryvia's advanced capabilities for enterprise GPU infrastructure management.

## Table of Contents

1. [GPU Health Monitoring](#gpu-health-monitoring)
2. [Advanced Retry Policies](#advanced-retry-policies)
3. [Resource Reservations](#resource-reservations)
4. [Multi-Tenancy](#multi-tenancy)
5. [Job Templates](#job-templates)
6. [Auto-Scaling](#auto-scaling)
7. [Budget Management](#budget-management)
8. [Priority & Preemption](#priority--preemption)
9. [ML Workflows](#ml-workflows)
10. [Network Intelligence](#network-intelligence)
11. [Advanced Scheduling](#advanced-scheduling)
12. [OIDC/SSO Authentication](#oidcsso-authentication)
13. [Python and Go SDKs](#python-and-go-sdks)

---

## GPU Health Monitoring

Proactive health monitoring and diagnostics for GPU infrastructure.

### Features

- **Automated Health Checks**: Scheduled GPU diagnostics
- **Multiple Check Types**: Temperature, ECC errors, NVLink, PCIe bandwidth
- **Auto-Remediation**: Automatic GPU reset, driver reload, node reboot
- **Alerting**: Email, Slack, webhook notifications
- **Cordon/Drain**: Automatic node isolation on failure

### Quick Start

```yaml
apiVersion: gryvia.io/v1alpha1
kind: GryviaHealthCheck
metadata:
  name: cluster-gpu-health
spec:
  target:
    type: cluster

  schedule: "*/5 * * * *"  # Every 5 minutes

  checks:
    - name: gpu-temperature
      type: gpu-temperature
      threshold:
        max: 90
        warning: 80

    - name: ecc-errors
      type: gpu-ecc-errors
      threshold:
        max: 0

  onFailure:
    cordon: true
    alert: true
    autoRemediate: true
```

### CLI Commands

```bash
# Check cluster health
gryvia health

# Check only GPU health
gryvia health gpu

# Inspect one GPU node
gryvia get node gpu-node-05

# Manual remediation: cordon and drain the node, then reset the GPU on the host
gryvia maintenance start gpu-node-05 --reason gpu-reset --drain

# Return the node to service
gryvia maintenance end gpu-node-05

# View the health check status and recent results
kubectl describe gryviahealthcheck cluster-gpu-health
```

### Health Check Types

| Check Type | Description | Thresholds |
|------------|-------------|------------|
| gpu-utilization | GPU compute usage | 0-100% |
| gpu-memory | GPU memory usage | 0-100% |
| gpu-temperature | GPU temperature | °C |
| gpu-ecc-errors | ECC error count | 0 = healthy |
| gpu-nvlink | NVLink connectivity | Links up |
| pcie-bandwidth | PCIe throughput | GB/s |
| gpu-power | Power draw | Watts |
| clock-speeds | GPU clock speeds | MHz |

---

## Advanced Retry Policies

Sophisticated retry mechanisms with resource adaptation.

### Features

- **Multiple Backoff Strategies**: Fixed, exponential, fibonacci, random
- **Conditional Retry**: Only retry on specific errors
- **Resource Adjustment**: Increase resources on OOM
- **Retry Budget**: Limit total cost and time
- **Circuit Breaker**: Stop retrying after N failures

### Backoff Strategies

```yaml
# Exponential backoff
backoff:
  type: exponential
  initialDelay: 1m
  maxDelay: 1h
  multiplier: 2
# Delays: 1m, 2m, 4m, 8m, 16m, 32m, 1h (capped)

# Fibonacci backoff
backoff:
  type: fibonacci
  initialDelay: 1m
  maxDelay: 2h
# Delays: 1m, 1m, 2m, 3m, 5m, 8m, 13m, 21m...
```

### Resource Adaptation

```yaml
apiVersion: gryvia.io/v1alpha1
kind: GryviaRetryPolicy
metadata:
  name: adaptive-retry
spec:
  maxRetries: 5

  resourceAdjustment:
    enabled: true
    onOutOfMemory:
      increaseMemory: 50%
      increaseGPUMemory: true  # Switch A100-40G → A100-80G

  budget:
    maxCostUSD: 1000
    maxTotalTime: 24h
```

### Use in Jobs

```yaml
spec:
  retryPolicyRef: adaptive-retry

  checkpointing:
    enabled: true  # Required for retries
    frequency: 10m
```

---

## Resource Reservations

Reserve GPU resources in advance with guaranteed availability.

### Reservation Types

1. **Immediate**: Start now, end at specific time
2. **Scheduled**: Start and end at specific times
3. **Recurring**: Regular time slots (e.g., weekly)

### Examples

#### Exclusive Team Reservation

```yaml
apiVersion: gryvia.io/v1alpha1
kind: GryviaReservation
metadata:
  name: ml-team-reservation
spec:
  owner:
    type: team
    name: ml-research

  resources:
    gpuType: A100-80G
    gpuCount: 32

  schedule:
    type: immediate
    endTime: "2024-01-25T18:00:00Z"

  guarantees:
    exclusive: true  # No other teams can use
    preemptible: false
    sla:
      availability: 99.9

  billing:
    chargeWhenIdle: true
    discount: 15
```

#### Recurring Weekly Slot

```yaml
spec:
  schedule:
    type: recurring
    recurrence:
      cron: "0 9 * * 1"  # Every Monday 9 AM
      duration: 8h
    autoExtend: true
```

### Cost Model

- **Exclusive Reservations**: Pay whether used or not
- **Shared Reservations**: Pay only when used
- **Discounts**: 5-25% off for reservations
- **Prepaid**: Additional discount for upfront payment

### Managing Reservations

The `gryvia` CLI does not manage reservations. Create them by applying a `GryviaReservation`
manifest (see the examples above) and inspect them with `kubectl`:

```bash
# Create a reservation
kubectl apply -f my-reservation.yaml

# List reservations
kubectl get gryviareservations

# View a reservation and its status
kubectl get gryviareservation my-reservation -o yaml
```

---

## Multi-Tenancy

Hierarchical team organization with quotas, isolation, and governance.

### Features

- **Hierarchical Teams**: Sub-teams inherit from parents
- **Per-Team Quotas**: GPU hours, cost, concurrent resources
- **Network Isolation**: Optional tenant isolation
- **Storage Quotas**: Per-team storage limits
- **Compliance**: Data classification, audit retention
- **Cost Centers**: Chargeback and billing

### Example Tenant

```yaml
apiVersion: gryvia.io/v1alpha1
kind: GryviaTenant
metadata:
  name: ml-research
spec:
  displayName: "Machine Learning Research"

  members:
    - username: alice
      role: admin
    - username: bob
      role: member

  quotas:
    gpuHours:
      monthly: 2000
      burst: 500
    costUSD:
      monthly: 50000
    concurrentGPUs: 128

  priorities:
    default: normal
    allowHighPriority: true

  storageQuotas:
    home: 10Ti
    datasets: 100Ti

  billing:
    costCenter: "CC-ML-001"
    paymentMethod: credit

  governance:
    dataClassification: internal
    complianceRequirements: [SOC2]
```

### Hierarchical Teams

```
ml-research (parent)
├── ml-research-cv (computer vision)
├── ml-research-nlp (NLP)
└── ml-research-rl (reinforcement learning)
```

Each sub-team gets a portion of parent's quota.

### Managing Tenants

The `gryvia` CLI does not manage tenants. Create them by applying a `GryviaTenant` manifest (see
the example above); members and quotas are part of the tenant spec. Inspect tenants with `kubectl`
and view spending with `gryvia cost`:

```bash
# Create or update a tenant
kubectl apply -f ml-research-tenant.yaml

# View the tenant and its status
kubectl get gryviatenant ml-research -o yaml

# View GPU quota and spend for the team
gryvia quota ml-research --budget
gryvia cost ml-research --period month --detailed
```

---

## Job Templates

Reusable job configurations with parameters.

### Built-in Templates

1. **pytorch-ddp-training**: Distributed PyTorch training
2. **llm-inference-vllm**: LLM serving with vLLM
3. **jupyter-dev-env**: JupyterLab environment
4. **mlperf-training-benchmark**: MLPerf benchmarks

### Using Templates

Templates are cluster-scoped `GryviaTemplate` resources. The `gryvia` CLI does not manage or
instantiate them, so list them with `kubectl` and copy the defaults into your job manifest:

```bash
# List templates
kubectl get gryviatemplates

# View a template
kubectl get gryviatemplate pytorch-ddp-training -o yaml

# Submit the resulting job
gryvia submit --file job.yaml
```

### Custom Template

```yaml
apiVersion: gryvia.io/v1alpha1
kind: GryviaTemplate
metadata:
  name: my-training-template
spec:
  description: "Custom training template"
  category: training

  defaults:
    framework: pytorch
    resources:
      gpuType: A100-80G
      gpuCount: 8
    command:
      - python
      - train.py
      - --data={{ .dataPath }}
      - --batch-size={{ .batchSize }}

  parameters:
    - name: dataPath
      type: string
      required: true
    - name: batchSize
      type: integer
      default: "32"
      validation:
        min: 1
        max: 512
```

---

## Auto-Scaling

Queue-based auto-scaling with predictive capabilities.

### Features

- **Queue-Based**: Scale based on pending jobs
- **Multiple Triggers**: Queue time, utilization, job count
- **Cost Controls**: Budget limits, spot instances
- **Predictive Scaling**: ML-based demand forecasting
- **Scale-to-Zero**: Reduce costs when idle

### Example

```yaml
apiVersion: gryvia.io/v1alpha1
kind: GryviaAutoScaler
metadata:
  name: a100-autoscaler
spec:
  queueRef: ml-research-queue
  gpuType: A100-80G

  minNodes: 2
  maxNodes: 20

  scaleUpPolicy:
    pendingJobs: 10
    queueTimeMinutes: 30
    increment: 2
    cooldownMinutes: 5

  scaleDownPolicy:
    idleTimeMinutes: 15
    utilizationPercent: 30
    decrement: 1
    cooldownMinutes: 15

  costControls:
    maxHourlyCost: 1000
```

### Cloud Integration

```yaml
nodeProvider:
  type: cloud-provision
  cloud:
    provider: aws
    instanceType: p4d.24xlarge
    region: us-west-2
    spotInstances: true
```

---

## Budget Management

Multi-tiered budget system with forecasting.

### Features

- **Flexible Periods**: Daily, weekly, monthly, quarterly, annual
- **Multi-Dimensional Limits**: Cost, GPU hours, job count
- **Graduated Alerts**: 50%, 75%, 90%, 100%
- **Enforcement**: Warn, block, or throttle
- **Forecasting**: Predict budget exhaustion
- **Rollover**: Carry unused budget

### Example

```yaml
apiVersion: gryvia.io/v1alpha1
kind: GryviaBudget
metadata:
  name: ml-team-budget
spec:
  scope:
    type: team
    name: ml-research

  period:
    type: monthly

  limits:
    costUSD: 50000
    gpuHours: 2000

  alerts:
    - threshold: 75
      actions: [email]
    - threshold: 100
      actions: [block, email]

  enforcement:
    enabled: true
    action: block
```

### Budget Hierarchy

```
Organization: $2M/year
└── Department: $300k/quarter
    └── Team: $50k/month
        └── User: $1k/day
```

---

## Priority & Preemption

7-tier priority system with smart preemption.

### Priority Levels

| Level | Value | Use Case | Can Preempt | Quota Override |
|-------|-------|----------|-------------|----------------|
| system-critical | 1,000,000 | Infrastructure | Yes | 50% |
| production | 100,000 | Serving | Yes | 25% |
| high | 10,000 | Critical research | Yes | 10% |
| normal | 1,000 | Standard work | No | 0% |
| low | 100 | Batch jobs | No | 0% |
| best-effort | 10 | Opportunistic | No | 0% |
| spot | 50 | Spot instances | No | 0% |

### Preemption Flow

1. High-priority job submitted
2. Cluster at capacity
3. Scheduler identifies lower-priority jobs
4. Jobs checkpoint state
5. Jobs gracefully terminate
6. High-priority job starts
7. Preempted jobs auto-queue for restart

### Example

```yaml
spec:
  priorityClassName: high

  checkpointing:
    enabled: true  # Required for preemption
    frequency: 10m
```

---

## ML Workflows

Gryvia provides a complete ML workflow toolkit for managing the full lifecycle of machine learning projects.

- **GryviaAutoTuner**: Hyperparameter optimization with Grid, Random, Bayesian (TPE), and ASHA early stopping strategies.
- **GryviaWorkflow**: DAG-based multi-step ML pipelines with dependency management, conditional execution, and fan-out/fan-in patterns.
- **GryviaModelRegistry**: Model versioning with dev, staging, and production stage promotion. Supports auto-deploy on production promotion.
- **GryviaInferenceService**: Production model serving with Triton, vLLM, TensorRT-LLM, and TorchServe backends. Includes canary deployments and auto-rollback.
- **GryviaWorkspace**: Managed interactive Jupyter and VS Code environments with GPU access, persistent storage, and idle pause/resume.

For full documentation, examples, and CLI usage, see the **[ML Workflows Guide](ML_WORKFLOWS.md)**.

---

## Network Intelligence

eBPF-powered network observability, security, and performance optimization for GPU clusters.

- **27 eBPF programs** covering GPU communication (NCCL, RDMA, GPUDirect Storage), security (container escape, crypto mining, exfiltration), performance (TCP tuning, NUMA path optimization), and AI-specific analysis (training patterns, data pipeline bottlenecks, gradient compression).
- **Intent-based network policies** (GryviaFlowPolicy) for high-level traffic control.
- **Self-healing firewall** (GryviaAutoPolicy) that learns traffic patterns and generates policies automatically.
- **Anomaly detection** (GryviaNetworkAnomaly) with baseline-driven alerting.
- **Service dependency graphs** (GryviaServiceGraph) generated from observed traffic.
- **Cost attribution** (GryviaNetworkCost) per team, job, and service.
- **Training insights** (GryviaTrainingInsight) with straggler detection and communication analysis.

For full documentation, CRD examples, and CLI commands, see the **[Network Intelligence Guide](NETWORK_INTELLIGENCE.md)**.

---

## Advanced Scheduling

Sophisticated scheduling capabilities for GPU workloads.

- **Gang Scheduling**: Atomic multi-pod placement to prevent deadlocks in distributed training.
- **DRF Fair-Share Queue**: Dominant Resource Fairness with hierarchical queues, backfill, and borrowing.
- **Elastic Training**: Dynamic worker scaling via PyTorch Elastic integration with fault tolerance.
- **Admission Webhooks**: Validating webhook for quota/budget enforcement and mutating webhook for automatic NCCL environment injection.
- **Priority Preemption**: Checkpoint-aware preemption with automatic requeueing.

For full documentation and configuration examples, see the **[Advanced Scheduling Guide](SCHEDULING.md)**.

---

## OIDC/SSO Authentication

Gryvia supports enterprise authentication via OIDC (OpenID Connect) and SSO providers.

- **Supported Providers**: Okta, Azure AD, Google Workspace, Keycloak, Auth0, and any OIDC-compliant provider.
- **RBAC Integration**: OIDC groups are mapped to Gryvia roles (admin, member, viewer) for team-based access control.
- **Token Refresh**: Automatic token refresh for long-running CLI sessions and API access.
- **MFA Support**: Multi-factor authentication enforced through the identity provider.

Configure OIDC via the operator ConfigMap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: gryvia-operator-config
  namespace: gryvia-system
data:
  auth.yaml: |
    oidc:
      enabled: true
      issuerURL: https://auth.example.com
      clientID: gryvia
      clientSecret:
        secretRef: oidc-client-secret
      scopes: [openid, profile, email, groups]
      groupsClaim: groups
      roleMapping:
        admin: ["platform-admins"]
        member: ["ml-engineers", "data-scientists"]
        viewer: ["ml-viewers"]
```

---

## Python and Go SDKs

Gryvia provides official SDKs for programmatic access to all platform features.

### Python SDK

```bash
pip install gryvia
```

```python
from gryvia import GryviaClient

client = GryviaClient(
    api_url="http://gryvia-api:8000",
    api_key="your-api-key"
)

# Submit a job
job = client.jobs.create(namespace="default", spec={...})

# List models in registry
models = client.models.list(stage="production")

# Get training insights
insight = client.insights.training(job="llm-training")

# Stream logs
for line in client.jobs.logs("default", "my-job", follow=True):
    print(line)
```

### Go SDK

```bash
go get github.com/zyvorai/gryvia/sdk/go/gryvia
```

```go
import "github.com/zyvorai/gryvia/sdk/go/gryvia"

client := gryvia.NewClient(gryvia.Config{
    APIURL: "http://gryvia-api:8000",
    APIKey: "your-api-key",
})

// Submit a job
job, err := client.Jobs.Create(ctx, "default", &gryvia.JobSpec{...})

// List models in registry
models, err := client.Models.List(ctx, gryvia.ModelFilter{Stage: "production"})

// Get training insights
insight, err := client.Insights.Training(ctx, "llm-training")
```

For full SDK documentation and examples, see the [API Reference](developer-guide/api-reference.md#sdks).

---

## Performance Tips

### GPU Utilization

- **Target**: >85% GPU utilization
- **Enable Mixed Precision**: 2-3x speedup
- **Optimize Data Loading**: Use prefetching
- **Increase Batch Size**: Fill GPU memory

### Cost Optimization

- **Use Spot Instances**: 40-70% savings
- **Enable MIG**: 86% savings for small workloads
- **Right-Size Resources**: Profile and adjust
- **Use Reservations**: 15-25% discount

### Reliability

- **Enable Checkpointing**: Resume after failures
- **Use Retry Policies**: Automatic recovery
- **Health Monitoring**: Proactive maintenance
- **Resource Reservations**: Guaranteed availability

---

## Best Practices

### 1. Always Enable Checkpointing

```yaml
spec:
  checkpointing:
    enabled: true
    frequency: 10m
    path: /checkpoints
```

### 2. Use Templates

Don't repeat job configurations. Create templates.

### 3. Set Realistic Budgets

Analyze historical usage before setting budgets.

### 4. Monitor Health

Enable cluster-wide health checks.

### 5. Use Appropriate Priorities

Don't abuse high priority. Reserve for critical work.

### 6. Profile Jobs

Use `gryvia gpu training` and `gryvia gpu nccl` to identify optimization opportunities.

### 7. Reserve for Deadlines

Book reservations in advance for critical work.

### 8. Enable Auto-Scaling

Let the system scale based on demand.

---

## Troubleshooting

### Job Won't Start

```bash
# Check budget
kubectl get gryviabudgets
gryvia quota my-team --budget

# Check reservation
kubectl get gryviareservations

# Check health
gryvia health
```

### High Costs

```bash
# Analyze costs
gryvia cost my-team --detailed

# Compare periods
gryvia cost my-team --period week

# Check for idle GPUs
gryvia capacity
```

### Poor Performance

```bash
# Check job status and logs
gryvia status my-job
gryvia logs my-job --tail 100

# Check GPU health
gryvia health gpu

# Check the node
gryvia get node gpu-node-05

# Check GPU communication (NCCL) for the job
gryvia gpu nccl --job my-job
```

---

## Support

- **Documentation**: https://gryvia.io/docs
- **Issues**: https://github.com/zyvorai/gryvia/issues
- **Discussions**: https://github.com/zyvorai/gryvia/discussions
- **Slack**: #gryvia-users

---

*Gryvia - Enterprise GPU Infrastructure Management*
