# Multi-Cluster Federation

Deploy and manage KubeFabric across multiple Kubernetes clusters for high availability, disaster recovery, and geographic distribution.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     Federation Control Plane                    │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐ │
│  │   Scheduler  │  │ Job Manager  │  │  Cost Aggregator     │ │
│  └──────────────┘  └──────────────┘  └──────────────────────┘ │
└────────────┬─────────────┬───────────────────┬─────────────────┘
             │             │                   │
    ┌────────┴────┐  ┌────┴─────┐  ┌─────────┴────────┐
    │             │  │          │  │                  │
┌───▼────────┐ ┌──▼─────────┐ ┌▼──────────┐ ┌───────▼──────┐
│ Cluster A  │ │ Cluster B  │ │ Cluster C │ │  Cluster D   │
│  (US-West) │ │ (US-East)  │ │   (EU)    │ │   (Asia)     │
│            │ │            │ │           │ │              │
│ 32x H100   │ │ 64x A100   │ │ 32x A100  │ │  16x A100    │
└────────────┘ └────────────┘ └───────────┘ └──────────────┘
```

## Features

- **Job Distribution**: Automatically distribute jobs across clusters
- **Failover**: Automatic failover if a cluster becomes unavailable
- **Cost Optimization**: Schedule jobs to lowest-cost cluster
- **Geographic Affinity**: Keep data and compute in same region
- **Unified Management**: Single pane of glass for all clusters

## Setup

### Prerequisites

- Multiple Kubernetes clusters (v1.28+)
- Network connectivity between clusters
- KubeFabric installed on each cluster

### Install Federation Control Plane

```bash
# On management cluster
helm install kubefabric-federation kubefabric/kubefabric-federation \
  --namespace kubefabric-system \
  --create-namespace
```

### Register Member Clusters

```yaml
# cluster-a.yaml
apiVersion: kubefabric.io/v1
kind: FabricCluster
metadata:
  name: cluster-a
  namespace: kubefabric-system
spec:
  apiEndpoint: https://cluster-a.example.com:6443
  region: us-west-2
  provider: baremetal
  credentials:
    secretRef:
      name: cluster-a-kubeconfig
  capabilities:
    totalGPUs: 32
    gpuTypes:
      - type: H100
        count: 32
    storage:
      - type: vast
        capacity: 500Ti
    network:
      infiniband: true
      bandwidth: 3200Gbps
  pricing:
    gpuHourlyRates:
      H100: 30.00
    currency: USD
  status:
    state: Ready
    health: Healthy
    availableGPUs: 16
```

```bash
# Create secret with kubeconfig
kubectl create secret generic cluster-a-kubeconfig \
  --from-file=kubeconfig=~/.kube/cluster-a.yaml \
  -n kubefabric-system

# Register cluster
kubectl apply -f cluster-a.yaml

# Repeat for each cluster
```

### Verify Federation

```bash
# List registered clusters
kubectl get fabricclusters -n kubefabric-system

# Check cluster status
kubectl describe fabriccluster cluster-a

# View aggregated resources
kfctl federation status
```

## Job Placement

### Automatic Placement

Jobs are automatically placed based on:
1. Resource availability
2. Cost optimization
3. Data locality
4. Geographic constraints
5. Team preferences

```yaml
apiVersion: kubefabric.io/v1
kind: FabricAIJob
metadata:
  name: training-job
  annotations:
    kubefabric.io/placement: auto
spec:
  framework: pytorch
  resources:
    gpuType: A100-80G
    gpuCount: 8
  # ... rest of spec
```

### Manual Cluster Selection

```yaml
metadata:
  annotations:
    kubefabric.io/target-cluster: cluster-a
```

### Cluster Affinity

```yaml
metadata:
  annotations:
    kubefabric.io/cluster-affinity: |
      preferredClusters:
        - cluster-a
        - cluster-b
      avoidClusters:
        - cluster-d
```

### Regional Constraints

```yaml
metadata:
  annotations:
    kubefabric.io/region: us-west-2
    kubefabric.io/region-affinity: required  # or preferred
```

### Cost-Optimized Placement

```yaml
metadata:
  annotations:
    kubefabric.io/placement-strategy: cost-optimized
    kubefabric.io/max-cost-per-hour: "200.00"
