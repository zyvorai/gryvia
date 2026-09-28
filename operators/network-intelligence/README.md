# NetPredator - Network Intelligence Layer

NetPredator is the network intelligence layer for the Gryvia GPU platform. It provides Cilium/eBPF-based network visibility, security, and traffic control through six Kubernetes custom resources.

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

### FabricFlowPolicy (ffp)

Intent-based network policy that translates high-level traffic intents into CiliumNetworkPolicies.

**Intents:**
- `low-latency` - Sets DSCP EF marking and high priority
- `high-throughput` - Optimizes for bulk data transfer
- `secure` - Enables WireGuard encryption via Cilium
- `default` - Standard traffic handling

```yaml
apiVersion: gryvia.io/v1
kind: FabricFlowPolicy
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

### FabricTrafficInsight (fti)

Real-time traffic analysis with percentile latencies, throughput measurements, drop rate tracking, top talker identification, and anomaly detection.

```yaml
apiVersion: gryvia.io/v1
kind: FabricTrafficInsight
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

### FabricAutoPolicy (fap)

Self-healing firewall that learns traffic patterns and automatically generates network policies.

**Modes:**
- `learn` - Observe traffic patterns via Hubble flow logs
- `suggest` - Generate CiliumNetworkPolicy suggestions with confidence scores
- `enforce` - Apply suggested policies (with optional approval gate)

```yaml
apiVersion: gryvia.io/v1
kind: FabricAutoPolicy
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

### FabricTraceSession (fts)

Time-limited network trace/debug sessions with flow capture stored in ConfigMaps.

**Trace levels:**
- `l3` - IP-level flow capture
- `l4` - TCP/UDP connection tracking
- `l7` - Application-layer protocol inspection (HTTP headers with `captureHeaders: true`)

```yaml
apiVersion: gryvia.io/v1
kind: FabricTraceSession
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

### FabricServiceGraph (fsg)

Service dependency graph built from Hubble flow data with health status, latency, and throughput per edge.

```yaml
apiVersion: gryvia.io/v1
kind: FabricServiceGraph
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

### FabricNetworkAnomaly (fna)

Network anomaly detection with threshold-based rules, webhook alerting, and automatic mitigation.

**Detection capabilities:**
- Latency spikes
- Traffic bursts
- Connection storms
- Unusual port activity
- Elevated error/drop rates

**Auto-mitigation:** Creates temporary CiliumNetworkPolicy deny rules for critical/high severity anomalies, with automatic expiration after 15 minutes.

```yaml
apiVersion: gryvia.io/v1
kind: FabricNetworkAnomaly
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

- Kubernetes cluster with Cilium CNI
- Hubble observability enabled in Cilium
- Prometheus for metrics collection (optional, enhances TrafficInsight and NetworkAnomaly)

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
