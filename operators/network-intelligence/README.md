# NetPredator - Network Intelligence Layer

NetPredator is the network intelligence layer for the Gryvia GPU platform. It provides Cilium/eBPF-based network visibility, security, and traffic control through Kubernetes custom resources. The operator registers ten controllers; this page details the first six (`GryviaSecurityPolicy`, `GryviaNetworkCost`, `GryviaTrainingInsight` and `GryviaInferenceInsight` are also registered but not described here).

> **Status.** The CRDs and controllers are registered and unit-tested, but the data-collection side is largely a stub. Only the Kubernetes-side actions are real: `GryviaFlowPolicy` builds and applies a CiliumNetworkPolicy, `GryviaAutoPolicy` has learn/suggest/enforce state handling, and `GryviaNetworkAnomaly` can create a temporary deny CiliumNetworkPolicy. The Hubble relay gRPC and Prometheus PromQL queries are **not implemented** (the code only checks whether the `hubble-relay` / `prometheus-server` services exist and otherwise preserves existing status values). As a result `GryviaTrafficInsight` does not measure latency or throughput, `GryviaTraceSession` does not capture flows, `GryviaServiceGraph` does not discover edges from live traffic, and `GryviaNetworkAnomaly` currently sees empty metrics. Nothing here has been verified on a cluster running Cilium and Hubble.

## Architecture

```
+------------------------------------------------------------------+
|                    NetPredator Operator                           |
|                                                                  |
|  +------------------+  +---------------------+  +--------------+ |
|  | FlowPolicy       |  | TrafficInsight      |  | AutoPolicy   | |
|  | Controller       |  | Controller          |  | Controller   | |
|  |                  |  |                     |  |              | |
|  | Intent -> Cilium |  | Prometheus/Hubble   |  | Learn ->     | |
|  | Policy mapping   |  | metrics collection  |  | Suggest ->   | |
|  |                  |  | anomaly detection   |  | Enforce      | |
|  +--------+---------+  +----------+----------+  +------+-------+ |
|           |                       |                     |         |
|  +--------+---------+  +----------+----------+  +------+-------+ |
|  | TraceSession     |  | ServiceGraph        |  | Network      | |
|  | Controller       |  | Controller          |  | Anomaly      | |
|  |                  |  |                     |  | Controller   | |
|  | Time-limited     |  | Dependency graph    |  |              | |
|  | flow capture     |  | from Hubble flows   |  | Threshold    | |
|  | via Hubble       |  | with health status  |  | detection +  | |
|  |                  |  |                     |  | auto-mitigate| |
|  +------------------+  +---------------------+  +--------------+ |
|                                                                  |
+----+--------------------+---------------------+------------------+
     |                    |                     |
     v                    v                     v
+----------+     +-----------------+    +----------------+
| Cilium   |     | Hubble Relay    |    | Prometheus     |
| CNI/eBPF |     | (gRPC API)     |    | (PromQL)       |
+----------+     +-----------------+    +----------------+
```

## Custom Resources

### GryviaFlowPolicy

Intent-based network policy that translates high-level traffic intents into CiliumNetworkPolicies.

**Intents:**
- `low-latency` - Sets DSCP EF marking and high priority
- `high-throughput` - Optimizes for bulk data transfer
- `secure` - Enables WireGuard encryption via Cilium
- `default` - Standard traffic handling

```yaml
apiVersion: gryvia.io/v1alpha1
kind: GryviaFlowPolicy
metadata:
  name: gpu-training-data
  namespace: ml-training
spec:
  source:
    service: data-loader
    namespace: ml-training
    labels:
      app: data-loader
  destination:
    service: model-server
    namespace: ml-training
    port: 8080
    labels:
      app: model-server
  protocol: tcp
  action: allow
  intent: high-throughput
  priority: 100
```

### GryviaTrafficInsight

Traffic analysis (design intent; live metric collection is not implemented, see Status) with percentile latencies, throughput measurements, drop rate tracking, top talker identification, and anomaly detection.