```

## Data Management

### Cross-Cluster Data

```yaml
# Replicate dataset across clusters
apiVersion: kubefabric.io/v1
kind: FabricDataset
metadata:
  name: imagenet
spec:
  source:
    cluster: cluster-a
    path: /datasets/imagenet
  replication:
    enabled: true
    clusters:
      - cluster-b
      - cluster-c
    strategy: on-demand  # or eager
  size: 150Gi
```

### Data Locality

```yaml
# Schedule job where data exists
apiVersion: kubefabric.io/v1
kind: FabricAIJob
metadata:
  name: training-with-data
spec:
  dataAffinity:
    dataset: imagenet
    locality: required  # Job must run where data exists
  # ... rest of spec
```

## Failover and HA

### Automatic Failover

```yaml
# Enable automatic failover
apiVersion: kubefabric.io/v1
kind: FabricAIJob
metadata:
  name: critical-job
  annotations:
    kubefabric.io/failover: enabled
    kubefabric.io/max-failover-attempts: "3"
spec:
  checkpointing:
    enabled: true
    interval: 3600
  # ... rest of spec
```

When a cluster fails:
1. Jobs are detected as unhealthy
2. Latest checkpoint is retrieved
3. Job is rescheduled to healthy cluster
4. Training resumes from checkpoint

### Health Monitoring

```yaml
# Federation controller monitors:
- Cluster API availability
- GPU availability
- Storage accessibility
- Network connectivity

# Automatic actions on failure:
- Mark cluster as unavailable
- Trigger job failover
- Alert administrators
- Update capacity planning
```

## Cost Management

### Aggregated Cost Tracking

```bash
# View costs across all clusters
kfctl federation costs --days 30

# By cluster
kfctl federation costs --by-cluster

# By team
kfctl federation costs --by-team ml-research
```

### Cost Optimization

```yaml
# Federation controller optimizes for:
- Lowest GPU hourly rate
- Regional pricing differences
- Spot instance availability
- Volume discounts
```

### Budget Allocation

```yaml
apiVersion: kubefabric.io/v1
kind: FabricFederatedQuota
metadata:
  name: ml-research-federated
spec:
  team: ml-research
  aggregatedLimits:
    totalGPUHours: 5000
    maxConcurrentGPUs: 64
  budgets:
    total: 250000
    perCluster:
      cluster-a: 100000
      cluster-b: 100000
      cluster-c: 50000
  allocation:
    strategy: cost-optimized  # or balanced, performance
```

## Scheduling Strategies

### Balanced Distribution

```yaml
strategy: balanced
# Distributes jobs evenly across clusters
# Use for: fault tolerance, load balancing
```

### Cost-Optimized

```yaml
strategy: cost-optimized
# Schedules to lowest-cost cluster
# Use for: budget optimization, non-urgent workloads
```

### Performance-Optimized

```yaml
strategy: performance-optimized
# Schedules to fastest GPUs
# Use for: time-critical workloads, benchmarking
```

### Locality-Aware

```yaml
strategy: locality-aware
# Schedules near data sources
# Use for: large datasets, low-latency requirements
```

## Monitoring

### Unified Dashboard

```bash
# Access federation dashboard
kubectl port-forward -n kubefabric-system \
  svc/kubefabric-federation-ui 8080:80

# Open browser
open http://localhost:8080
```

**Dashboard shows:**
- All clusters status
- Aggregated GPU utilization
- Job distribution
- Cost breakdown
- Data replication status

### Metrics

```prometheus
# Total GPUs across federation
kubefabric_federation_total_gpus 144

# Available GPUs by cluster
kubefabric_federation_available_gpus{cluster="cluster-a"} 16
kubefabric_federation_available_gpus{cluster="cluster-b"} 32

# Jobs per cluster
kubefabric_federation_jobs{cluster="cluster-a",status="running"} 8

# Federated cost
kubefabric_federation_cost_total{team="ml-research"} 125432.50
```

## Network Configuration

### Cluster Mesh

```yaml
# Configure service mesh for cross-cluster communication
apiVersion: install.istio.io/v1alpha1
kind: IstioOperator
metadata:
  name: istio-federation
spec:
  meshConfig:
    serviceSettings:
      - settings:
          clusterLocal: false
        hosts:
          - "*.kubefabric-system.svc.cluster.local"
  values:
    global:
      meshID: kubefabric-mesh
      multiCluster:
        clusterName: cluster-a
      network: network-a
