# Gryvia Network Intelligence Helm Chart

Deploys the Gryvia network-intelligence operator and, optionally, the experimental eBPF collector DaemonSet. This is the
sixth Gryvia operator; the other five are installed by [`helm/gryvia`](../gryvia/README.md). The two charts are
independent: neither requires the other.

**Status: experimental.**

- The operator reconciles ten kinds (`GryviaFlowPolicy`, `GryviaTrafficInsight`, `GryviaAutoPolicy`,
  `GryviaTraceSession`, `GryviaServiceGraph`, `GryviaNetworkAnomaly`, `GryviaSecurityPolicy`, `GryviaNetworkCost`,
  `GryviaTrainingInsight`, `GryviaInferenceInsight`). Several of them read data from Hubble, Prometheus or the collector; see
  the [operator README](../../operators/network-intelligence/README.md) for which paths are real.
- The collector (`ebpf.enabled`) is **off by default**, runs privileged with `hostNetwork` and `hostPID`, and its image
  (`gryvia-ebpf-collector`) is built in CI but **not published by the release workflow**: build it from
  `collector/Dockerfile` and set `collector.image.repository`/`tag`. It was verified on Linux 7.0 x86_64 only; GPU, RDMA,
  arm64 and the gated attachments are unverified on hardware. See [collector/README.md](../../collector/README.md) and
  [ebpf/README.md](../../ebpf/README.md).
- Known issue found by reading the code, not yet run against a cluster: the operator Deployment passes
  `--security-enabled` and `--security-auto-block` (from `security.enabled` and `security.autoBlock`), but
  `operators/network-intelligence/main.go` defines no such flags, so Go's flag parser rejects them and the operator
  container exits at start-up. Until the chart and the operator agree, treat `security.*` as non-functional and expect
  to remove those two arguments from `templates/operator-deployment.yaml` (or add the flags to the operator).
- Known issue (code reading, not run): the operator's controllers call the collector at the hard-coded
  `http://gryvia-collector.gryvia-system.svc.cluster.local:9090`, but this chart creates no Service with that name (the
  collector is in `gryvia-network`), and the operator asks for some paths the collector does not serve. Treat the
  operator-to-collector data path as not working.
- Flow data can also come from [Netra](https://github.com/zyvorai/netra) through the gateway
  (`apiGateway.netra.url` in the main chart), which needs neither this collector nor its privileges.

## Prerequisites

- Kubernetes 1.30 or newer (as the main chart), Helm 3
- For the collector: a Linux kernel with BTF (`/sys/kernel/btf/vmlinux`); Linux 6.6 or newer for the TCX programs
- Prometheus Operator (optional, for the ServiceMonitors)

## Installation

```bash
# published chart
helm install network-intelligence oci://ghcr.io/zyvorai/charts/gryvia-network-intelligence \
  --namespace gryvia-network --create-namespace

# or from a checkout
helm install network-intelligence ./helm/network-intelligence \
  --namespace gryvia-network --create-namespace
```

The chart is also in the classic repository (`helm repo add gryvia https://zyvorai.github.io/gryvia/charts`), where its
name is `gryvia-network-intelligence`.

## Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `operator.image.repository` / `.tag` | Operator image | `ghcr.io/zyvorai/gryvia-network-intelligence-operator`, chart appVersion |
| `operator.replicas`, `operator.resources` | Operator size | `1`, 100m/128Mi requests, 500m/512Mi limits |
| `ebpf.enabled` | Run the eBPF collector DaemonSet | `false` |
| `collector.image.repository` / `.tag` | Collector image (not published; build your own) | `ghcr.io/zyvorai/gryvia-ebpf-collector` |
| `collector.hostNetwork` | Host networking for the collector; the pod's port 9090 is then bound on the node | `true` |
| `ebpf.interface` | Interface for the XDP/TCX programs (`-iface`); empty means they are not attached | `""` |
| `ebpf.cgroupPath` | cgroup v2 path for sockops/sk_msg (`-cgroup-path`); empty means not attached | `""` |
| `ebpf.ncclLib`, `ebpf.cudaLib` | Library paths for the GPU uprobes; empty means auto-discover | `""` |
| `ebpf.flightTokenSecret`, `ebpf.flightTokenKey` | Secret holding the Flight Recorder token (`-flight-token-file`); empty disables that endpoint | `""`, `token` |
| `prometheus.serviceMonitor.enabled` | Create ServiceMonitors (only when the Prometheus Operator CRD exists) | `true` |
| `security.enabled`, `security.autoBlock` | See the known issue above | `true`, `false` |
| `namespace.name`, `namespace.create` | Target namespace | `gryvia-network`, `true` |
| `ha.leaderElection` | Operator leader election | `true` |

The collector runs as root and privileged (a requirement of loading eBPF programs); its port 9090 serves unauthenticated
endpoints except the Flight Recorder route. Do not expose it outside the cluster operators: restrict ingress with a
NetworkPolicy or use node firewall rules, since a pod NetworkPolicy does not apply with `hostNetwork: true`. Details
and a policy example are in [docs/flight-recorder.md](../../docs/flight-recorder.md).

The program list is not configurable per program: the collector loads every `.o` in its image and attaches what it can.
Check which ones attached on a node:

```bash
kubectl -n gryvia-network port-forward pod/<collector-pod> 9090:9090
curl -s localhost:9090/api/v1/ebpf/status
```

## Upgrading and uninstalling

```bash
helm upgrade network-intelligence ./helm/network-intelligence --namespace gryvia-network
helm uninstall network-intelligence --namespace gryvia-network
```

The CRDs come from `helm/gryvia` (or `crds/`), not from this chart, and are kept on uninstall.

## Monitoring

With `prometheus.serviceMonitor.enabled=true` and the Prometheus Operator installed, ServiceMonitors are created for the
operator (`:8080/metrics`) and the collector (`:9090/metrics`). `monitoring/grafana-dashboards/` holds dashboard
templates; most metric names they query are not produced by any Gryvia component yet (see `monitoring/README.md`).
