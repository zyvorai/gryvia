# Quick Start Guide

Get Gryvia running in minutes.

## Prerequisites

- Kubernetes cluster (K3s, K8s 1.30+, or EKS/GKE/AKS)
- `kubectl` configured and cluster accessible
- Node.js 20+ (for building the Web UI)
- Go 1.24+ (optional, for building operators from source)

## Option 1: One-Command Remote Deploy

Deploy to a single-node k3s server over SSH (key authentication; the SSH user needs `sudo`,
`podman`, `helm`, and access to the k3s cluster):

```bash
# Full deployment: rsync, build images on the host, CRDs, operators, API gateway, Web UI
./scripts/deploy-remote.sh <host> <user>        # or: user@host

# Skip the image builds (images already imported on the host)
./scripts/deploy-remote.sh <host> <user> --quick

# Re-run only the health and API checks
./scripts/deploy-remote.sh <host> <user> --verify-only
```

The script finishes with a smoke test (deployments, UI, API authentication, every dashboard
endpoint, and a custom-resource round trip) and prints the dashboard URL, `https://<host>:32443`. The UI and the API gateway serve HTTPS with a
self-signed certificate generated when the pod starts, so the browser shows a warning to accept once, and `curl` needs `-k`.

### Signing in

Open the dashboard and sign in as **`admin`** with password **`Admin@321`**. That is a
well-known lab default, so for anything reachable from an untrusted network deploy with your
own key. It becomes the `admin` password:

```bash
GRYVIA_API_KEY='a-long-random-secret' ./scripts/deploy-remote.sh <host> <user>
```

## Option 2: Manual Install

### 1. Install CRDs

```bash
kubectl apply -f crds/
```

Verify:
```bash
kubectl get crd | grep gryvia
# fabricaijobs.gryvia.io
# fabricgpunodes.gryvia.io
# fabricnetworks.gryvia.io
# fabricquotas.gryvia.io
# fabricstorages.gryvia.io
```

### 2. Create Namespace and Deploy Operators

```bash
kubectl create namespace gryvia-system

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
podman build --network=host -t gryvia/ui:1.0.0 -f docker/Dockerfile.ui .

# For K3s: import image
podman save gryvia/ui:1.0.0 -o /tmp/kf-ui.tar
k3s ctr images import /tmp/kf-ui.tar
k3s ctr images tag localhost/gryvia/ui:1.0.0 docker.io/gryvia/ui:1.0.0

# Deploy
kubectl apply -f manifests/deploy/ui-deployment.yaml

# Expose as NodePort
kubectl patch svc gryvia-ui -n gryvia-system \
  --type=merge -p '{"spec":{"type":"NodePort","ports":[{"port":80,"targetPort":80,"nodePort":30081}]}}'
```

### 4. Build and Deploy API Gateway

```bash
# Build
podman build --network=host -t gryvia/api-gateway:1.0.0 \
  -f services/api-gateway/Dockerfile services/api-gateway/

# Import into K3s
podman save gryvia/api-gateway:1.0.0 -o /tmp/kf-api.tar
k3s ctr images import /tmp/kf-api.tar
k3s ctr images tag localhost/gryvia/api-gateway:1.0.0 docker.io/gryvia/api-gateway:1.0.0

# Deploy
kubectl apply -f manifests/deploy/api-gateway-deployment.yaml

# Set API key
kubectl set env deployment/gryvia-api-gateway -n gryvia-system \
  GRYVIA_API_KEY=your-secure-key-here
```

### 5. Verify

```bash
kubectl get pods -n gryvia-system
# gryvia-api-gateway-xxx   1/1   Running
# gryvia-ui-xxx            1/1   Running

kubectl get svc -n gryvia-system
# gryvia-api-gateway   NodePort   8080:30088/TCP
# gryvia-ui            NodePort   80:30081/TCP
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
apiVersion: gryvia.io/v1
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
apiVersion: gryvia.io/v1
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
apiVersion: gryvia.io/v1
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
sudo cp target/release/gryvia /usr/local/bin/

# Usage
gryvia cluster
gryvia list jobs
gryvia quota --budget
gryvia cost --period month
```

## Uninstall

```bash
# Via deploy script
./scripts/deploy-remote.sh <host> <user> --uninstall

# Or manually
kubectl delete namespace gryvia-system
kubectl delete crd fabricaijobs.gryvia.io fabricgpunodes.gryvia.io \
  fabricnetworks.gryvia.io fabricquotas.gryvia.io fabricstorages.gryvia.io
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
kubectl set env deployment/gryvia-api-gateway -n gryvia-system \
  GRYVIA_API_KEY=your-key
```

### Web UI Shows "Failed to load cluster stats"

Check that the API gateway pods are running and the nginx proxy is configured:
```bash
kubectl get pods -n gryvia-system -l app=gryvia-api-gateway
kubectl logs -n gryvia-system deployment/gryvia-api-gateway
```

## Next Steps

- [Bare Metal Deployment Guide](../guides/DEPLOYMENT_GUIDE.md) - Full production setup
- [CLI Guide](../guides/CLI_GUIDE.md) - Command-line reference
- [API Reference](../developer-guide/api-reference.md) - REST API docs
- [Examples](https://github.com/zyvorai/gryvia/tree/main/examples/README.md) - Job templates and workflows
