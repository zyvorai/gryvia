# Complete Gryvia Setup Example

This directory contains a complete example setup for Gryvia with all components configured.

## Prerequisites

- Kubernetes cluster with GPU nodes
- kubectl configured
- Helm 3.x installed
- 3+ GPU nodes with NVIDIA drivers installed

## Quick Start

```bash
# 1. Install Gryvia
./deploy.sh

# 2. Verify installation
kubectl get pods -n gryvia-system

# 3. Submit example job
kubectl apply -f example-job.yaml

# 4. Check job status
kubectl get gryviaaijobs -n default
```

## What's Included

- **GPU Nodes**: 3 example GPU node configurations (H100, A100, L40)
- **Quotas**: 3 team quotas with budgets
- **Storage**: VAST Data parallel filesystem
- **Network**: RDMA/InfiniBand configuration
- **Jobs**: Example training jobs for PyTorch, TensorFlow, JAX
- **Monitoring**: Prometheus and Grafana dashboards

## Step-by-Step Deployment

### 1. Deploy Infrastructure Components

```bash
# Create namespace
kubectl create namespace gryvia-system

# Deploy CRDs
kubectl apply -f ../../crds/

# Deploy operators
kubectl apply -f ../../operators/gpu-operator/config/deployment.yaml
kubectl apply -f ../../operators/ai-operator/config/deployment.yaml
kubectl apply -f ../../operators/storage-operator/config/deployment.yaml
kubectl apply -f ../../operators/network-operator/config/deployment.yaml
kubectl apply -f ../../operators/quota-operator/config/deployment.yaml
```

### 2. Configure GPU Nodes

```bash
# Apply node configurations
kubectl apply -f gpu-nodes/
```

This creates GryviaGpuNode resources for:
- 2x H100 nodes (8 GPUs each)
- 2x A100-80G nodes (8 GPUs each)
- 1x L40 node (4 GPUs each)

### 3. Set Up Storage

```bash
# Deploy VAST storage backend
kubectl apply -f storage-config.yaml
```

Configures:
- 500TB VAST Data cluster
- CSI driver deployment
- StorageClass for dynamic provisioning

### 4. Configure Networking

```bash
# Deploy RDMA network
kubectl apply -f network-config.yaml
```

Sets up:
- InfiniBand RDMA
- Mellanox ConnectX-7 adapters
- 400Gb/s networking

### 5. Create Team Quotas

```bash
# Apply quota configurations
kubectl apply -f quotas/
```

Creates quotas for:
- **ML Research**: 32 GPUs, $50k/month budget
- **Computer Vision**: 16 GPUs, $30k/month budget
- **NLP**: 64 GPUs, $100k/month budget

### 6. Deploy Monitoring

```bash
# Install Prometheus and Grafana
kubectl apply -f ../../monitoring/
```

### 7. Deploy Web UI

```bash
# Deploy Web UI and API Gateway
kubectl apply -f ../../manifests/deploy/api-gateway-deployment.yaml
kubectl apply -f ../../manifests/deploy/ui-deployment.yaml
```

### 8. Submit Example Jobs

```bash
# Submit training jobs
kubectl apply -f jobs/
```

Example jobs:
- PyTorch distributed training (8x H100)
- TensorFlow single-node training (1x A100)
- JAX multi-node training (4x L40)

## Accessing the System

### Web UI

```bash
kubectl port-forward -n gryvia-system svc/gryvia-ui 8443:443
```

Open https://localhost:8443 (self-signed certificate; accept the browser warning)

### CLI

```bash
# Install CLI
curl -L https://github.com/zyvorai/gryvia/releases/latest/download/gryvia-linux-amd64 -o gryvia
chmod +x gryvia
sudo mv gryvia /usr/local/bin/

# Check cluster status
gryvia cluster

# List jobs
gryvia list jobs

# Get quota information
gryvia quota
```

### Grafana Dashboards

