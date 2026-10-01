# LLM gateway

One OpenAI-compatible endpoint in front of every `GryviaInferenceService` annotated `gryvia.io/llm-model`. Callers
use per-tenant API keys, the gateway routes on the request's `model`, meters tokens into `GryviaUsageRecord`s of kind
`tokens` and refuses requests with HTTP 429 once a `GryviaQuota.spec.tokensPerDay` is used up. It is opt-in.

It runs as `/manager llm-gateway`, a mode of the ai-operator binary (same image), in its own Deployment.

## What is verified and what is not

| Verified | How |
| --- | --- |
| Key authentication, routing by model (own namespace first, then shared), the model rewrite, the Authorization header not being forwarded, `/v1/models` scoped to the key, streaming with `stream_options.include_usage` and usage read from the last SSE chunk, OpenAI error bodies, 400/404/413/502 | Unit tests with `httptest` upstreams (`operators/ai-operator/pkg/llmgateway/llmgateway_test.go`) |
| Hourly token records (create, update while open, final after the hour), the tokensPerDay check seeded from today's records, the UTC day reset, 429 | Same tests with a fake client |
| Key create/list/revoke with tenant scoping, the key shown once and only its hash stored, models and usage routes | `services/api-gateway/tests/test_llm.py` |
| CLI key Secret, hashing, model and usage tables | `cli/src/commands/llm.rs` tests |
| The gateway on a real cluster against a stand-in OpenAI server: key, model list, a chat completion, a streamed one, the usage record and a 429 | The "LLM gateway" step of `.github/workflows/e2e-ml.yml`; these steps also passed on a single-node k3s host (2026-10-01) |

| Not verified | Why |
| --- | --- |
| vLLM itself behind the gateway | No GPU in CI; the stand-in speaks the same JSON and SSE format |
| Exact quota enforcement with several replicas | Each replica counts its own traffic after seeding from the records (see Quotas) |

## Turning it on

```yaml
llmGateway:
  enabled: true
  replicas: 1
  keyNamespace: gryvia-llm-keys     # created by the chart; holds only the key Secrets
  priceInputPer1M: 0                # default prices per million tokens (annotations override them)
  priceOutputPer1M: 0
  currency: USD
  flushInterval: 5m                 # --flush-interval: how often usage records are written
  quotaRefreshInterval: 30s         # --quota-refresh-interval
  service: {type: ClusterIP, port: 8080}
```

The chart adds the Deployment and Service `<release>-llm-gateway` in the release namespace, the namespace
`keyNamespace`, and:

| Who | Access |
| --- | --- |
| llm-gateway ServiceAccount (ClusterRole) | get/list/watch `gryviainferenceservices`, `gryviaquotas`, `gryviavectorindexes`; get/list/create/update `gryviausagerecords` |
| llm-gateway ServiceAccount (Role in keyNamespace) | get/list/watch `secrets` |
| api-gateway ServiceAccount (Role in keyNamespace) | get/list/create/delete `secrets` (key issue and revoke) |

Keep `keyNamespace` dedicated to keys: the api-gateway may create and delete any Secret in it. The namespace has
`helm.sh/resource-policy: keep`, so uninstalling the chart keeps the keys.

The api-gateway learns the key namespace and the gateway URL from `GRYVIA_LLM_KEY_NAMESPACE` and
`GRYVIA_LLM_GATEWAY_URL` (set by the chart).

## Publishing a model

Annotate the inference service. Its `status.endpoint` is the upstream; a service without an endpoint is not routed.

| Annotation | Meaning |
| --- | --- |
| `gryvia.io/llm-model` | The name callers put in `model` (letters, digits, `.`, `_`, `-`; at most 63) |
| `gryvia.io/llm-served-model` | What the upstream calls the model (vLLM `--served-model-name`), when different; the gateway rewrites `model` |
| `gryvia.io/llm-shared` | `"true"`: keys of every namespace may call it; otherwise only keys of the service's namespace |
| `gryvia.io/llm-price-input-per-1m`, `gryvia.io/llm-price-output-per-1m` | Price per million tokens for the usage records |

When two services publish the same model name, a key uses the one in its own namespace, else the first shared one
(by namespace name). See `examples/llm-gateway/published-model.yaml`.

## Keys

A key is `gk-` and random characters. Only its sha256 is stored, in a Secret of `keyNamespace` labelled
`gryvia.io/llm-key=true` with data `hash` and `namespace` and labels `gryvia.io/tenant`, `gryvia.io/llm-key-name`
and `gryvia.io/llm-key-namespace`. The key is shown once, at creation.

```bash
gryvia llm keys create ci -n tenant-alpha --description "CI runs"   # prints the key once
gryvia llm keys list -n tenant-alpha                                # or -A for every namespace
gryvia llm keys delete ci -n tenant-alpha --yes
```

