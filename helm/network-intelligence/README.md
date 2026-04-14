# TensorReaper Network Intelligence Helm Chart

Deploy the TensorReaper Network Intelligence Operator with eBPF-based network monitoring for Kubernetes GPU clusters.

## Prerequisites

- Kubernetes >= 1.30
- Helm >= 3.0
- Linux kernel >= 5.8 (for eBPF support)
- Prometheus Operator (optional, for ServiceMonitor)

## Installation

### Add the Helm repository

```bash
helm repo add tensorreaper https://charts.tensorreaper.ai
helm repo update
```

### Install the chart

```bash
helm install network-intelligence tensorreaper/tensorreaper-network-intelligence \
  --namespace tensorreaper-network \
  --create-namespace
```

### Install from local source

```bash
helm install network-intelligence ./helm/network-intelligence \
  --namespace tensorreaper-network \
  --create-namespace
```

## Configuration

### Key values

| Parameter | Description | Default |
|-----------|-------------|---------|
| `operator.image.repository` | Operator image | `tensorreaper/network-intelligence-operator` |
| `operator.replicas` | Operator replicas | `1` |
| `operator.resources` | Operator resource limits | `500m CPU, 512Mi memory` |
| `collector.image.repository` | Collector image | `tensorreaper/ebpf-collector` |
| `collector.hostNetwork` | Use host networking | `true` |
| `collector.resources` | Collector resource limits | `500m CPU, 512Mi memory` |
| `ebpf.enabled` | Enable eBPF programs | `true` |
| `ebpf.programs` | List of eBPF programs | See `values.yaml` |
| `prometheus.enabled` | Enable Prometheus metrics | `true` |
| `prometheus.serviceMonitor.enabled` | Create ServiceMonitor | `true` |
| `security.enabled` | Enable security monitoring | `true` |
| `security.autoBlock` | Auto-block suspicious traffic | `false` |

### Custom values example

```yaml
# custom-values.yaml
operator:
  replicas: 2
  resources:
    limits:
      cpu: 1000m
      memory: 1Gi

collector:
  resources:
    limits:
      cpu: 1000m
      memory: 1Gi

ebpf:
  programs:
    - name: tcp_connect
      enabled: true
    - name: tcp_close
      enabled: true
    - name: dns_monitor
      enabled: true
    - name: packet_drop
      enabled: true
    - name: nccl_monitor
      enabled: true

security:
  enabled: true
  autoBlock: true
```

```bash
helm install network-intelligence ./helm/network-intelligence -f custom-values.yaml
```

## Upgrading

```bash
helm upgrade network-intelligence ./helm/network-intelligence
```

## Uninstalling

```bash
helm uninstall network-intelligence
kubectl delete namespace tensorreaper-network
```

## Architecture

The chart deploys two main components:

1. **Operator Deployment** - Manages network policies, processes flow data, and detects anomalies.
2. **Collector DaemonSet** - Runs on every node with privileged access to load eBPF programs that capture network flows, DNS queries, and packet drops.

## Monitoring

When `prometheus.serviceMonitor.enabled` is `true`, ServiceMonitor resources are created for both the operator and collector. Import the Grafana dashboards from `monitoring/grafana-dashboards/` for visualization.