```

### VPN/Private Network

```yaml
# WireGuard VPN between clusters
apiVersion: v1
kind: ConfigMap
metadata:
  name: wireguard-config
data:
  wg0.conf: |
    [Interface]
    Address = 10.0.1.1/24
    PrivateKey = <private-key>

    [Peer]
    # cluster-b
    PublicKey = <public-key>
    Endpoint = cluster-b.example.com:51820
    AllowedIPs = 10.0.2.0/24
```

## Disaster Recovery

### Backup Strategy

```yaml
# Backup to multiple clusters
apiVersion: v1
kind: ConfigMap
metadata:
  name: backup-config
data:
  strategy: |
    replication:
      - source: cluster-a
        destinations:
          - cluster-b
          - cluster-c
      interval: 6h
      retention: 30d
```

### Recovery Procedure

```bash
# 1. Detect cluster failure
kfctl federation health

# 2. Trigger failover
kfctl federation failover --from cluster-a --to cluster-b

# 3. Verify job migration
kfctl federation jobs --cluster cluster-b

# 4. Restore cluster-a (when available)
kfctl federation restore cluster-a
```

## Best Practices

1. **Geographic Distribution**: Spread clusters across regions for DR
2. **Capacity Planning**: Maintain 20-30% spare capacity per cluster
3. **Data Replication**: Replicate critical datasets to multiple clusters
4. **Cost Monitoring**: Set up alerts for cross-cluster cost spikes
5. **Network Bandwidth**: Ensure adequate inter-cluster bandwidth
6. **Unified Quotas**: Use federated quotas for team budgets
7. **Regular Failover Tests**: Test failover procedures monthly

## Advanced Features

### Multi-Cluster ML Pipeline

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  name: federated-training
spec:
  entrypoint: multi-cluster-pipeline
  templates:
    - name: multi-cluster-pipeline
      steps:
        # Data prep on cluster-a (cheapest)
        - - name: data-prep
            template: data-prep-task
            arguments:
              parameters:
                - name: target-cluster
                  value: cluster-a

        # Parallel training on all clusters
        - - name: train-cluster-a
            template: training-task
            arguments:
              parameters:
                - name: target-cluster
                  value: cluster-a
                - name: shard
                  value: "0-3"

          - name: train-cluster-b
            template: training-task
            arguments:
              parameters:
                - name: target-cluster
                  value: cluster-b
                - name: shard
                  value: "4-7"

        # Merge results on cluster-a
        - - name: merge
            template: merge-task
```

### Cross-Cluster Model Serving

```yaml
# Deploy model across clusters for HA
apiVersion: serving.kserve.io/v1beta1
kind: FederatedInferenceService
metadata:
  name: llama-7b-federated
spec:
  clusters:
    - name: cluster-a
      weight: 50  # 50% traffic
      minReplicas: 2
    - name: cluster-b
      weight: 30  # 30% traffic
      minReplicas: 2
    - name: cluster-c
      weight: 20  # 20% traffic
      minReplicas: 1
  predictor:
    # ... predictor spec
```

## Troubleshooting

### Cluster Unreachable

```bash
# Check network connectivity
kfctl federation ping cluster-a

# Check credentials
kubectl get secret cluster-a-kubeconfig -n kubefabric-system

# Test API access
kubectl --kubeconfig=<path> get nodes
```

### Job Not Scheduling

```bash
# Check placement decision
kubectl describe fabricaijob <job-name> | grep -A 10 "Placement"

# Check cluster capacity
kfctl federation capacity

# View scheduler logs
kubectl logs -n kubefabric-system \
  deployment/kubefabric-federation-scheduler
```

### Data Sync Issues

```bash
# Check replication status
kubectl get fabricdataset imagenet -o yaml

# Force sync
kfctl federation sync-dataset imagenet

# Check data transfer logs
kubectl logs -n kubefabric-system \
  job/sync-imagenet-cluster-b
```

## Configuration Reference

See `multi-cluster/examples/` for:
- Federation controller deployment
- Cluster registration templates
- Scheduling policies
- Network mesh configuration
- Disaster recovery playbooks

## Support

- Federation Issues: https://github.com/ssahani/kube-fabric/issues
- Multi-cluster Guide: https://github.com/ssahani/kube-fabric/docs/multi-cluster
