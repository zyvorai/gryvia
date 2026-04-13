# TensorReaper Quota Operator

The Quota Operator manages GPU quotas and budgets for teams in TensorReaper. It enforces resource limits, tracks spending, and ensures fair resource allocation across teams.

## Features

- **GPU Quotas** - Limit max GPUs per team and per job
- **Job Limits** - Control max running and queued jobs
- **Budget Management** - Track spending and enforce budget limits
- **GPU Type Restrictions** - Allow/deny specific GPU models per team
- **Priority Scheduling** - Higher priority teams get preference
- **Namespace Labeling** - Automatic team labeling of namespaces
- **Cost Tracking** - GPU-hour tracking with configurable pricing

## Architecture

```
FabricQuota CR
       ↓
Quota Operator (1min reconcile loop)
       ↓
  ┌────┴─────┬──────────┬─────────┐
  ↓          ↓          ↓         ↓
Label     Calculate  Enforce   Track
Namespaces Usage     Quota     Budget
  ↓          ↓          ↓         ↓
Team Tags  GPU Count  Reject    Monthly
           Job Count  Jobs      Spending
```

## Installation

```bash
# Apply CRD
kubectl apply -f crds/fabricquota.yaml

# Deploy operator
kubectl apply -f operators/quota-operator/config/

# Verify deployment
kubectl get pods -n tensorreaper -l app=quota-operator
```

## Usage

### Create Team Quota

```yaml
apiVersion: tensorreaper.ai/v1
kind: FabricQuota
metadata:
  name: team-ml
spec:
  team: ml-research
  namespaces:
    - ml-training
    - ml-inference
  priority: 80

  gpuQuota:
    maxGPUs: 32
    maxGPUsPerJob: 16
    maxRunningJobs: 5
    maxQueuedJobs: 10
    allowedGPUTypes:
      - H100
      - A100-80G

  budget:
    monthlyBudget: 50000.00
    alertThreshold: 80.0
    hardLimit: true
```

### Check Quota Status

```bash
kubectl get fabricquota team-ml -o yaml
```

Output:
```yaml
status:
  phase: Active
  currentUsage:
    allocatedGPUs: 16
    runningJobs: 2
    queuedJobs: 3
    gpuHours: 2456.5
  budgetStatus:
    spentThisMonth: 19652.00
    remainingBudget: 30348.00
    percentUsed: 39.3
    projectedSpend: 45234.00
  lastUpdated: "2024-01-15T10:30:00Z"
```

## Implementation Details

### Quota Controller

**Main Reconciler** (`controllers/fabricquota_controller.go`)
- Reconciles every 1 minute
- Uses `EnqueueRequestsFromMapFunc` to watch FabricAIJob changes and map them to the corresponding FabricQuota objects, ensuring quotas are re-evaluated promptly when jobs change
- Labels namespaces with team info
- Calculates current GPU usage
- Enforces quota limits (including `AllowedGPUTypes` -- jobs requesting a disallowed GPU type are queued rather than scheduled)
- Tracks budget spending
- Leader election is enabled by default
- ~250 lines of code

### Usage Tracking

**Usage Calculator** (`pkg/usage/usage.go`)
- Counts allocated GPUs across team namespaces
- Tracks running and queued jobs
- Calculates GPU-hours for billing
- Filters by current month for budget tracking

### Budget Management

**Budget Calculator** (`pkg/budget/budget.go`)
- Configurable GPU pricing by type
- Monthly cost calculation
- Projected spending based on burn rate
- Alert threshold notifications
- Hard limit enforcement

**Default Pricing:**
```go
H100:     $8.00/hour
A100-80G: $4.00/hour
A100-40G: $3.00/hour
L40:      $2.50/hour
A10:      $1.50/hour
V100:     $2.00/hour
T4:       $0.75/hour
```

### Quota Enforcement

When quota is exceeded:
1. **GPU Limit**: New jobs are rejected if they would exceed `maxGPUs`
2. **Job Limit**: Jobs are queued if `maxRunningJobs` reached
3. **Budget Limit**: If `hardLimit: true`, new jobs rejected when budget exceeded
4. **GPU Type**: Jobs requesting disallowed GPU types are queued (not scheduled)

Job rejection is atomic — if the status update to mark a job as "Rejected"
fails, the operator returns an error and retries on the next reconciliation,
ensuring no job silently bypasses quota enforcement.

## Examples

### High-Priority LLM Team

```yaml
apiVersion: tensorreaper.ai/v1
kind: FabricQuota
metadata:
  name: team-nlp
spec:
  team: nlp
  namespaces: [nlp-training]
  priority: 90  # Highest priority

  gpuQuota:
    maxGPUs: 64
    maxGPUsPerJob: 32
    allowedGPUTypes: [H100]  # H100 only

  budget:
    monthlyBudget: 100000.00
    hardLimit: true
```

### Development Team with Soft Limits

```yaml
apiVersion: tensorreaper.ai/v1
kind: FabricQuota
metadata:
  name: team-dev
spec:
  team: development
  namespaces: [dev-experiments]
  priority: 50

  gpuQuota:
    maxGPUs: 8
    maxGPUsPerJob: 4
    allowedGPUTypes: [T4, A10]  # Lower-cost GPUs

  budget:
    monthlyBudget: 5000.00
    alertThreshold: 80.0
    hardLimit: false  # Warnings only
```

## Monitoring

### View All Quotas

