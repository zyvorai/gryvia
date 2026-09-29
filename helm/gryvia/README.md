# Gryvia Helm chart

Installs Gryvia: the operators, the API gateway and the web dashboard.

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
| NVIDIA device plugin, DCGM exporter | `nvidiaDevicePlugin`, `dcgmExporter` | on (need NVIDIA GPU nodes) |

CRDs are installed from `crds/` on first install. Helm does not upgrade CRDs; apply new versions with
`kubectl apply --server-side -f crds/` before `helm upgrade`.

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

See `values.yaml` for every option. Uninstall with `helm uninstall gryvia -n gryvia-system`; CRDs and the
`gryvia-tls` Secret are kept.