```bash
kubectl port-forward -n gryvia-system svc/prometheus-grafana 3000:80
```

Open http://localhost:3000
- Username: admin
- Password: (get from secret)

```bash
kubectl get secret -n gryvia-system prometheus-grafana -o jsonpath="{.data.admin-password}" | base64 -d
```

## Example Workflows

### Submit a Training Job

```yaml
apiVersion: gryvia.io/v1
kind: GryviaAIJob
metadata:
  name: llama-training
  namespace: default
  labels:
    team: nlp
spec:
  type: training
  distributed:
    framework: pytorch
    enabled: true
    nodes: 1
    gpusPerNode: 8
  gpus: 8
  gpuType: H100
  resources:
    requests:
      cpu: "64"
      memory: 512Gi
  image: nvcr.io/nvidia/pytorch:24.01-py3
  command:
    - torchrun
    - --nproc_per_node=8
    - train.py
    - --model=llama-70b
    - --batch-size=4
  env:
    - name: NCCL_DEBUG
      value: INFO
```

```bash
kubectl apply -f llama-training.yaml
gryvia status llama-training
gryvia logs llama-training
```

### Monitor GPU Utilization

```bash
# Real-time GPU metrics
watch kubectl get gryviagpunodes -o custom-columns=NAME:.metadata.name,TYPE:.spec.gpuType,GPUS:.spec.gpuCount,PHASE:.status.phase

# Detailed node metrics
kubectl describe gryviagpunode gpu-worker-01
```

### Check Team Quota Usage

```bash
# Via CLI
gryvia quota

# Via kubectl
kubectl get gryviaquotas

# Detailed quota info
kubectl describe gryviaquota ml-research-quota
```

## Troubleshooting

### Operators not starting

```bash
# Check operator logs
kubectl logs -n gryvia-system -l app=gryvia-gpu-operator --tail=50

# Check RBAC
kubectl auth can-i --list --as=system:serviceaccount:gryvia:gryvia-gpu-operator
```

### Jobs stuck in Pending

```bash
# Check quota limits
kubectl get gryviaquotas

# Check GPU availability
kubectl get gryviagpunodes

# Check job events
kubectl describe gryviaaijob <job-name>
```

### Storage not mounting

```bash
# Check storage operator
kubectl logs -n gryvia-system -l app=gryvia-storage-operator

# Check CSI driver
kubectl get pods -n gryvia-system | grep vast-csi

# Verify storage backend
kubectl get gryviastorage
```

## Cleanup

```bash
# Delete all jobs
kubectl delete gryviaaijobs --all

# Delete example resources
kubectl delete -f .

# Uninstall Gryvia
helm uninstall gryvia -n gryvia-system
kubectl delete namespace gryvia-system
```

## Production Considerations

1. **Security**
   - Replace credential placeholders in `production-deployment.yaml` (marked `<REPLACE_WITH_*>`) with real values or use a secrets manager (e.g., Vault, Sealed Secrets)
   - Enable RBAC for team isolation
   - Use network policies
   - Enable audit logging
   - Set up authentication (OIDC, LDAP)
   - Pin all container images to specific version tags (never use `:latest`)

2. **High Availability**
   - Run 3+ operator replicas
   - Use pod anti-affinity
   - Configure leader election
   - Set up backup/restore

3. **Monitoring**
   - Configure Alertmanager
   - Set up Slack/PagerDuty notifications
   - Enable audit logs
   - Monitor GPU health continuously

4. **Capacity Planning**
   - Monitor queue depths
   - Track quota utilization
   - Plan for peak usage
   - Budget for growth

5. **Cost Optimization**
   - Set appropriate quotas
   - Use spot instances where applicable
   - Monitor idle resources
   - Implement auto-scaling policies

## Support

- Documentation: https://github.com/zyvorai/gryvia
- Issues: https://github.com/zyvorai/gryvia/issues
- Discussions: https://github.com/zyvorai/gryvia/discussions
