# Advanced Features Guide

Overview of Gryvia's advanced capability areas, with what is implemented and what is only a CRD or a design.

## Status of each area

| Area | Kind(s) | State |
|------|---------|-------|
| GPU health monitoring | `GryviaHealthCheck` | CRD only, no controller registered. `gryvia health` is separate and reads `GryviaGpuNode`, `GryviaStorage` and `GryviaNetwork` status |
| Retry policies | `GryviaRetryPolicy` | CRD only, no controller |
| Reservations | `GryviaReservation` | CRD only, no controller |
| Multi-tenancy | `GryviaTenant`, `GryviaQuota` | Running: quota-operator creates the `tenant-<name>` namespace, ResourceQuota, LimitRange and an optional NetworkPolicy |
| Job templates | `GryviaTemplate` | CRD only, no controller |
| Auto-scaling | `GryviaAutoScaler` | CRD only, no controller; no node provisioning exists |
| Budgets | `GryviaBudget` | CRD only, no controller; nothing enforces or alerts |
| Priority and preemption | `GryviaAIJob.spec.priority`, Kueue | Opt-in via the Kueue integration (`--kueue-integration`): priority maps to a WorkloadPriorityClass and preempts within a queue, victims are requeued; unit-tested, e2e unverified ([details](https://github.com/zyvorai/gryvia/blob/main/docs/kueue-integration.md)). Without it `spec.priority` is validated but not acted on. `GryviaPriority` is CRD only |
| ML workflows | AutoTuner, Workflow, ModelRegistry, InferenceService, Workspace | CRD and gateway/dashboard CRUD only; see [ML Workflows](ML_WORKFLOWS.md) |
| Network intelligence | 10 kinds | Running via the network-intelligence operator (own chart); eBPF collector off by default; see [Network Intelligence](NETWORK_INTELLIGENCE.md) |
| Advanced scheduling | | Mostly library code; see [Scheduling](SCHEDULING.md) |
| OIDC/SSO | gateway | Implemented in the gateway; not verified against a real identity provider |
| SDKs | | Python REST client and Go Kubernetes client, from source |

"CRD only" means the manifest is accepted by the API server and the schema is real, but nothing reconciles its spec, so it has no effect. The full list is in the [CRD reference](../reference/crds.md). Numbers such as discounts, savings percentages and speedups that appeared in earlier versions of this page were illustrative, not measured, and have been removed.

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

Status: the `GryviaHealthCheck` CRD exists, but no controller is registered for it, so scheduled checks, alerting, auto-remediation and automatic cordoning do not happen. What does work is the CLI view of node, storage and network status written by the gpu-, storage- and network-operators.

### Schema example (not acted on today)

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

# Manual remediation: cordon and drain the node (Eviction API, respects PDBs),
# then reset the GPU on the host yourself
gryvia maintenance start gpu-node-05 --reason gpu-reset --drain

# Return the node to service
gryvia maintenance end gpu-node-05

# The GryviaHealthCheck object can be created and read, but has no status yet
kubectl get gryviahealthcheck cluster-gpu-health -o yaml
```

---

## Advanced Retry Policies

Status: CRD only. `GryviaRetryPolicy` defines `maxRetries`, `backoff`, `retryOn`/`noRetryOn`, `circuitBreaker`, `resourceAdjustment` and `budget`, but no controller applies it and `GryviaAIJob` has no field that references a policy. The only retry-related job fields are `spec.retryLimit` and `status.retries`, which the controller does not act on today.

### Schema example (not acted on today)

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

---

## Resource Reservations

Status: CRD only. Nothing reserves capacity, blocks other teams or bills for a reservation. The schema has `owner`, `resources`, `schedule`, `guarantees`, `billing` and `notifications`.

### Schema examples (not acted on today)

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

### Working with it today

The `gryvia` CLI does not manage reservations. You can create and read the object with `kubectl`, but it has no effect:

```bash
kubectl apply -f my-reservation.yaml
kubectl get gryviareservations
```

The `guarantees.sla.availability` and `billing.discount` values in the example are placeholders; no SLA is enforced and no discounts are computed.

---

## Multi-Tenancy

Status: implemented by the quota-operator (`GryviaTenant`, `GryviaQuota`). For each tenant the controller ensures the namespace `tenant-<name>`, a ResourceQuota when `spec.quotas` is set, a LimitRange, and a NetworkPolicy when `spec.networkPolicy.isolated` is true, and it writes usage and quota utilisation into the tenant status. The admission webhook checks the tenant's allowed SKUs and the namespace quotas when jobs are created. Tested with unit tests against fake clients, not on a production cluster. Fields such as `billing`, `governance`, `priorities` and `notifications` are stored in the spec; the controller does not enforce them.

For the tenant and quota model, see [GPU as a service](GPU_AS_A_SERVICE.md). Costs are estimates from the `GryviaGpuSku` catalog; there is no payment integration.

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

### Hierarchy

The schema has `parentTenant`, but the controller does not inherit quotas from a parent tenant; each tenant is reconciled on its own.

### Managing Tenants

```bash
# Create or update a tenant with the CLI (namespace tenant-ml-research)
gryvia tenant create ml-research --display-name "Machine Learning Research" \
  --allowed-sku a100-80g --max-gpus 128 --isolated true
gryvia tenant list
gryvia tenant get ml-research

# Or apply a full manifest
kubectl apply -f ml-research-tenant.yaml
kubectl get gryviatenant ml-research -o yaml

# GPU quota and estimated spend
gryvia quota ml-research --budget
gryvia cost ml-research --period month --detailed
```

---

## Job Templates

Status: CRD only. `GryviaTemplate` (cluster-scoped, `category` and `defaults` required) is plain data: no controller renders `{{ .param }}` placeholders, validates parameters or instantiates jobs, and no built-in templates are shipped in this repository. The `gryvia` CLI does not manage them.

```bash
kubectl get gryviatemplates
kubectl get gryviatemplate my-training-template -o yaml
```

Use the defaults as a reference when writing a `GryviaAIJob` manifest and submit that with `gryvia submit --file job.yaml`.

### Schema example (not acted on today)

Note that the template's `defaults` schema (`framework`, `resources.gpuType`, `resources.gpuCount`) does not match the `GryviaAIJob` spec (`gpus`, `gpuType`), so defaults cannot be copied into a job verbatim.

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

Status: CRD only. `GryviaAutoScaler` has `queueRef`, `gpuType`, `minNodes`, `maxNodes`, `scaleUpPolicy`, `scaleDownPolicy`, `costControls`, `nodeProvider` and `advanced`, but no controller reads it, there is no queue kind for `queueRef` to point at, and no code provisions or removes nodes or cloud instances. Predictive scaling, scale-to-zero and spot handling are not implemented. Use your cluster autoscaler for node scaling.

### Schema example (not acted on today)

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

---

## Budget Management

Status: CRD only. `GryviaBudget` has `scope`, `period`, `limits`, `alerts`, `enforcement`, `rollover` and `priority`, but no controller evaluates it: nothing sends alerts, blocks jobs or forecasts exhaustion. The admission webhook does not check budgets. For real numbers today use `gryvia cost` and `gryvia usage` (estimates from metered GPU usage and the SKU catalog) and the quota limits in `GryviaQuota`.

### Schema example (not acted on today)

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

---

## Priority & Preemption

Status: opt-in through Kueue (`--kueue-integration`, see [Kueue integration](https://github.com/zyvorai/gryvia/blob/main/docs/kueue-integration.md); unit-tested, kind e2e written but unverified), otherwise not implemented. A `GryviaPriority` CRD (`value` required, plus `preemptionPolicy`, `quotaOverride`, `sla`) exists but no controller is registered for it. `GryviaAIJob.spec.priority` is an integer from 0 to 100 that the admission webhook range-checks; without the Kueue integration the scheduler and controller do not order or preempt by it, and there is no `priorityClassName` on the job. The seven-tier table with quota override percentages that earlier versions showed was a proposal.

See [Scheduling](SCHEDULING.md#priority-preemption) for the details.

---

## ML Workflows

The ML workflow kinds have CRDs and gateway/dashboard CRUD, but no controller is registered for any of them (no trials, DAG execution, model serving or workspace pods).

- **GryviaAutoTuner**: hyperparameter study spec (grid, random, bayesian, asha).
- **GryviaWorkflow**: DAG step spec with `dependsOn`.
- **GryviaModelRegistry**: model version records with a `stage` field (the gateway can patch it; nothing deploys).
- **GryviaInferenceService**: serving spec with canary and autoscaling fields; no rollout runs.
- **GryviaWorkspace**: notebook environment spec; no pod is created.

For full documentation, examples, and CLI usage, see the **[ML Workflows Guide](ML_WORKFLOWS.md)**.

---

## Network Intelligence

eBPF-powered network observability, security, and performance optimization for GPU clusters.

- **35 CO-RE eBPF programs** covering GPU communication (NCCL, RDMA, GPUDirect Storage), security detections and network paths. They have been verified only on Linux 7.0 x86_64 (arm64 is compile-only); the collector is off by default, runs privileged with host networking, and GPU/NCCL/RDMA behaviour is unverified on real hardware.
- **Network policy kinds** (GryviaFlowPolicy) with a registered controller in the network-intelligence operator.
- **GryviaAutoPolicy** for policy suggestions from observed traffic.
- **Anomaly detection** (GryviaNetworkAnomaly) with baseline-driven alerting.
- **Service dependency graphs** (GryviaServiceGraph) generated from observed traffic.
- **Cost attribution** (GryviaNetworkCost) per team, job, and service.
- **Training insights** (GryviaTrainingInsight) and a fabric-signal CRD (`gryviafabricsignals`) for straggler and fabric health signals (the fabric signal kind has a CRD but no controller yet).

For full documentation, CRD examples, and CLI commands, see the **[Network Intelligence Guide](NETWORK_INTELLIGENCE.md)**.

---

## Advanced Scheduling

What runs: GPU-aware node selection and a validating admission webhook (quota and SKU policy, fails open). Opt-in and Kueue-backed (`--kueue-integration`; unit-tested, kind e2e written but unverified): gang admission, per-tenant queues and quota with borrowing, priority preemption ([Kueue integration](https://github.com/zyvorai/gryvia/blob/main/docs/kueue-integration.md)). Library code only, not called by anything: the in-tree gang scheduler, the DRF queue, elastic scaling and the mutating NCCL-injection webhook. Backfill is not implemented.

For details, see the **[Scheduling Guide](SCHEDULING.md)**.

---

## OIDC/SSO Authentication

The API gateway can validate OIDC JWTs. The gateway is configured through environment variables set by chart values (`OIDC_ENABLED`, `OIDC_ISSUER_URL`, `OIDC_CLIENT_ID`, `OIDC_AUDIENCE`, `GRYVIA_OIDC_ADMIN_GROUPS`), not through a `gryvia-operator-config` ConfigMap. OIDC users are tenant users (namespaces `tenant-<name>`) unless their `groups` claim matches an admin group. The API key and session tokens keep working alongside OIDC. Token refresh and MFA are functions of your identity provider and the dashboard's OIDC flow, not features Gryvia adds.

This has not been verified against a real identity provider. Details and the full option list are in [Auth and TLS](AUTH_AND_TLS.md).

---

## Python and Go SDKs

There are two SDKs in the repository, neither published as a package by this project (install from source), and they are not equivalent:

- **Python** (`sdk/python`): an async REST client of the gateway API for jobs, nodes, quotas, costs and metrics. It has no `models` or `insights` clients and does not cover tenants, SKUs, usage or invoices.
- **Go** (`sdk/go`): a controller-runtime Kubernetes client for the Gryvia custom resources. It talks to the Kubernetes API, not to the REST gateway.

```bash
pip install -e sdk/python
```

See the [API Reference](../developer-guide/api-reference.md#sdks) and `sdk/python/README.md`.

---

## Performance Tips

General ML practice, not Gryvia-specific and not measured here:

- Keep GPUs fed: prefetch data, increase batch size until memory is filled, use mixed precision where it suits the model.
- Right-size requests: profile before choosing GPU count and type.
- Checkpoint from your training code so a restarted pod can resume.
- Use `gryvia capacity` and `gryvia cost` to see free GPUs and estimated spend.

---

## Best Practices

### 1. Checkpoint from your training code

Write checkpoints to a mounted volume (`spec.volumes` and `spec.volumeMounts`, or `spec.storage` for a PVC at `/data`). There is no `checkpointing` block on `GryviaAIJob`.

### 2. Keep job manifests in version control

Templates are not instantiated by any controller today, so copy a known-good manifest.

### 3. Set quotas

Use `GryviaQuota` and `GryviaTenant` limits; budgets are not enforced.

### 4. Monitor health

Use `gryvia health` and `gryvia status`; `GryviaHealthCheck` objects are not evaluated.

### 5. Profile jobs

Use `gryvia gpu training` and `gryvia gpu nccl` to identify optimization opportunities.

---

## Troubleshooting

### Job Won't Start

```bash
# Why was it not scheduled? See the Scheduled condition and message
kubectl describe gryviaaijob my-job

# Jobs waiting for resources, and free GPUs
gryvia queue
gryvia capacity

# Quota and tenant limits
gryvia quota my-team --budget

# Check health
gryvia health
```

### High Costs

```bash
# Analyze costs
gryvia cost my-team --detailed

# Compare periods
gryvia cost my-team --period week

# Free GPUs
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

---

*Gryvia - Enterprise GPU Infrastructure Management*
