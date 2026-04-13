# Quick Start Guide

Get TensorReaper running in minutes.

## Prerequisites

- Kubernetes cluster (K3s, K8s 1.30+, or EKS/GKE/AKS)
- `kubectl` configured and cluster accessible
- Node.js 20+ (for building the Web UI)
- Go 1.24+ (optional, for building operators from source)

## Option 1: One-Command Remote Deploy

Deploy to any server with a Kubernetes cluster:

```bash
# Full deployment (rsync + CRDs + operators + Web UI)
./scripts/deploy-remote.sh <host> <user> <password>

# Example
./scripts/deploy-remote.sh 185.165.240.5 root mypassword

# Quick mode (skip system deps, just rsync + kubectl apply)
./scripts/deploy-remote.sh 185.165.240.5 root mypassword --quick

# SSH key auth (no password)
./scripts/deploy-remote.sh 185.165.240.5 root --quick
```

Expected output:
```
  ✅ Synced to 185.165.240.5:/root/tensor-reaper
  ✅ Prerequisites checked
  ✅ CRDs installed
  ✅ Operators deployed
  ✅ API gateway and Web UI deployed
  ✅ Deployment verified
```

## Option 2: Manual Install

### 1. Install CRDs

```bash
kubectl apply -f crds/
```

Verify:
```bash
kubectl get crd | grep tensorreaper
# fabricaijobs.tensorreaper.ai
# fabricgpunodes.tensorreaper.ai
# fabricnetworks.tensorreaper.ai
# fabricquotas.tensorreaper.ai
# fabricstorages.tensorreaper.ai
```

### 2. Create Namespace and Deploy Operators

```bash
kubectl create namespace tensorreaper

# Deploy operators (storage, network, quota)
for op in storage-operator network-operator quota-operator; do
    kubectl apply -f operators/$op/config/deployment.yaml
done
```

### 3. Build and Deploy Web UI

```bash
# Build the UI
cd web-ui && npm install && npx vite build && cd ..

# Build container image (podman or docker)
podman build --network=host -t tensorreaper/ui:1.0.0 -f docker/Dockerfile.ui .

# For K3s: import image
podman save tensorreaper/ui:1.0.0 -o /tmp/kf-ui.tar
k3s ctr images import /tmp/kf-ui.tar
k3s ctr images tag localhost/tensorreaper/ui:1.0.0 docker.io/tensorreaper/ui:1.0.0

# Deploy
kubectl apply -f manifests/deploy/ui-deployment.yaml

# Expose as NodePort
kubectl patch svc tensorreaper-ui -n tensorreaper \
  --type=merge -p '{"spec":{"type":"NodePort","ports":[{"port":80,"targetPort":80,"nodePort":30081}]}}'
```

### 4. Build and Deploy API Gateway

```bash
# Build
podman build --network=host -t tensorreaper/api-gateway:1.0.0 \
  -f services/api-gateway/Dockerfile services/api-gateway/

# Import into K3s
podman save tensorreaper/api-gateway:1.0.0 -o /tmp/kf-api.tar
k3s ctr images import /tmp/kf-api.tar
k3s ctr images tag localhost/tensorreaper/api-gateway:1.0.0 docker.io/tensorreaper/api-gateway:1.0.0

# Deploy
kubectl apply -f manifests/deploy/api-gateway-deployment.yaml

# Set API key
kubectl set env deployment/tensorreaper-api-gateway -n tensorreaper \
  TENSORREAPER_API_KEY=your-secure-key-here
```

### 5. Verify

```bash
kubectl get pods -n tensorreaper
# tensorreaper-api-gateway-xxx   1/1   Running
# tensorreaper-ui-xxx            1/1   Running

kubectl get svc -n tensorreaper
# tensorreaper-api-gateway   NodePort   8080:30088/TCP
# tensorreaper-ui            NodePort   80:30081/TCP
```

## Access the Dashboard

Open your browser:
```
http://<server-ip>:30081
```

The dark-themed dashboard shows:
- GPU cluster overview with stat cards
- Job pipeline (pending, running, completed, failed)
- GPU utilization charts
- Node health indicators
- Quick action links

## Register GPU Nodes

```yaml
# gpu-node.yaml
apiVersion: tensorreaper.ai/v1
kind: FabricGpuNode
metadata:
  name: gpu-node-01
spec:
  nodeName: gpu-node-01
  gpuType: A100
  gpuCount: 8
  rdma: true
  interconnect: NVLink
  memoryGB: 80
```

```bash
kubectl apply -f gpu-node.yaml
kubectl get fabricgpunodes
```

## Submit Your First Job

```yaml
# training-job.yaml
apiVersion: tensorreaper.ai/v1
kind: FabricAIJob
metadata:
  name: pytorch-test
spec:
  type: training
  gpus: 1
  gpuType: any
  image: nvcr.io/nvidia/pytorch:24.01-py3
  command:
    - python
    - -c
    - "import torch; print(f'CUDA: {torch.cuda.is_available()}')"
```

```bash
kubectl apply -f training-job.yaml
kubectl get fabricaijobs
```

## Set Up Team Quota

```yaml
# team-quota.yaml
apiVersion: tensorreaper.ai/v1
kind: FabricQuota
metadata:
  name: ml-research
spec:
  team: ml-research
  namespaces:
    - ml-training
  gpuQuota:
    maxGPUs: 16
    maxRunningJobs: 10
    allowedGPUTypes:
      - H100
      - A100
  budget:
    monthlyBudget: 5000.00
    alertThreshold: 80.0
    hardLimit: false
```

```bash
kubectl apply -f team-quota.yaml
kubectl get fabricquotas
```

## Install CLI (Optional)

```bash
cd cli
cargo build --release
sudo cp target/release/tensorreaper /usr/local/bin/

# Usage
tensorreaper cluster
tensorreaper list jobs
tensorreaper quota --budget
tensorreaper cost --period month
```

## Uninstall

```bash
# Via deploy script
./scripts/deploy-remote.sh <host> <user> <password> --uninstall

# Or manually
kubectl delete namespace tensorreaper
kubectl delete crd fabricaijobs.tensorreaper.ai fabricgpunodes.tensorreaper.ai \
  fabricnetworks.tensorreaper.ai fabricquotas.tensorreaper.ai fabricstorages.tensorreaper.ai
```

## Troubleshooting

### Pods in ImagePullBackOff

Container images need to be built locally and imported into K3s:
```bash
podman build --network=host -t <image> -f <Dockerfile> .
podman save <image> -o /tmp/img.tar
k3s ctr images import /tmp/img.tar
```

### DNS Failures During Build

Use `--network=host` with podman/docker build:
```bash
podman build --network=host -t myimage .
```

### API Returns 401/403

Set the API key on the gateway deployment:
```bash
kubectl set env deployment/tensorreaper-api-gateway -n tensorreaper \
  TENSORREAPER_API_KEY=your-key
```

### Web UI Shows "Failed to load cluster stats"

Check that the API gateway pods are running and the nginx proxy is configured:
```bash
kubectl get pods -n tensorreaper -l app=tensorreaper-api-gateway
kubectl logs -n tensorreaper deployment/tensorreaper-api-gateway
```

## Next Steps

- [Bare Metal Deployment Guide](../DEPLOYMENT_GUIDE.md) - Full production setup
- [CLI Guide](../CLI_GUIDE.md) - Command-line reference
- [API Reference](../developer-guide/api-reference.md) - REST API docs
- [Examples](../../examples/README.md) - Job templates and workflows
