# Advanced Features Guide

Complete guide to KubeFabric's advanced capabilities for enterprise GPU infrastructure management.

## Table of Contents

1. [GPU Health Monitoring](#gpu-health-monitoring)
2. [Advanced Retry Policies](#advanced-retry-policies)
3. [Resource Reservations](#resource-reservations)
4. [Multi-Tenancy](#multi-tenancy)
5. [Job Templates](#job-templates)
6. [Auto-Scaling](#auto-scaling)
7. [Budget Management](#budget-management)
8. [Priority & Preemption](#priority--preemption)

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
apiVersion: kubefabric.io/v1
kind: FabricHealthCheck
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
kfctl health status cluster-gpu-health

# Diagnose specific GPU
kfctl health diagnose --node gpu-node-05 --gpu 3

# Manual remediation
kfctl health remediate gpu-node-05 --action gpu-reset

# View history
kfctl health history gpu-node-05
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
apiVersion: kubefabric.io/v1
kind: FabricRetryPolicy
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
apiVersion: kubefabric.io/v1
kind: FabricReservation
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

### CLI

```bash
# Create reservation
kfctl reservation create my-reservation \
  --team ml-research \
  --gpu-type A100-80G \
  --gpu-count 32 \
  --duration 4d \
  --exclusive

# View utilization
kfctl reservation usage my-reservation
# Output: 75% utilized, $12,960 wasted
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
apiVersion: kubefabric.io/v1
kind: FabricTenant
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

### CLI

```bash
# Create tenant
kfctl tenant create ml-research \
  --quota-gpus 128 \
  --quota-cost 50000

# Add member
kfctl tenant add-member ml-research alice --role admin

# View usage
kfctl tenant usage ml-research

# Generate report
kfctl tenant report ml-research --month 2024-01 --output report.pdf
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

```bash
# Create job from template
kfctl job create --template pytorch-ddp-training \
  --param dataPath=/data/imagenet \
  --param batchSize=128 \
  --param epochs=90
```

### Custom Template

```yaml
apiVersion: kubefabric.io/v1
kind: FabricTemplate
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
apiVersion: kubefabric.io/v1
kind: FabricAutoScaler
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
apiVersion: kubefabric.io/v1
kind: FabricBudget
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

Use performance profiler to identify optimization opportunities.

### 7. Reserve for Deadlines

Book reservations in advance for critical work.

### 8. Enable Auto-Scaling

Let the system scale based on demand.

---

## Troubleshooting

### Job Won't Start

```bash
# Check budget
kfctl budget status --team my-team

# Check reservation
kfctl reservation list

# Check health
kfctl health status cluster-gpu-health
```

### High Costs

```bash
# Analyze costs
kfctl cost analyze --team my-team

# Get recommendations
kfctl cost optimize --team my-team

# Check for idle resources
kfctl cost waste --team my-team
```

### Poor Performance

```bash
# Profile job
kfctl profile my-job

# Check GPU health
kfctl health check node gpu-node-05

# View metrics
kfctl metrics gpu-utilization
```

---

## Support

- **Documentation**: https://kubefabric.io/docs
- **Issues**: https://github.com/ssahani/kube-fabric/issues
- **Discussions**: https://github.com/ssahani/kube-fabric/discussions
- **Slack**: #kubefabric-users

---

*KubeFabric - Enterprise GPU Infrastructure Management*
