# Gryvia Helm chart

Installs Gryvia: the GPU, AI workload and quota operators (plus the storage and network operators when enabled), the
API gateway and the web dashboard, and optionally NVIDIA's GPU Operator. The sixth operator, network intelligence, and
the eBPF collector have their own chart, [`helm/network-intelligence`](../network-intelligence/README.md).

Gryvia is alpha software: the GPU path has not been run on real GPU hardware (see
[docs/gpu-validation.md](../../docs/gpu-validation.md)), and read [SECURITY.md](../../SECURITY.md) before exposing an install.

## Install

```bash
helm install gryvia oci://ghcr.io/zyvorai/charts/gryvia \
  --namespace gryvia-system --create-namespace
```

or run `./scripts/install.sh` (checks prerequisites and prints how to open the dashboard).
Trying it without GPUs: `./scripts/kind-demo.sh`.

Open the dashboard:

```bash
kubectl -n gryvia-system port-forward svc/gryvia-ui 8443:443   # https://localhost:8443
```

Sign in as `admin` with the API key. **The default key is the well-known lab value `Admin@321`.**
Set your own for anything reachable from an untrusted network:

```bash
helm upgrade --install gryvia oci://ghcr.io/zyvorai/charts/gryvia -n gryvia-system \
  --set auth.apiKey='a-long-random-secret'
# or keep it in your own Secret (key: GRYVIA_API_KEY)
  --set auth.existingSecret=my-gryvia-key
```

The certificate is self-signed by default; your browser shows a warning once.

## What is installed

| Component | Values key | Default |
|-----------|-----------|---------|
| GPU operator | `gpuOperator` | on |
| AI workload operator | `aiOperator` | on |
| Quota operator | `quotaOperator` | on |
| Storage operator | `storageOperator` | off |
| Network operator | `networkOperator` | off |
| API gateway (HTTPS, 2 replicas) | `apiGateway` | on |
| Web dashboard (HTTPS, 2 replicas) | `ui` | on |
| NVIDIA device plugin, DCGM exporter | `nvidiaDevicePlugin`, `dcgmExporter` | on, but only scheduled on nodes labelled `nvidia.com/gpu.present=true`; skipped when `nvidia.enabled=true` |
| NVIDIA GPU Operator sub-chart (driver, toolkit, device plugin, DCGM, feature discovery) | `nvidia` | off |

The quota operator also runs the `GryviaTenant` and `GryviaUsageRecord` controllers (GPU as a Service). Only some CRDs
have a controller: see the Controller column of the [CRD reference](../../website/docs/reference/crds.md).

CRDs are installed from `crds/` on first install. Helm does not upgrade CRDs; apply new versions with
`kubectl apply --server-side -f crds/` before `helm upgrade`.

## GPU nodes

`--set nvidia.enabled=true` installs NVIDIA's GPU Operator as a sub-chart (`nvidia.driver.enabled=false` when the
hosts already have drivers; `nvidia.toolkit.env` for k3s). The Gryvia GPU operator then registers a `GryviaGpuNode` for every node labelled by GPU feature discovery
(`gpuOperator.autoRegister`, on by default). `scripts/install-k3s-gpu.sh` wraps this for a fresh Ubuntu server. See
[GPU nodes](../../website/docs/guides/GPU_NODES.md). Not yet validated on real GPUs.

## API gateway options

