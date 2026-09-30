# Inference scaling and weighted canaries

## Scaling on GPU utilization and request rate

The existing `spec.autoscaling` targets now create `autoscaling/v2` HPA Pods metrics:

| Field | Custom metric | Units/target |
|---|---|---|
| `targetGPUUtilization` | `gryvia_gpu_utilization` | Average percentage per stable serving pod, 1-100 |
| `targetRequestsPerSecond` | `gryvia_inference_requests_per_second` | Average requests/second per stable serving pod, positive integer |

Both can be set. Kubernetes chooses the largest replica recommendation, subject to the configured min/max.
When neither is set, CPU utilization at 80% remains the default. Scale-down has a 300-second stabilization
window. Negative targets and GPU percentages above 100 produce `AutoscalingValid=False`, with no HPA.

**A custom metrics adapter is required.** Gryvia configures the HPA, not the metrics API server.
`examples/inference/prometheus-adapter-values.yaml` contains example adapter rules; check your actual
DCGM label names and scrape configuration before installing them. Every series must be attributable to
one namespace and pod. Use per-pod DCGM values; node-wide GPU utilization is unsuitable for tenant scaling.
The RPS rule assumes the serving application or proxy exports the illustrated request counter. These
rules do not add instrumentation to vLLM/Triton automatically. Missing series must stay missing, not
be rewritten to zero. The adapter and metrics pipeline need your usual access controls.

`AutoscalingValid=True` means configuration is valid. `AutoscalingReady` mirrors the HPA's current
`ScalingActive` condition, including failures to fetch custom metrics; stale/unobserved HPA status is
Unknown. It is not proof that every GPU metric is accurate or every desired replica is ready.
The HPA controls the stable Deployment only; the existing canary replica calculation is retained.

## Weighted HTTP routing

Enable the operator's optional capability:

```bash
helm upgrade gryvia ./helm/gryvia -n gryvia-system --reuse-values \
  --set aiOperator.inferenceGatewayRouting=true
```

This adds `--inference-gateway-routing=true` and HTTPRoute RBAC. Install the Gateway API v1 standard
CRDs and a compatible Gateway controller separately. Gryvia does not install either or create a Gateway.
No HTTPRoute API calls occur for default services that have never requested routing.

Opt a service in with annotations (full example in `examples/inference/weighted-canary.yaml`):

```yaml
metadata:
  annotations:
    gryvia.io/gateway-parent: public
    gryvia.io/gateway-section: https
    gryvia.io/gateway-hostname: chat.example.com
spec:
  canary:
    enabled: true
    weight: 10
    modelVersion: llama-v2
    autoPromote: true
    promoteAfterSeconds: 300
```

The Gateway must be in the service's namespace. Its listener must allow this HTTPRoute and, for
TLS, provide the appropriate certificate. The hostname and listener annotations are optional.
Gryvia creates `<name>-route`, `<name>-route-stable` and `<name>-route-canary` (long names are hashed).
The two track Services select exclusively stable or canary pods. Gateway backend weights total 100;
weight 10 produces 90/10 independently of replica counts. Gateway implementations distribute requests
statistically; a small request sample need not be exactly 90/10, and existing streaming requests are
not moved between backends when weights change.

An unready canary gets zero Gateway traffic. An old or incomplete Deployment rollout does not authorize traffic
to an updated version. `GatewayRouting=True` requires current-generation `Accepted=True` and
`ResolvedRefs=True` on the requested parent. Missing APIs, disabled routing and pending/rejected routes
are exposed through this condition. Accepted configuration is not a data-plane traffic measurement.

Use the **Gateway address and configured hostname** to get weighted traffic. The existing
`status.endpoint` remains the internal aggregate ClusterIP Service for compatibility; direct calls to
that Service keep the older pod-count traffic split. Gryvia does not infer the public Gateway address.

## Promotion and rollback

With routing requested, automatic promotion is blocked while the route is disabled, unaccepted, stale,
owned by another object, using different weights, or sending zero canary traffic. Once readiness and
current routing are observed together, a hold period of `promoteAfterSeconds` begins. Readiness loss,
route rejection, weight/parent/version or pod-template changes reset the hold period. It is sampled
at reconcile intervals, not continuously monitored. Deployment creation time alone is insufficient.

Promotion and the existing readiness-based rollback return the generated route to stable-only.
Latency/error-rate SLO evaluation is not implemented by this patch; a healthy readiness probe does not
prove model quality. Promotion still uses the existing Deployment rollout; there may be a period when
the stable endpoint serves the previous model during rollout. Client streaming/draining behavior depends
on the Gateway, server and pod termination settings.

To remove routing, clear all Gateway annotations while the capability is enabled. Gryvia deletes its
owned HTTPRoute first, then the track Services once the route is gone. Name collisions are left untouched.
To disable the operator capability globally, first remove per-service routes; switching the flag off stops
management and does not remove already-created routes. Deleting the inference resource garbage-collects
its owned route and Services.

## Validation and limits

Unit tests cover HPA targets/errors, status freshness, route weighting, cold/unready canaries, defaults,
cleanup, ownership and promotion gating. An `integration` Go test runs against envtest with Kubernetes
API defaults/validation and the official Gateway API HTTPRoute CRD. Its Gateway and HPA conditions are
explicit fixtures: no Gateway data plane, HPA controller, GPU, metrics adapter or model server is running.
A CI workflow installs the pinned test dependencies and runs the same checks.

Real-cluster acceptance requires observing HPA replica changes under load, request distributions through
your chosen Gateway, canary rollback and streaming/drain behavior with actual serving images.
