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
Opt-in error-rate and latency evaluation is described below. A healthy readiness probe does not prove
model quality. Promotion still uses the existing Deployment rollout; there may be a period when
the stable endpoint serves the previous model during rollout. Client streaming/draining behavior depends
on the Gateway, server and pod termination settings.

To remove routing, clear all Gateway annotations while the capability is enabled. Gryvia deletes its
owned HTTPRoute first, then the track Services once the route is gone. Name collisions are left untouched.
To disable the operator capability globally, first remove per-service routes; switching the flag off stops
management and does not remove already-created routes. Deleting the inference resource garbage-collects
its owned route and Services.

## Measured canary SLO evaluation

Configure the operator's Prometheus base URL with `aiOperator.inferencePrometheusURL` in Helm or
`--inference-prometheus-url=http://prometheus.monitoring:9090`. The default is empty. The administrator
chooses this endpoint; services cannot supply endpoints or arbitrary PromQL. Use a trusted Prometheus
or query proxy reachable by the operator. URL credentials, query parameters and redirects are rejected;
authentication headers and custom CA configuration are not provided by this feature.

Add both `gryvia.io/canary-max-error-rate` (a fraction from 0 to 1) and
`gryvia.io/canary-max-p95-seconds` (positive seconds) to the inference service. Optional
`gryvia.io/canary-min-requests` defaults to 100 requests. Invalid or partial policies fail configuration
validation. `examples/inference/slo-canary.yaml` includes a complete service example.

Instrumentation must provide these normalized metrics, with **all** of the following labels:
`namespace`, `inference` (service name), `track="canary"`, `model_version`, `deployment_uid` (canary
Deployment UID), and `revision` (the Deployment's `gryvia.io/spec-hash` annotation). The histogram also
needs `le`. A serving proxy/exporter can obtain these values from Kubernetes metadata. No backend
instrumentation or automatic label enrichment is installed by Gryvia.

| Metric | Required meaning |
|---|---|
| `gryvia_inference_requests_total` | Counter of completed requests, including failed requests |
| `gryvia_inference_errors_total` | Counter of failed requests; export zero before the first failure |
| `gryvia_inference_request_duration_seconds_bucket` | Cumulative classic histogram of completed request durations in seconds |

Every serving replica must be represented; duplicate scrapes or incomplete series distort analysis.
Use the same request population for all three metrics and define which responses count as errors in
your instrumentation. The operator sums `increase` of request/error counters over 60 seconds and
computes p95 from summed histogram bucket rates. Counter resets are handled by Prometheus. Counts
are extrapolated estimates and can be fractional. A zero-traffic window has no usable error ratio;
missing errors or histogram data must stay missing, rather than being filled with synthetic success.
This is completed-request latency, not token latency or time to first token.

`CanaryAnalysisReady` reports `SLOPassed`, `SLOBreached`, `WarmingUp`, `InsufficientTraffic`,
`NotConfigured` or `TelemetryUnavailable`. After a ready canary is observed, Gryvia waits a full
60-second window for its current policy and template revision. Samples and underlying raw metric
timestamps must be no more than 30 seconds old. Each evaluation has a four-second total timeout;
responses are limited to 64 KiB. Missing, malformed, partial, stale or non-finite results block promotion
and clear the consecutive SLO breach count. They do not trigger an SLO rollback. Readiness failures
continue to follow the existing startup grace and rollback policy.

A measured threshold breach counts once per `healthCheck.intervalSeconds` (default 30). With
`healthCheck.autoRollback` enabled, `failureThreshold` consecutive breaches reject the version and
remove the canary, returning its Gateway route to stable-only. A ready canary continues receiving its
configured traffic share while awaiting analysis or a rollback threshold, allowing telemetry to accrue.
If automatic rollback is disabled, failures block promotion and require operator intervention.

Promotion requires sampled passing evaluations for `promoteAfterSeconds`; the persisted success
hold resets on missing data, breaches, readiness loss, policy/version/template changes or a changed
Prometheus endpoint. A requested Gateway must independently pass its existing routing hold.
The hold survives short operator restarts. Gaps between successful evaluations longer than twice the
health interval (at least 60 seconds) reset the hold. These are checks at reconcile intervals, not proof of continuous
health between samples. The feature evaluates serving reliability; it does not assess output quality.

## Validation and limits

Unit tests cover HPA targets/errors, status freshness, route weighting, cold/unready canaries, defaults,
cleanup, ownership and promotion gating. An `integration` Go test runs against envtest with Kubernetes
API defaults/validation and the official Gateway API HTTPRoute CRD. Its Gateway and HPA conditions are
explicit fixtures: no Gateway data plane, HPA controller, GPU, metrics adapter or model server is running.
A CI workflow installs the pinned test dependencies and runs the same checks.

Real-cluster acceptance requires observing HPA replica changes under load, request distributions through
your chosen Gateway, canary rollback and streaming/drain behavior with actual serving images.

Prometheus query semantics and response formats: [functions](https://prometheus.io/docs/prometheus/latest/querying/functions/) and [HTTP API](https://prometheus.io/docs/prometheus/latest/querying/api/).

Opt-in proxy telemetry and live Prometheus validation are described in [platform completion](platform-completion.md). Real model-server and GPU Gateway qualification remains required.
