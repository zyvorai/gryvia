# Enterprise & Cost Management Examples

Example YAML manifests for Gryvia enterprise CRDs covering cost tracking, observability, multi-tenancy, budgets, and SLAs.

## Examples

- **[network-cost.yaml](network-cost.yaml)** - `FabricNetworkCost` tracking per-team network costs with zone-based pricing
- **[training-insight.yaml](training-insight.yaml)** - `FabricTrainingInsight` monitoring NCCL communication in a distributed training job
- **[inference-insight.yaml](inference-insight.yaml)** - `FabricInferenceInsight` analyzing per-phase latency for an inference endpoint
- **[tenant.yaml](tenant.yaml)** - `FabricTenant` creating an isolated team with quotas, billing, and governance
- **[budget.yaml](budget.yaml)** - `FabricBudget` with a $50K monthly limit and graduated alert thresholds
- **[sla.yaml](sla.yaml)** - `FabricSLA` with gold-tier queue time, uptime, and resource guarantees

## Usage

```bash
kubectl apply -f network-cost.yaml
kubectl apply -f training-insight.yaml
kubectl apply -f inference-insight.yaml
kubectl apply -f tenant.yaml
kubectl apply -f budget.yaml
kubectl apply -f sla.yaml
```
