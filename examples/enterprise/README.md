# Enterprise & Cost Management Examples

Example YAML manifests for Gryvia enterprise CRDs covering cost tracking, observability, multi-tenancy, budgets, and SLAs.

## Examples

- **[network-cost.yaml](network-cost.yaml)** - `GryviaNetworkCost` tracking per-team network costs with zone-based pricing
- **[training-insight.yaml](training-insight.yaml)** - `GryviaTrainingInsight` monitoring NCCL communication in a distributed training job
- **[inference-insight.yaml](inference-insight.yaml)** - `GryviaInferenceInsight` analyzing per-phase latency for an inference endpoint
- **[tenant.yaml](tenant.yaml)** - `GryviaTenant` creating an isolated team with quotas, billing, and governance
- **[budget.yaml](budget.yaml)** - `GryviaBudget` with a $50K monthly limit and graduated alert thresholds
- **[sla.yaml](sla.yaml)** - `GryviaSLA` with gold-tier queue time, uptime, and resource guarantees

## Usage

```bash
kubectl apply -f network-cost.yaml
kubectl apply -f training-insight.yaml
kubectl apply -f inference-insight.yaml
kubectl apply -f tenant.yaml
kubectl apply -f budget.yaml
kubectl apply -f sla.yaml
```
