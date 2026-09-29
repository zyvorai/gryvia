# Quick Start Guide

Get Gryvia running in minutes.

## Prerequisites

- `kubectl` and `helm` 3.14+
- A Kubernetes cluster 1.30+ (kind, k3s, EKS/GKE/AKS). GPU nodes are only needed to run real GPU work.

## Option 1: Try it on your laptop (no GPUs)

`scripts/kind-demo.sh` creates a [kind](https://kind.sigs.k8s.io) cluster, installs Gryvia and loads
**fictional** demo data (GPU nodes, a quota, a job) so the dashboard is populated:

```bash
git clone https://github.com/zyvorai/gryvia && cd gryvia
./scripts/kind-demo.sh
kubectl -n gryvia-system port-forward svc/gryvia-ui 8443:443
```

Open `https://localhost:8443`, accept the self-signed certificate warning, and sign in as
**`admin`** / **`Admin@321`**. Remove it with `./scripts/kind-demo.sh --delete`.

## Option 2: Install on your cluster with Helm

```bash
helm install gryvia oci://ghcr.io/zyvorai/charts/gryvia \
  --namespace gryvia-system --create-namespace \
  --set auth.apiKey='a-long-random-secret'
kubectl -n gryvia-system port-forward svc/gryvia-ui 8443:443
```

Or run `./scripts/install.sh`, which checks the prerequisites and prints how to open the dashboard.
The chart installs the operators, the API gateway and the dashboard. See the
[chart README](https://github.com/zyvorai/gryvia/tree/main/helm/gryvia) for exposing the dashboard
(NodePort, Ingress), TLS with cert-manager and other options.

### Signing in

Sign in as **`admin`**; the password is the API key you set with `auth.apiKey` (or `GRYVIA_API_KEY`
for the scripts). If you set nothing, the key is the well-known lab default **`Admin@321`**, so always
set your own for anything reachable from an untrusted network.

## Option 3: A fresh GPU server (k3s, drivers and Gryvia in one step)

On Ubuntu 22.04/24.04 with an NVIDIA GPU, as root:

```bash
sudo ./scripts/install-k3s-gpu.sh server
```

This installs k3s, Gryvia and NVIDIA's GPU Operator (driver, container toolkit, device plugin), then prints the
dashboard URL. See [GPU nodes](../guides/GPU_NODES.md) for options, extra nodes and troubleshooting.

## Option 4: Deploy to a remote k3s host over SSH

Deploy to a single-node k3s server over SSH (key authentication; the SSH user needs `sudo`,
`podman`, `helm`, and access to the k3s cluster). The script builds the images on the host and
installs the same chart:

```bash
# Full deployment: rsync, build images on the host, CRDs, operators, API gateway, dashboard
./scripts/deploy-remote.sh <host> <user>        # or: user@host

# Skip the image builds (only when the images were already built for this exact code)
./scripts/deploy-remote.sh <host> <user> --quick

# Re-run only the health and API checks
./scripts/deploy-remote.sh <host> <user> --verify-only
```

It finishes with a smoke test (deployments, dashboard, API authentication, every dashboard endpoint and a
custom-resource round trip) and prints the dashboard URL, `https://<host>:32443`. Use
`GRYVIA_API_KEY='...' ./scripts/deploy-remote.sh <host> <user>` to set your own key.

## Access the Dashboard

The dashboard follows your system light or dark setting and includes:
- Cluster overview, job pipeline (pending, running, completed, failed) and GPU utilization, each linking to the page behind it
- Jobs with search, filters, sorting, and per-job pods, logs and events
- Workspaces, models, inference services, workflows and the auto-tuner, with structured create forms
- Quotas, GPU nodes and costs, with drill-down links to the jobs involved
- Network flows and policies, security policies and GPU communication analysis, which say so when no collector is feeding them

## Register GPU Nodes

```yaml
# gpu-node.yaml
apiVersion: gryvia.io/v1alpha1
kind: GryviaGpuNode
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
kubectl get gryviagpunodes
```

## Submit Your First Job

```yaml
# training-job.yaml
apiVersion: gryvia.io/v1alpha1
kind: GryviaAIJob
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
kubectl get gryviaaijobs
```

## Set Up Team Quota

```yaml
# team-quota.yaml
apiVersion: gryvia.io/v1alpha1
kind: GryviaQuota
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
kubectl get gryviaquotas
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
kubectl delete crd gryviaaijobs.gryvia.io gryviagpunodes.gryvia.io \
  gryvianetworks.gryvia.io gryviaquotas.gryvia.io gryviastorages.gryvia.io
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