```bash
kubectl get fabricquotas
```

Output:
```
NAME        TEAM           PHASE    ALLOCATED   MAX    SPENT      BUDGET
team-ml     ml-research    Active   16/32       32     $19,652    $50,000
team-cv     computer-vision Active   8/24        24     $8,234     $30,000
team-nlp    nlp            Active   32/64       64     $78,456    $100,000
team-dev    development    Active   4/8         8      $1,234     $5,000
```

### Check Budget Status

```bash
kubectl get fabricquota team-ml -o jsonpath='{.status.budgetStatus}' | jq
```

Output:
```json
{
  "spentThisMonth": 19652.00,
  "remainingBudget": 30348.00,
  "percentUsed": 39.3,
  "projectedSpend": 45234.00
}
```

### Alert on Budget Threshold

The operator sets a condition when budget threshold is reached:

```bash
kubectl get fabricquota team-ml -o jsonpath='{.status.conditions[?(@.type=="BudgetAlert")]}'
```

## Integration with AI Workload Operator

The Quota Operator works with the AI Workload Operator to:

1. **Pre-Submission Validation**: Check quotas before scheduling jobs
2. **Job Rejection**: Mark jobs as "Rejected" if quota exceeded
3. **Priority Scheduling**: Higher priority teams get scheduled first
4. **Cost Attribution**: Track GPU-hours per team for chargeback

### Job Rejection Example

```yaml
# Job status when quota exceeded
status:
  phase: Rejected
  message: "Budget exceeded for team ml-research"
```

## Custom Pricing

Update GPU pricing for your infrastructure:

```go
import "github.com/ssahani/tensorreaper/operators/quota-operator/pkg/budget"

// Set custom H100 pricing (returns error on invalid input)
if err := budget.UpdatePricing("H100", 10.00); err != nil {
    log.Fatal(err)  // e.g. empty GPU type, negative rate, NaN
}

// Set custom on-prem pricing
if err := budget.UpdatePricing("A100-80G", 0.50); err != nil {
    log.Fatal(err)
}
```

`UpdatePricing` validates its inputs and returns an error if the GPU type
is empty or the hourly rate is negative, NaN, or infinite.

## RBAC

The operator requires:
- `fabricquotas`: Full CRUD
- `fabricaijobs`: Get, List, Watch, Update, Patch
- `namespaces`: Get, List, Watch, Update, Patch
- `resourcequotas`: Full CRUD (future use)

## Troubleshooting

**Quota Not Enforced**
```bash
# Check operator logs
kubectl logs -n tensorreaper -l app=quota-operator

# Verify quota exists
kubectl get fabricquota

# Check namespace labels
kubectl get namespace ml-training -o yaml | grep tensorreaper.ai/team
```

**Incorrect Budget Calculation**
```bash
# Verify GPU pricing
kubectl exec -it <operator-pod> -- env | grep GPU_PRICING

# Check job timestamps
kubectl get fabricaijobs -n ml-training -o yaml | grep startTime

# Recalculate manually
kubectl delete fabricquota team-ml
kubectl apply -f examples/quota/team-ml-quota.yaml
```

**Jobs Not Being Rejected**
```bash
# Check quota status
kubectl get fabricquota team-ml -o jsonpath='{.status.phase}'

# Verify job namespace has correct team label
kubectl get namespace ml-training -o jsonpath='{.metadata.labels}'

# Check job phase
kubectl get fabricaijob -n ml-training <job-name> -o jsonpath='{.status.phase}'
```

## Performance

- **Reconciliation**: Every 1 minute
- **Memory Usage**: ~50MB per quota
- **CPU**: <100m per quota
- **Scalability**: Tested with 100+ quotas

## Roadmap

- [ ] Prometheus metrics export (quota usage, budget)
- [ ] Webhook for admission control (pre-validate jobs)
- [ ] Slack/Email notifications for budget alerts
- [ ] Grafana dashboard for quota visualization
- [ ] Historical usage reporting
- [ ] Quota templates for common team sizes
- [ ] Fair-share scheduling based on quota
- [ ] GPU time banking (roll-over unused quota)

## Best Practices

1. **Set Realistic Limits**: Base `maxGPUs` on historical usage patterns
2. **Use Priorities**: Assign higher priority to production teams
3. **Enable Alerts**: Set `alertThreshold` to 80% for advance warning
4. **Soft Limits for Dev**: Use `hardLimit: false` for experimentation teams
5. **GPU Type Restrictions**: Limit expensive GPUs (H100) to teams that need them
6. **Review Monthly**: Adjust budgets based on actual spending patterns

## Example Deployment

```bash
# Create quotas for 3 teams
kubectl apply -f examples/quota/team-ml-quota.yaml
kubectl apply -f examples/quota/team-cv-quota.yaml
kubectl apply -f examples/quota/team-nlp-quota.yaml

# Verify all quotas are active
kubectl get fabricquotas

# Launch a training job
kubectl apply -f - <<EOF
apiVersion: tensorreaper.ai/v1
kind: FabricAIJob
metadata:
  name: llm-training
  namespace: nlp-training
spec:
  framework: pytorch
  resources:
    gpuType: H100
    gpuCount: 32
  image: nvcr.io/nvidia/pytorch:24.01-py3
  command: ["torchrun", "train.py"]
EOF

# Check quota updated
kubectl get fabricquota team-nlp -o jsonpath='{.status.currentUsage}'
```
