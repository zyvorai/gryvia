# GPU as a Service

Run Gryvia as a multi-tenant GPU cloud: you (the **provider**) onboard **tenants**, publish a **catalog** of GPU
SKUs with prices, tenants self-serve GPU workloads in their own isolated namespace, and Gryvia **meters** each
tenant's GPU hours and cost.

:::note What this is, and is not
Costs are **metered estimates** from job run time (GPUs x hours x the SKU's hourly rate). Gryvia does not take
payments, issue invoices or reserve capacity. Use the CSV/JSON export to feed your billing system.
:::

```
provider (API key / admin group)                tenant user (OIDC token)
  create tenants, publish SKUs                    sees only its own jobs, usage and quotas
        │                                                    │
        ▼                                                    ▼
 GryviaTenant ─▶ namespace tenant-<name> + ResourceQuota + NetworkPolicy
 GryviaGpuSku  (price sheet)  ─┐
 GryviaAIJob   (run time)     ─┴─▶ usage controller ─▶ GryviaUsageRecord ─▶ /api/usage, CSV/JSON, dashboard, CLI
```

## 1. Configure sign-in

Turn on OIDC ([Authentication](./AUTH_AND_TLS.md#single-sign-on)) and make your identity provider put the tenant in
the token: the `org` claim, or the tenant among the `groups`. Optionally list your operators' group in
`apiGateway.oidc.adminGroups`. The API key remains the provider admin.

## 2. Publish the catalog

```yaml
apiVersion: gryvia.io/v1alpha1
kind: GryviaGpuSku
metadata:
  name: h100-80g
spec:
  gpuType: H100
  hourlyRate: 8        # per GPU-hour
  currency: USD
  spotDiscount: 60     # percent, informational
```

Or `gryvia catalog`, the dashboard **Catalog** page (admin can add and edit), or `POST /api/skus`. With no SKUs at
all, Gryvia prices jobs from a built-in default table so a fresh install still shows costs.

## 3. Onboard a tenant

```yaml
apiVersion: gryvia.io/v1alpha1
kind: GryviaTenant
metadata:
  name: acme
spec:
  displayName: Acme Robotics
  allowedSkus: [h100-80g]      # empty = every enabled SKU
  quotas:
    concurrentGPUs: 8
  networkPolicy:
    isolated: true
```

The tenant controller creates the namespace `tenant-acme` with a ResourceQuota, LimitRange and (when isolated) a
NetworkPolicy. Jobs whose GPU type is not allowed are rejected by the quota operator
(`GpuTypeNotAllowed` / `NoEnabledSku`) wherever a `GryviaQuota` covers the namespace.

## 4. What a tenant sees

A tenant user sees only its own tenant: jobs, workspaces, inference, workflows, tuners, quotas, usage and the
catalog (enabled SKUs, limited to `allowedSkus`). Cluster-wide pages (nodes, network, security, GPU metrics) are
admin-only and answer 403.

## 5. Metering and export

One `GryviaUsageRecord` per job (`usage-<job uid>` in the job's namespace) records GPUs, start, end, GPU hours, rate
and cost. A running job's record is refreshed every minute; when the job ends it is finalised and never changed again.

```bash
gryvia usage --group-by tenant
gryvia usage --tenant acme --from 2026-09-01 --to 2026-09-30 --group-by day -o csv
curl -H "Authorization: Bearer $TOKEN" "https://<gateway>/api/usage/export?format=csv&tenant=acme"
```

The dashboard **Usage** page shows the same numbers with export buttons.

## Limits

- Estimates only; restarted, preempted or spot jobs are metered by wall-clock time.
- Enforcement needs a `GryviaQuota` covering the tenant namespace; a namespace without one is not enforced.
- The admission webhook path in the chart does not match the code, so admission-time quota checks are not active;
  enforcement is reactive (jobs are rejected shortly after creation).
- Verified with unit tests and fake clusters; it has not yet run against a real identity provider or GPUs.