The CLI writes the Secret directly, so it needs create rights on Secrets in `keyNamespace`. Tenant users create keys
through the dashboard ("LLM gateway" page) or `POST /api/llm-keys` instead; the api-gateway only lets them pick one
of their own namespaces. Revoking takes effect as soon as the gateway's informer sees the deletion (seconds).

Vector indexes ([RAG](rag.md)) get keys of their own from the ai-operator: Secrets `<namespace>.index-<name>` with the
label `gryvia.io/llm-key-owner=index`, listed with the tenant's keys. The operator writes them back on its next
reconcile, so suspend or delete the index rather than revoking its key. [Agents](agents.md) get keys the same way:
Secrets `<namespace>.agent-<name>`, label `gryvia.io/llm-key-owner=agent`. The gateway also caches the copies of external
vector-store API keys kept in `keyNamespace` (label `gryvia.io/llm-key=store`); they are not API keys.

## Calling it

```bash
curl http://<release>-llm-gateway.gryvia-system.svc.cluster.local:8080/v1/chat/completions \
  -H "Authorization: Bearer $KEY" -H "Content-Type: application/json" \
  -d '{"model":"chat","messages":[{"role":"user","content":"Hello"}]}'
```

| Route | Behaviour |
| --- | --- |
| `GET /v1/models` | The models the key may call |
| `POST /v1/chat/completions`, `/v1/completions`, `/v1/embeddings` | Proxied to the model's endpoint; streaming responses are relayed event by event |
| `POST /v1/retrieve` | Nearest chunks of a `GryviaVectorIndex` of the key's namespace; the query embedding is metered (see [RAG](rag.md)) |
| `GET /healthz` | No key needed |

Errors use the OpenAI shape `{"error": {"message", "type", "code"}}`: 401 (missing or unknown key), 404 (unknown model
or not visible to the key), 413 (body over `--max-body-bytes`, 16 MiB), 429 (tokensPerDay used up), 502 (upstream
unreachable). The `Authorization` header is not forwarded upstream. For streamed chat and completion requests the
gateway sets `stream_options.include_usage` so the final chunk carries the token counts.

The Service is ClusterIP. Expose it with your own Ingress or Gateway and TLS if callers are outside the cluster.

## Metering

Prometheus on port 8081 (`/metrics`):

| Metric | Labels |
| --- | --- |
| `gryvia_llm_tokens_total` | `tenant`, `model`, `type` (`input` or `output`) |
| `gryvia_llm_requests_total` | `tenant`, `model`, `code` |
| `gryvia_llm_quota_rejections_total` | `tenant` |

Every 5 minutes (`--flush-interval`) each replica writes one `GryviaUsageRecord` per tenant, model and hour in the
key's namespace: `kind: tokens`, `job: llm:<model>`, `gpus: 0`, `start`/`end` the hour, `inputTokens`,
`outputTokens`, `cost` (from the prices) and `final: true` once the hour is over. Records are labelled
`gryvia.io/usage-kind=tokens`. Budgets and chargeback that sum `cost` include them; GPU-hour totals do not change.
A replica that dies loses at most its last 5 minutes of counts.

```bash
gryvia llm usage -n tenant-alpha --group-by day --days 7
gryvia llm usage -A --group-by tenant
```

`GET /api/llm/usage?groupBy=model|tenant|namespace|day&from=&to=` returns the same sums plus each quota's tokens used
today.

## Quotas

Set `tokensPerDay` on a `GryviaQuota`; it applies to keys of the namespaces in `spec.namespaces`, counting input plus
output tokens per UTC day:

```yaml
spec:
  namespaces: [tenant-research]
  tokensPerDay: 2000000
```

The gateway reloads quotas every 30 seconds. A namespace's count for today is the process's own traffic, counted in
memory, plus the token records other gateway processes wrote (read once per namespace and day; records carry the
writer's `gryvia.io/llm-gateway-instance` label, so a process never counts its own traffic twice). A request is
refused when the count is at or over the limit, so the last request may go over it by its own size. With several
replicas each sees the others' traffic only as of that read, so the limit is approximate by up to (replicas - 1)
times the traffic since they started; run one replica where the cap must be tight.

## Flags

| Flag | Default |
| --- | --- |
| `--listen-address` | `:8080` |
| `--metrics-address` | `:8081` |
| `--key-namespace` | `gryvia-llm-keys` |
| `--price-input-per-1m`, `--price-output-per-1m` | `0` |
| `--currency` | `USD` |
| `--flush-interval` | `5m` |
| `--quota-refresh-interval` | `30s` |
| `--max-body-bytes` | 16 MiB |
| `--upstream-timeout` | `10m` |
