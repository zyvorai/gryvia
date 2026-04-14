# Network Intelligence Examples

Example YAML manifests for TensorReaper network intelligence CRDs powered by eBPF and Cilium.

## Examples

- **[flow-policy.yaml](flow-policy.yaml)** - `FabricFlowPolicy` allowing payment-to-database traffic with low-latency intent
- **[auto-policy.yaml](auto-policy.yaml)** - `FabricAutoPolicy` in learn mode for 10 minutes, then suggesting policies
- **[trace-session.yaml](trace-session.yaml)** - `FabricTraceSession` debugging the payment service at L7 for 2 minutes
- **[service-graph.yaml](service-graph.yaml)** - `FabricServiceGraph` mapping service dependencies in production
- **[network-anomaly.yaml](network-anomaly.yaml)** - `FabricNetworkAnomaly` detecting latency spikes > 100ms with auto-mitigation
- **[traffic-insight.yaml](traffic-insight.yaml)** - `FabricTrafficInsight` analyzing payment service traffic patterns

## Usage

```bash
kubectl apply -f flow-policy.yaml
kubectl apply -f auto-policy.yaml
kubectl apply -f trace-session.yaml
kubectl apply -f service-graph.yaml
kubectl apply -f network-anomaly.yaml
kubectl apply -f traffic-insight.yaml
```
