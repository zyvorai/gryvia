# Quick Start Guide

Get up and running with KubeFabric in 10 minutes.

## Prerequisites

- Kubernetes cluster 1.28+
- kubectl configured
- Helm 3.0+
- GPU nodes with NVIDIA drivers

## Installation

### 1. Install KubeFabric via Helm

```bash
# Add Helm repository
helm repo add kubefabric https://ssahani.github.io/kube-fabric
helm repo update

# Install KubeFabric
helm install kubefabric kubefabric/kubefabric \
  --namespace kubefabric \
  --create-namespace \
  --wait
```

### 2. Verify Installation

```bash
# Check operators are running
kubectl get pods -n kubefabric

# Expected output:
# NAME                                      READY   STATUS    RESTARTS   AGE
# kubefabric-gpu-operator-xxx               1/1     Running   0          2m
# kubefabric-ai-operator-xxx                1/1     Running   0          2m
# kubefabric-storage-operator-xxx           1/1     Running   0          2m
# kubefabric-network-operator-xxx           1/1     Running   0          2m
# kubefabric-quota-operator-xxx             1/1     Running   0          2m
```

### 3. Install CLI

```bash
# Download CLI
curl -LO https://github.com/ssahani/kube-fabric/releases/latest/download/kfctl-linux-amd64

# Make executable and move to PATH
chmod +x kfctl-linux-amd64
sudo mv kfctl-linux-amd64 /usr/local/bin/kfctl

# Verify
kfctl version
```

## Register GPU Nodes

### 1. Label GPU Nodes

```bash
# Label nodes with GPU type
kubectl label nodes gpu-node-1 kubefabric.io/gpu-type=A100-80G
kubectl label nodes gpu-node-1 kubefabric.io/gpu-count=8
```

### 2. Create GPU Node Profile

```yaml
# gpu-node.yaml
apiVersion: kubefabric.io/v1
kind: FabricGPUNode
metadata:
  name: gpu-node-1
spec:
  gpuType: A100-80G
  gpuCount: 8
  memory: 512Gi
  nvlink: true
  infiniband: true
  pricing:
    hourlyRate: 24.00
```

```bash
kubectl apply -f gpu-node.yaml
```

### 3. Verify GPU Nodes

```bash
kfctl cluster nodes

# Expected output:
# NODE         GPU TYPE    COUNT   AVAILABLE   STATUS
# gpu-node-1   A100-80G    8       8           Ready
```

## Submit Your First Job

### 1. Create a Simple Training Job

```yaml
# training-job.yaml
apiVersion: kubefabric.io/v1
kind: FabricAIJob
metadata:
  name: pytorch-training
  namespace: default
spec:
  framework: pytorch
  resources:
    gpuType: A100-80G
    gpuCount: 1
    memory: 32Gi
    cpu: 16
  image: nvcr.io/nvidia/pytorch:24.01-py3
  command:
    - python
    - train.py
```

### 2. Submit Job

```bash
# Using kubectl
kubectl apply -f training-job.yaml

# Or using CLI
kfctl submit training-job.yaml
```

### 3. Monitor Job

```bash
# Get job status
kfctl status pytorch-training

# View logs
kfctl logs pytorch-training

# Watch progress
watch kfctl status pytorch-training
```

## Set Up Team Quota

### 1. Create Quota

```yaml
# team-quota.yaml
apiVersion: kubefabric.io/v1
kind: FabricQuota
metadata:
  name: ml-research
spec:
  team: ml-research
  limits:
    gpuHours: 1000
    maxGPUs: 16
  budget:
    monthly: 50000
  notifications:
    - type: email
      threshold: 80
      recipients:
        - team-lead@company.com
```

```bash
kubectl apply -f team-quota.yaml
```

### 2. Check Quota Usage

```bash
kfctl quota ml-research

# Expected output:
# TEAM         GPU HOURS   USED    REMAINING   BUDGET      SPENT
# ml-research  1000        245     755         $50,000     $12,450
```

## Access Web Dashboard

```bash
# Port-forward Web UI
kubectl port-forward -n kubefabric svc/kubefabric-ui 3000:3000

# Open browser
open http://localhost:3000
```

## Next Steps

### Learn More
- [Submit distributed training jobs](../user-guide/jobs.md#distributed-training)
- [Configure storage backends](../user-guide/storage.md)
- [Set up monitoring](../admin-guide/monitoring.md)
- [Use Argo Workflows](../examples/workflows/)

### Common Tasks
- [View cluster resources](../user-guide/gpus.md#viewing-resources)
- [Cancel jobs](../user-guide/jobs.md#cancelling-jobs)
- [Track costs](../user-guide/costs.md)
- [Debug failed jobs](../admin-guide/troubleshooting.md#job-failures)

### Integrations
- [JupyterHub](../examples/integrations/jupyterhub.md)
- [VSCode Server](../examples/integrations/vscode.md)
- [Ray Cluster](../examples/integrations/ray.md)

## Troubleshooting

### Pods Not Starting

```bash
# Check operator logs
kubectl logs -n kubefabric deployment/kubefabric-gpu-operator

# Check events
kubectl get events -n kubefabric
```

### GPU Not Detected

```bash
# Verify NVIDIA drivers
kubectl exec -it <gpu-pod> -- nvidia-smi

# Check GPU node labels
kubectl get nodes --show-labels | grep gpu
```

### Job Stuck in Pending

```bash
# Check scheduling decision
kubectl describe fabricaijob <job-name>

# Check available GPUs
kfctl cluster nodes
```

## Support

Need help?

- Check [Troubleshooting Guide](../admin-guide/troubleshooting.md)
- Ask in [GitHub Discussions](https://github.com/ssahani/kube-fabric/discussions)
- File an [Issue](https://github.com/ssahani/kube-fabric/issues)
