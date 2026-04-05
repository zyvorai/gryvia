# KubeFabric Helm Chart

Official Helm chart for deploying KubeFabric GPU compute platform.

## TL;DR

```bash
helm repo add kubefabric https://ssahani.github.io/kube-fabric
helm install kubefabric kubefabric/kubefabric --namespace kubefabric --create-namespace
```

## Introduction

This chart bootstraps a complete KubeFabric deployment on a Kubernetes cluster using the Helm package manager.

## Prerequisites

- Kubernetes 1.28+
- Helm 3.8+
- GPU nodes with NVIDIA drivers installed
- cert-manager (if webhooks enabled)
- Prometheus Operator (if monitoring enabled)

## Installing the Chart

### Basic Installation

```bash
helm install kubefabric kubefabric/kubefabric \
  --namespace kubefabric \
  --create-namespace
```

### Production Installation

```bash
helm install kubefabric kubefabric/kubefabric \
  --namespace kubefabric \
  --create-namespace \
  --set highAvailability.enabled=true \
  --set prometheus.enabled=true \
  --set grafana.enabled=true \
  --set networkPolicy.enabled=true \
  --set webUI.ingress.enabled=true \
  --set webUI.ingress.hosts[0].host=kubefabric.example.com
```

### Custom Values

```bash
helm install kubefabric kubefabric/kubefabric \
  --namespace kubefabric \
  --create-namespace \
  --values custom-values.yaml
```

## Uninstalling the Chart

```bash
helm uninstall kubefabric --namespace kubefabric
```

This removes all Kubernetes components but keeps CRDs by default.

To remove CRDs:

```bash
kubectl delete crds \
  fabricgpunodes.kubefabric.ai \
  fabricaijobs.kubefabric.ai \
  fabricstorages.kubefabric.ai \
  fabricnetworks.kubefabric.ai \
  fabricquotas.kubefabric.ai
```

## Configuration

The following table lists the configurable parameters and their default values.

### Global Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `global.imageRegistry` | Global Docker image registry | `ghcr.io` |
| `global.imagePullSecrets` | Global image pull secrets | `[]` |
| `namespace` | Namespace to deploy to | `kubefabric` |

### Operator Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `gpuOperator.enabled` | Enable GPU Operator | `true` |
| `gpuOperator.replicaCount` | Number of replicas | `2` |
| `gpuOperator.image.repository` | Image repository | `ssahani/kubefabric-gpu-operator` |
| `gpuOperator.image.tag` | Image tag | `latest` |
| `aiOperator.enabled` | Enable AI Workload Operator | `true` |
| `aiOperator.replicaCount` | Number of replicas | `2` |
| `storageOperator.enabled` | Enable Storage Operator | `true` |
| `networkOperator.enabled` | Enable Network Operator | `true` |
| `quotaOperator.enabled` | Enable Quota Operator | `true` |

### Web UI Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `webUI.enabled` | Enable Web UI | `true` |
| `webUI.replicaCount` | Number of replicas | `2` |
| `webUI.service.type` | Service type | `ClusterIP` |
| `webUI.ingress.enabled` | Enable ingress | `false` |
| `webUI.ingress.className` | Ingress class | `nginx` |
| `webUI.ingress.hosts` | Ingress hosts | `[]` |

### Monitoring Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `prometheus.enabled` | Enable Prometheus | `true` |
| `grafana.enabled` | Enable Grafana | `true` |
| `dcgmExporter.enabled` | Enable DCGM Exporter | `true` |
| `metrics.enabled` | Enable metrics collection | `true` |

### Security Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `rbac.create` | Create RBAC resources | `true` |
| `networkPolicy.enabled` | Enable network policies | `true` |
| `podSecurityPolicy.enabled` | Enable PSP (deprecated) | `false` |

### High Availability

| Parameter | Description | Default |
|-----------|-------------|---------|
| `highAvailability.enabled` | Enable HA with leader election | `true` |

## Examples

### Minimal Installation (Single Node Testing)

```yaml
# values-minimal.yaml
gpuOperator:
  replicaCount: 1
aiOperator:
  replicaCount: 1
storageOperator:
  enabled: false
networkOperator:
  enabled: false
quotaOperator:
  replicaCount: 1
webUI:
  replicaCount: 1
apiGateway:
  replicaCount: 1
prometheus:
  enabled: false
grafana:
  enabled: false
highAvailability:
  enabled: false
```

