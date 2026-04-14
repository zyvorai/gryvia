# Scheduling Examples

Example YAML manifests for TensorReaper GPU scheduling CRDs covering elastic training, gang scheduling, and priority-based preemption.

## Examples

- **[elastic-training.yaml](elastic-training.yaml)** - `FabricAIJob` with elastic annotations scaling between 2 and 8 nodes
- **[gang-scheduling.yaml](gang-scheduling.yaml)** - `FabricAIJob` distributed job requiring atomic allocation of all 4 nodes
- **[priority-preemption.yaml](priority-preemption.yaml)** - `FabricPriority` defining critical, normal, and preemptible scheduling tiers

## Usage

```bash
# Apply priority classes first (cluster-scoped)
kubectl apply -f priority-preemption.yaml

# Then submit jobs that reference those priorities
kubectl apply -f elastic-training.yaml
kubectl apply -f gang-scheduling.yaml
```