| Value | Purpose |
|-------|---------|
| `auth.apiKey`, `auth.existingSecret` | The provider admin key (see above) |
| `apiGateway.oidc.adminGroups` | Comma-separated OIDC `groups` values that are provider admins; every other OIDC user is a tenant user |
| `apiGateway.oidc.legacyNamespaces` | Only while no `GryviaTenant` exists: trust the token claim as a namespace (old behaviour) |
| `apiGateway.netra.url`, `.tokenSecret`, `.tokenKey`, `.insecureTLS` | Take `/api/network/flows` from [Netra](https://github.com/zyvorai/netra) |
| `apiGateway.flightTokenSecret`, `.flightTokenKey`, `.flightCollectorNamespace` | Flight Recorder cluster view (needs the same token on the collector; see [docs/flight-recorder.md](../../docs/flight-recorder.md)) |
| `aiOperator.mergeFabricSignals` | Fold per-node entries of `GryviaFabricSignal` `status.nodes[]` into the top-level status (`--merge-fabric-signals`; default `false`; see `docs/fabric-status.md`) |
| `apiGateway.prometheusUrl` | Prometheus for cost and metric history |
| `apiGateway.metrics.enabled`, `.tokenSecret`, `.tokenKey` | Serve the gateway `/metrics` (off by default; needs an existing Secret with a token of at least 32 characters; never served unauthenticated) |
| `monitoring.enabled` (default `false`) | Install a PodMonitor for the operators, a ServiceMonitor for the gateway (only with `apiGateway.metrics.enabled`), a PrometheusRule and a Grafana dashboards ConfigMap. The monitors and the rule are skipped without the Prometheus Operator CRDs; see `monitoring.prometheus.*`, `monitoring.grafana.*` and [docs/observability.md](../../docs/observability.md) |

OIDC itself is switched on with the gateway's `OIDC_*` environment variables; see
[Authentication and TLS](../../website/docs/guides/AUTH_AND_TLS.md). Route-by-route access rules:
[API reference](../../website/docs/developer-guide/api-reference.md).

## Admission webhook

The ai-operator serves a validating admission webhook for `GryviaAIJob` (CREATE and UPDATE, all namespaces except
`kube-system`). It rejects a job with a clear message when: `spec.type` is not one of `training`, `inference`,
`fine-tuning`, `evaluation`; `spec.gpus` is not greater than 0; `spec.image` is empty; `metadata.name` is not a valid
DNS name; `spec.network` is not `standard`, `rdma` or `sriov`; `spec.priority` is outside 0-100; the `distributed`
block is inconsistent (nodes, framework, backend, more than 1024 GPUs); or CPU/memory requests are non-positive or
exceed their limits. On CREATE it also denies a job whose GPU type is not allowed by the quotas covering its namespace
or by the tenant's catalog SKUs, or that exceeds the per-job GPU limit; if the quotas or tenant cannot be read the job is
admitted (fails open) and the quota operator's reactive enforcement applies. Checks against live node capacity or GPU
labels are only returned as warnings, never denials.

The chart creates the `<release>-webhook` Service (443 to 9443), a `<release>-webhook-tls` Secret (own CA plus a
serving certificate, generated once and reused on upgrades; independent of `tls.mode`), and a
`ValidatingWebhookConfiguration` with the CA injected as `caBundle`.

| Value | Purpose |
|-------|---------|
| `webhook.enabled` | `true` by default; `false` removes the webhook objects and the operator's extra port, volume and flag |
| `webhook.failurePolicy` | `Ignore` (default): if the operator is down, jobs are still admitted. `Fail`: jobs are rejected while the webhook is unreachable (hardening) |
| `webhook.timeoutSeconds` | Admission call timeout (default 5) |

To rotate the certificate, delete the `<release>-webhook-tls` Secret and run `helm upgrade`, then restart the
ai-operator.

## Common settings

| Value | Purpose |
|-------|---------|
| `global.imageRegistry` / `global.imageTag` | Registry prefix (`ghcr.io/zyvorai`, mirrored as `docker.io/zyvorai`) and tag (default: appVersion) |
| `ui.service.type` / `ui.service.nodePort` | Expose the dashboard (`NodePort`, `LoadBalancer`) |
| `ui.ingress.*` | Ingress with optional TLS secret; add `nginx.ingress.kubernetes.io/backend-protocol: HTTPS` for ingress-nginx |
| `tls.mode` | `selfSigned` (default), `certManager` (needs `tls.certManager.issuerName`), or `existingSecret` |
| `tls.extraSANs` | Extra DNS names or IPs for the self-signed certificate |
| `podDisruptionBudget.enabled`, `networkPolicy.enabled` | Optional hardening |
| `apiGateway.prometheusUrl` | Prometheus for cost and metric history |

The keys `ha.enabled`, `crds.install`, `crds.keep`, `gpuOperator.healthCheck.*`, `quotaOperator.pricing.*` and the
top-level `labels` and `annotations` were removed because no template read them (GPU prices come from `GryviaGpuSku`
objects, or a table built into the operator when there are none); setting them was already a no-op. The chart deploys no
database and no Prometheus (use `helm/observability`). `scripts/check-chart-values.py` fails CI when a value is not read.

See `values.yaml` for every option. Uninstall with `helm uninstall gryvia -n gryvia-system`; CRDs and the
`gryvia-tls` Secret are kept.
