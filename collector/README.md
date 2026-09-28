# Gryvia Flow Collector

A Go-based userspace agent that loads eBPF programs into the kernel,
consumes flow events from perf ring buffers, and exports enriched
network telemetry as Prometheus metrics and a live service dependency
graph.

## Architecture

```
                                    +-----------------+
                                    |  Prometheus     |
                                    |  /metrics       |
                                    +--------+--------+
                                             ^
   +----------+     perf events    +---------+---------+
   |  Kernel  | -----------------> |  Flow Collector   |
   |  eBPF    |                    |                   |
   |  Programs|                    |  decoder          |
   +----------+                    |  aggregator       |
       ^                           |  graph builder    |
       |  kprobe/XDP/tracepoint    |  anomaly detector |
       |                           |  exporter         |
   +---+------+                    +---------+---------+
   |  Linux   |                              |
   |  Kernel  |                    +---------+---------+
   +----------+                    |  /api/v1/graph    |
                                   |  /api/v1/anomalies|
                                   +-------------------+
```

### Subsystems

| Package | Purpose |
|---------|---------|
| `pkg/loader` | Loads compiled eBPF `.o` files, attaches to kprobes/tracepoints/XDP, manages lifecycle |
| `pkg/decoder` | Reads raw perf events, binary-decodes `flow_event` structs, enriches with K8s metadata from `/proc` |
| `pkg/aggregator` | Sliding-window aggregation: p50/p95/p99 latency, bytes/sec, connections/sec per service pair |
| `pkg/graph` | Builds in-memory service dependency graph (nodes = services, edges = traffic flows) |
| `pkg/anomaly` | Statistical anomaly detection (EMA + stddev); detects latency spikes, traffic bursts, new connections |
| `pkg/exporter` | Prometheus metrics: `gryvia_network_flow_bytes_total`, `_latency_seconds`, `_connections_active`, `_drops_total`, `_dns_latency_seconds` |

## Metrics Reference

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `gryvia_network_flow_bytes_total` | Counter | src_service, dst_service, protocol, namespace | Total bytes transferred |
| `gryvia_network_latency_seconds` | Histogram | src_service, dst_service, protocol, namespace | Network latency |
| `gryvia_network_connections_active` | Gauge | src_service, dst_service, protocol, namespace | Active connections |
| `gryvia_network_drops_total` | Counter | src_service, dst_service, protocol, namespace | Dropped packets |
| `gryvia_network_dns_latency_seconds` | Histogram | namespace | DNS resolution latency |

## Building

```bash
# Build the collector binary
cd collector
go build -o gryvia-collector .

# Build the container image (includes eBPF compilation)
docker build -f collector/Dockerfile -t gryvia-collector .
```

## Deployment

The collector runs as a DaemonSet on every node in the cluster.  It
requires privileged access for eBPF operations.

```bash
# Create namespace
kubectl create namespace gryvia-system

# Deploy
kubectl apply -f collector/deploy/daemonset.yaml
```

### Prerequisites

- Kernel >= 5.8 (for BPF ring buffer and BTF support)
- BPF filesystem mounted at `/sys/fs/bpf`
- debugfs mounted at `/sys/kernel/debug`

### Configuration Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--metrics-addr` | `:9090` | Prometheus metrics listen address |
| `--ebpf-dir` | `/opt/gryvia/ebpf` | Directory containing compiled `.o` files |
| `--iface` | `eth0` | Network interface for XDP attachment |
| `--nats-url` | (empty) | NATS server URL for event streaming |
| `--window` | `300` | Aggregation window in seconds |

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/metrics` | GET | Prometheus metrics |
| `/api/v1/graph` | GET | Service dependency graph (JSON) |
| `/api/v1/anomalies` | GET | Recent anomalies (JSON) |
| `/healthz` | GET | Health check |
