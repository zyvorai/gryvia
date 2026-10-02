# Sovereign AI OS

An on-premises AI platform built from three Zyvor products, installed as one Helm release
([`helm/sovereign-aios`](../helm/sovereign-aios)) and run without any public cloud:

- **Gryvia** (this repository): GPU scheduling, model serving, the [LLM gateway](llm-gateway.md), [RAG](rag.md),
  [agents](agents.md), [datasets](datasets.md), the model registry, tenants, metering.
- **Zyntra**: a business ontology over Gryvia's resources, KPI gaps, ranked actions, approvals by named people and a
  signed audit trail.
- **Netra**: eBPF network observability and enforcement under the GPU fabric.

It covers the same layers as a vendor "sovereign AI OS" (Palantir's AIOS reference architecture with NVIDIA), from
components you can read and run disconnected. It is early: see
[What is verified and what is not](#what-is-verified-and-what-is-not).

## Layers

| Layer | Vendor stack | Here |
| --- | --- | --- |
| Hardware | NVIDIA HGX B300, Spectrum-X | Same hardware; [`ansible/`](../ansible), [`terraform/bare-metal`](../terraform/bare-metal), NVIDIA GPU and Network Operator sub-charts |
| Hardened Kubernetes | Rubix | Any Kubernetes >= 1.30; Netra for network visibility and policy. RKE2 CIS hardening, Kyverno, Falco, SPIRE and OpenBao are not wired yet |
| Delivery and air gap | Apollo | Helm. Offline bundles (Zarf, Harbor mirror) are not built yet |
| Models, agents, retrieval | AIP | Gryvia LLM gateway, `GryviaInferenceService` (vLLM, llama.cpp), `GryviaAgent`, `GryviaVectorIndex` (Qdrant) |
| Model hub | AIP Hub | `GryviaModelRegistry`, the [model factory](model-factory.md) |
| Ontology and decisions | Foundry ontology, actions | Zyntra: objects with provenance, typed actions, approvals, signed decisions |
| Data integration | Foundry pipelines | `GryviaDataset` (http, s3, nfs), Zyntra connectors (REST, SQL, Kubernetes, files) |

## How the parts are wired

```text
 Zyntra ──REST connectors (Gryvia API key)──▶ Gryvia API gateway ──▶ Gryvia CRDs (jobs, nodes, models, ...)
   │  ──KPIs: /api/cluster/stats, /api/metrics/costs ─┘
   │  ──KPIs: /api/v1/ebpf/health, /metrics ─────────▶ Netra
   │  ──Ask, drafts (platform LLM key) ──────────────▶ Gryvia LLM gateway ──▶ vLLM / llama.cpp
   └──approved actions: GryviaPriority, GryviaGPUSharingPolicy (kubectl) ──▶ Kubernetes API
 Netra ──AI briefs (platform LLM key) ───────────────▶ Gryvia LLM gateway
```

The chart does this wiring:

- **One credentials Secret** (`sovereign-aios-credentials`): the Gryvia API key (Gryvia's gateway uses it through
  `gryvia.auth.existingSecret`, Zyntra sends it as `ZYNTRA_GRAVIA_TOKEN`), the platform LLM key, and Netra's API and
  agent keys. Every key is generated on first install and read back on upgrades.
- **An LLM gateway key without the API**: the chart writes the SHA-256 of the platform key into the gateway's key
  namespace, the same format the ai-operator uses for agent keys. Zyntra's and Netra's model calls are metered under
  the release namespace and capped by its `GryviaQuota` like any tenant's.
- **One certificate**: Gryvia's self-signed certificate gets Netra's names (`gryvia.tls.extraSANs`) and Netra serves
  it (`netra.tls.existingSecret`), so Zyntra verifies both with `SSL_CERT_FILE` pointing at that one certificate. No
  client skips verification.
- **The Zyntra pack** ([`files/zyntra-pack`](../helm/sovereign-aios/files/zyntra-pack)) is mounted from a ConfigMap.
  It turns Gryvia tenants, GPU nodes, AI jobs, models, inference services and datasets into Zyntra objects, refreshed
  every one to five minutes, and Gryvia and Netra numbers into KPIs.

Zyntra's in-cluster Kubernetes client only reads built-in kinds, so it reads Gryvia's custom resources through the
Gryvia gateway, which also applies Gryvia's tenant scoping.

## Install

Gryvia, Zyntra and Netra must be checked out side by side (or set `ZYNTRA_DIR` and `NETRA_DIR`): Netra does not
publish its chart, so the dependencies are copied from the checkouts.

```sh
make sovereign-aios-test       # copies the charts, then the render tests
make sovereign-aios-install    # helm upgrade --install sovereign-aios helm/sovereign-aios -n gryvia-system
```

The release must go into Gryvia's namespace (`gryvia.namespace.name`, `gryvia-system` by default): Zyntra and Netra
mount Gryvia's certificate and the credentials Secret, which cannot cross namespaces. The chart refuses another
namespace.

Then publish a model on the gateway (a `GryviaInferenceService` annotated `gryvia.io/llm-model`, see
[LLM gateway](llm-gateway.md)) and set `zyntra.ai.model` and `netra.ai.model` to its name. Until then Zyntra and Netra
give their deterministic answers.

## Agents on the ontology

A `GryviaAgent` with a [`zyntra` tool](agents.md#zyntra-tools) searches the ontology this release builds and drafts
proposals that people approve in Zyntra. The agent authenticates with a Zyntra service token, which Zyntra limits to
the viewer and proposer roles, so it cannot approve.

The chart makes that token. It is generated once (`zst_` and 64 hex digits) and kept in the credentials Secret as
`AGENT_ZYNTRA_TOKEN`. Its SHA-256 is added to `service_tokens` in Zyntra's policy: the chart merges `zyntra.policy`
with the token entry and writes the result to the Secret `zyntra.policyExistingSecret`, which Zyntra reads. Each
namespace in `agentToken.namespaces` gets the token in Secret `zyntra-agent-token` (key `token`):

```yaml
agentToken:
  namespaces: [tenant-alpha]
  roles: [viewer, proposer]       # [viewer] for read-only agents
  tenant: ""                      # bind the token to one Zyntra tenant
```

The tool's `url` is `http://<release>-zyntra.gryvia-system.svc:8080`, as the install notes print. The agent's
NetworkPolicy opens egress to it; if `zyntra.networkPolicy.enabled` is set, add the agent's namespace to
`zyntra.networkPolicy.ingressFrom`. Zyntra does not restart when only the token's roles or tenant change; restart its
Deployment. Service tokens and `policyExistingSecret` are in Zyntra's 0.4.0 chart and image from `main`.

## Hardware tiers

| Tier | Nodes | GPUs | Network | Use |
| --- | --- | --- | --- | --- |
| Dev | kind or one k3s host | none (llama.cpp on CPU) | any | the chart, the pack, the wiring |
| One scalable unit | 4 GPU nodes, 3 control-plane nodes | 32 (8 per HGX node) | 400G east-west, RDMA | one site, training and serving |
| Full | 32 scalable units, 128 GPU nodes | 1,024 | rail-optimized 400G fabric, Spectrum-X or InfiniBand | multi-team training and national-scale serving |

The scalable unit follows the vendor reference architecture (4 HGX nodes per unit, up to 128 nodes). Gryvia's
[fabric scheduling](fabric-scheduling.md) places jobs by GPU type, NVLink/NVSwitch and RDMA labels; Netra watches the
host network under it.

## What is verified and what is not

| Verified | How |
| --- | --- |
| The chart renders with all three products: the shared credentials, the gateway key hash (SHA-256 of a `gk-` key), Zyntra's pack, URLs, certificate and keys, the agents' service token (its hash in Zyntra's policy merged with `zyntra.policy`, the token in each agent namespace), Netra's certificate and keys, Netra and Zyntra switched off, and the namespace guard | [`scripts/tests/sovereign-aios-chart.test.sh`](../scripts/tests/sovereign-aios-chart.test.sh) (`make sovereign-aios-test`), in the Sovereign AI OS workflow |
| An agent's zyntra tool against a real Zyntra wired by the chart: Zyntra built from source serves the pack against a stand-in Gryvia gateway with the policy and token the chart rendered; the reference runtime, driven by a scripted model, finds and reads Gryvia objects, proposes `raise-inference-priority` (recorded as `service:gryvia-agents`, pending), is refused the approver-only `enable-mig-sharing`, gets 403 approving its own proposal, and a person's approval succeeds; an unknown token gets 401 | [`scripts/tests/sovereign-aios-agent-zyntra.sh`](../scripts/tests/sovereign-aios-agent-zyntra.sh) (`make sovereign-aios-agent-test`), in the Sovereign AI OS workflow; passed locally (2026-10-02) |
| The pack is valid for Zyntra 0.4: 8 KPIs, 6 object types, 3 link types, 2 typed actions, 3 views | `zyntra pack validate helm/sovereign-aios/files/zyntra-pack` |
| Zyntra reads Gryvia through the pack: all six REST connectors ingest objects with their provenance and resolve the three links, and Gryvia's stats become KPIs and gaps (5 pending jobs against a target of 2, 42.5% utilization against 60%) | Zyntra 0.4.0-dev serving the pack against a stand-in Gryvia gateway with the same response shapes and bearer auth (2026-10-02) |

| Not verified | Why |
| --- | --- |
| The release installed on a cluster: Zyntra reaching Gryvia's real gateway over TLS, the gateway accepting the platform key, Netra serving Gryvia's certificate | Not run yet; only rendered |
| Approved actions running | Zyntra runs Gryvia actions with `kubectl`, which its image does not carry; proposals show the rendered object but running one fails until `kubectl` is provided |
| Per-object actions (suspend this job) | Zyntra typed actions take fixed parameters; passing the chosen object's name and namespace into a Gryvia template needs a Zyntra change |
| Objects with the same name in two tenants | AI jobs and datasets are keyed by name, so two tenants' jobs of the same name are one object |
| A real model choosing the Zyntra tools, and the agent inside a cluster | The agent test scripts the model and runs the runtime as a process; the in-cluster path (Secret, env var, NetworkPolicy) is covered by the controller tests only |
| Hardening (RKE2 CIS, Kyverno, Falco, SPIRE, OpenBao, image signatures) and the offline bundle | Not built yet |
| Anything on GPUs, RDMA or at the scale of the tiers above | No GPU hardware in CI, as for the rest of Gryvia |

## Licenses

Gryvia is Apache-2.0. Zyntra and Netra are under the Zyvor Production License v1.0, not an open-source license, so
the release as a whole is not open source. Check that the combination fits your use before you deploy it.
