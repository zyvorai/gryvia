# Gryvia Network Intelligence Helm Chart

Deploy the Gryvia Network Intelligence Operator with eBPF-based network monitoring for Kubernetes GPU clusters.

## Prerequisites

- Kubernetes >= 1.30
- Helm >= 3.0
- Linux kernel >= 5.8 (for eBPF support)
- Prometheus Operator (optional, for ServiceMonitor)

## Installation

### Add the Helm repository

```bash
helm repo add gryvia https://zyvorai.github.io/gryvia/charts
helm repo update
```

### Install the chart

```bash
helm install network-intelligence gryvia/gryvia-network-intelligence \
  --namespace gryvia-network \
  --create-namespace
```

### Install from local source

```bash
helm install network-intelligence ./helm/network-intelligence \
  --namespace gryvia-network \
  --create-namespace
```

## Configuration

### Key values

| Parameter | Description | Default |
|-----------|-------------|---------|
| `operator.image.repository` | Operator image | `gryvia/network-intelligence-operator` |
| `operator.replicas` | Operator replicas | `1` |
| `operator.resources` | Operator resource limits | `500m CPU, 512Mi memory` |
| `collector.image.repository` | Collector image | `gryvia/ebpf-collector` |
| `collector.hostNetwork` | Use host networking | `true` |
| `collector.resources` | Collector resource limits | `500m CPU, 512Mi memory` |
| `ebpf.enabled` | Run the eBPF collector DaemonSet | `false` |
| `ebpf.interface` | Interface for the XDP/TCX programs (empty: not attached) | `""` |
| `ebpf.cgroupPath` | cgroup v2 path for sockops/sk_msg (empty: not attached) | `""` |
| `ebpf.ncclLib`, `ebpf.cudaLib` | Library paths for the GPU uprobes (empty: discover) | `""` |
| `ebpf.publishFabricStatus` | Patch the status of existing `GryviaFabricSignal` objects every 30 s (`-publish-fabric-status`); adds `list` on `gryviafabricsignals` and `patch` on `gryviafabricsignals/status` to the collector ClusterRole | `false` |
| `ebpf.quotaPace.enabled` | Attach `quota_pace`, the one eBPF program that changes sockets (`-quota-pace`); needs `ebpf.cgroupPath`; alone it paces nothing | `false` |
| `ebpf.quotaPace.sync` | **Mutating.** Grant pace leases from `GryviaQuota` `spec.network.maxEgressMbps` (`-quota-pace-sync`); needs `ebpf.quotaPace.enabled`; adds `list` on `gryviaquotas` to the collector ClusterRole | `false` |
| `ebpf.quotaPace.dryRun` | Log what `sync` would do and write nothing (`-quota-pace-dry-run`); needs `ebpf.quotaPace.sync` | `false` |
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
kubectl delete namespace gryvia-network
```

## Architecture

The chart deploys two main components:

1. **Operator Deployment** - Manages network policies, processes flow data, and detects anomalies.
2. **Collector DaemonSet** - Runs on every node with privileged access to load eBPF programs that capture network flows, DNS queries, and packet drops.

## Monitoring

When `prometheus.serviceMonitor.enabled` is `true`, ServiceMonitor resources are created for both the operator and collector. Import the Grafana dashboards from `monitoring/grafana-dashboards/` for visualization.