```yaml
apiVersion: gryvia.io/v1alpha1
kind: GryviaTrafficInsight
metadata:
  name: model-server-insight
  namespace: ml-training
spec:
  service: model-server
  namespace: ml-training
  window: "5m"
  metrics:
    - latency
    - throughput
    - drops
    - retransmits
```

### GryviaAutoPolicy

Self-healing firewall that learns traffic patterns and automatically generates network policies.

**Modes:**
- `learn` - Observe traffic patterns via Hubble flow logs
- `suggest` - Generate CiliumNetworkPolicy suggestions with confidence scores
- `enforce` - Apply suggested policies (with optional approval gate)

```yaml
apiVersion: gryvia.io/v1alpha1
kind: GryviaAutoPolicy
metadata:
  name: ml-namespace-autopolicy
  namespace: ml-training
spec:
  mode: learn
  learningWindow: "24h"
  targetNamespaces:
    - ml-training
    - ml-inference
  excludeServices:
    - kube-dns
    - metrics-server
  approvalRequired: true
```

### GryviaTraceSession

Time-limited network trace/debug sessions. Session lifecycle and the results ConfigMap are managed, but flow capture from Hubble is not implemented yet.

**Trace levels:**
- `l3` - IP-level flow capture
- `l4` - TCP/UDP connection tracking
- `l7` - Application-layer protocol inspection (HTTP headers with `captureHeaders: true`)

```yaml
apiVersion: gryvia.io/v1alpha1
kind: GryviaTraceSession
metadata:
  name: debug-model-server
  namespace: ml-training
spec:
  service: model-server
  namespace: ml-training
  duration: "10m"
  level: l4
  filters:
    port: 8080
    protocol: tcp
  captureHeaders: false
```

### GryviaServiceGraph

Service dependency graph intended to be built from Hubble flow data (not yet implemented; existing edges are preserved) with health status, latency, and throughput per edge.

```yaml
apiVersion: gryvia.io/v1alpha1
kind: GryviaServiceGraph
metadata:
  name: ml-platform-graph
  namespace: ml-training
spec:
  namespaces:
    - ml-training
    - ml-inference
    - data-pipeline
  refreshInterval: "30s"
  includeExternal: true
  depth: 3
```

### GryviaNetworkAnomaly

Network anomaly detection with threshold-based rules, webhook alerting, and automatic mitigation.

**Detection capabilities:**
- Latency spikes
- Traffic bursts
- Connection storms
- Unusual port activity
- Elevated error/drop rates

**Auto-mitigation:** For critical/high severity anomalies, creates temporary CiliumNetworkPolicy deny rules, with automatic expiration after 15 minutes.

```yaml
apiVersion: gryvia.io/v1alpha1
kind: GryviaNetworkAnomaly
metadata:
  name: model-server-anomaly-detector
  namespace: ml-training
spec:
  targetService: model-server
  detectionRules:
    - metric: latency
      operator: gt
      threshold: 500
      window: "5m"
    - metric: drops
      operator: gt
      threshold: 100
      window: "1m"
    - metric: connections
      operator: gt
      threshold: 10000
      window: "5m"
  alertWebhook: "https://alerts.example.com/network"
  autoMitigate: true
```

## Prerequisites

- Kubernetes cluster with Cilium CNI (needed for CiliumNetworkPolicy resources)
- Hubble observability enabled in Cilium (checked for, not yet queried)
- Prometheus (checked for, not yet queried)

## Building

```bash
make build       # Build the operator binary
make test        # Run tests
make docker-build # Build Docker image
make docker-push  # Push Docker image
```

## Deploying

```bash
make deploy      # Apply CRDs and deploy the operator
make undeploy    # Remove the operator deployment
```

Note: `make deploy` applies `config/`, which does not exist in this directory, so it will fail as is. Use the Helm chart in `helm/network-intelligence` (or the main `helm/gryvia` chart) instead.
