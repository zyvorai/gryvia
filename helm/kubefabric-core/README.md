# KubeFabric Helm Chart

This Helm chart installs KubeFabric - a Kubernetes-native GPU Compute Fabric for AI workloads.

## Prerequisites

- Kubernetes 1.28+
- Helm 3.0+
- NVIDIA GPU nodes
- NVIDIA drivers installed (or use auto-install)

## Installing the Chart

```bash
# Add the KubeFabric Helm repository
helm repo add kubefabric https://kubefabric.ai/charts
helm repo update

# Install KubeFabric
helm install kubefabric kubefabric/kubefabric-core \
  --namespace kubefabric-system \
  --create-namespace

# Or install from local chart
helm install kubefabric ./helm/kubefabric-core \
  --namespace kubefabric-system \
  --create-namespace
```

## Configuration

The following table lists the configurable parameters and their default values.

| Parameter | Description | Default |
|-----------|-------------|---------|
| `gpuOperator.enabled` | Enable GPU Operator | `true` |
| `gpuOperator.image.repository` | GPU Operator image | `kubefabric/gpu-operator` |
| `gpuOperator.image.tag` | GPU Operator image tag | `1.0.0` |
| `aiOperator.enabled` | Enable AI Workload Operator | `true` |
| `aiOperator.image.repository` | AI Operator image | `kubefabric/ai-operator` |
| `aiOperator.image.tag` | AI Operator image tag | `1.0.0` |
| `nvidiaDevicePlugin.enabled` | Enable NVIDIA Device Plugin | `true` |
| `dcgmExporter.enabled` | Enable DCGM Exporter for metrics | `true` |
| `monitoring.enabled` | Enable monitoring stack | `true` |
| `rbac.create` | Create RBAC resources | `true` |

## Custom Values

Create a `custom-values.yaml` file:

```yaml
gpuOperator:
  enabled: true
  replicas: 1
  resources:
    limits:
      cpu: 1000m
      memory: 1Gi

aiOperator:
  enabled: true
  replicas: 2

monitoring:
  enabled: true
  prometheus:
    enabled: true
  grafana:
    enabled: true
```

Install with custom values:

```bash
helm install kubefabric kubefabric/kubefabric-core \
  --namespace kubefabric-system \
  --create-namespace \
  --values custom-values.yaml
```

## Upgrading

```bash
helm upgrade kubefabric kubefabric/kubefabric-core \
  --namespace kubefabric-system
```

## Uninstalling

```bash
helm uninstall kubefabric --namespace kubefabric-system
```

## Verifying the Installation

```bash
# Check operator pods
kubectl get pods -n kubefabric-system

# Check CRDs
kubectl get crds | grep kubefabric.ai

# Submit a test job
kubectl apply -f examples/simple-training-job.yaml
```