```bash
helm install kubefabric kubefabric/kubefabric -f values-minimal.yaml
```

### Production Installation

```yaml
# values-production.yaml
global:
  imageRegistry: ghcr.io
  imagePullSecrets:
    - name: ghcr-credentials

gpuOperator:
  replicaCount: 3
  resources:
    limits:
      cpu: 1000m
      memory: 1Gi
    requests:
      cpu: 500m
      memory: 512Mi

aiOperator:
  replicaCount: 3
  webhook:
    enabled: true

highAvailability:
  enabled: true

webUI:
  replicaCount: 3
  ingress:
    enabled: true
    className: nginx
    hosts:
      - host: kubefabric.example.com
        paths:
          - path: /
            pathType: Prefix
    tls:
      - secretName: kubefabric-ui-tls
        hosts:
          - kubefabric.example.com

prometheus:
  enabled: true
  server:
    persistentVolume:
      enabled: true
      size: 100Gi
      storageClass: fast-ssd

grafana:
  enabled: true
  persistence:
    enabled: true
    size: 20Gi
  adminPassword: <secure-password>

networkPolicy:
  enabled: true

defaultQuotas:
  enabled: true
  quotas:
    - name: ml-research-quota
      team: ml-research
      namespaces:
        - ml-research
      gpuQuota:
        maxGPUs: 32
        maxGPUsPerJob: 8
        allowedGPUTypes:
          - H100
          - A100-80G
        maxRunningJobs: 20
      priority: 100
```

```bash
helm install kubefabric kubefabric/kubefabric -f values-production.yaml
```

### Air-Gapped Installation

```yaml
# values-airgap.yaml
global:
  imageRegistry: registry.internal.company.com
  imagePullSecrets:
    - name: internal-registry

gpuOperator:
  image:
    repository: kubefabric/gpu-operator
    tag: 1.0.0

aiOperator:
  image:
    repository: kubefabric/ai-operator
    tag: 1.0.0

# ... repeat for all components

prometheus:
  enabled: false  # Use existing Prometheus

grafana:
  enabled: false  # Use existing Grafana
```

## Upgrading

### Upgrade to Latest Version

```bash
helm repo update
helm upgrade kubefabric kubefabric/kubefabric --namespace kubefabric
```

### Upgrade with Custom Values

```bash
helm upgrade kubefabric kubefabric/kubefabric \
  --namespace kubefabric \
  --values custom-values.yaml \
  --reuse-values
```

## Troubleshooting

### Check Deployment Status

```bash
helm status kubefabric --namespace kubefabric
```

### View Release Values

```bash
helm get values kubefabric --namespace kubefabric
```

### Debug Installation

```bash
helm install kubefabric kubefabric/kubefabric \
  --namespace kubefabric \
  --dry-run \
  --debug
```

### Common Issues

#### Operators Not Starting

```bash
kubectl get pods -n kubefabric
kubectl describe pod <pod-name> -n kubefabric
kubectl logs <pod-name> -n kubefabric
```

#### CRDs Not Installing

```bash
# Manually install CRDs
kubectl apply -f https://raw.githubusercontent.com/ssahani/kube-fabric/main/crds/
```

#### Webhook Issues

```bash
# Check cert-manager
kubectl get certificates -n kubefabric
kubectl describe certificate <cert-name> -n kubefabric
```

## Dependencies

This chart has the following optional dependencies:

- **prometheus**: Monitoring and alerting
- **grafana**: Metrics visualization

Install with all dependencies:

```bash
helm install kubefabric kubefabric/kubefabric \
  --namespace kubefabric \
  --create-namespace \
  --set prometheus.enabled=true \
  --set grafana.enabled=true
```

## Contributing

Contributions are welcome! Please read the [contributing guidelines](https://github.com/ssahani/kube-fabric/blob/main/CONTRIBUTING.md).

## License

Apache 2.0 - See [LICENSE](https://github.com/ssahani/kube-fabric/blob/main/LICENSE)

## Support

- Documentation: https://github.com/ssahani/kube-fabric/docs
- Issues: https://github.com/ssahani/kube-fabric/issues
- Discussions: https://github.com/ssahani/kube-fabric/discussions
