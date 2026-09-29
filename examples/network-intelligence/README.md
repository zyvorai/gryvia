# Network Intelligence Examples

Example YAML manifests for Gryvia network intelligence CRDs powered by eBPF and Cilium.

> **Status.** The controllers are registered, but live Hubble and Prometheus data collection is not implemented, so insight, trace, graph and anomaly resources will not show measured data. `GryviaFlowPolicy` (creates a CiliumNetworkPolicy) requires Cilium. See `operators/network-intelligence/README.md`.

## Examples

- **[flow-policy.yaml](flow-policy.yaml)** - `GryviaFlowPolicy` allowing payment-to-database traffic with low-latency intent
- **[auto-policy.yaml](auto-policy.yaml)** - `GryviaAutoPolicy` in learn mode for 10 minutes, then suggesting policies
- **[trace-session.yaml](trace-session.yaml)** - `GryviaTraceSession` debugging the payment service at L7 for 2 minutes
- **[service-graph.yaml](service-graph.yaml)** - `GryviaServiceGraph` mapping service dependencies in production
- **[network-anomaly.yaml](network-anomaly.yaml)** - `GryviaNetworkAnomaly` detecting latency spikes > 100ms with auto-mitigation
- **[traffic-insight.yaml](traffic-insight.yaml)** - `GryviaTrafficInsight` analyzing payment service traffic patterns

## Usage

```bash
kubectl apply -f flow-policy.yaml
kubectl apply -f auto-policy.yaml
kubectl apply -f trace-session.yaml
kubectl apply -f service-graph.yaml
kubectl apply -f network-anomaly.yaml
kubectl apply -f traffic-insight.yaml
```
