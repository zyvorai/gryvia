# Gryvia API Gateway

REST API service that provides aggregated metrics and cluster data for the Gryvia Web UI.

## Features

- **Cluster Statistics**: Real-time GPU availability, utilization, and job counts
- **GPU Metrics**: Historical GPU utilization, temperature, and memory usage
- **Cost Analysis**: Budget tracking, spending by team and GPU type
- **Job Metrics**: Job statistics by status and framework
- **Quota Usage**: Team quota utilization and budget tracking
- **Node Health**: GPU node health monitoring

## API Endpoints

### Cluster Stats
```
GET /api/cluster/stats
```
Returns overall cluster statistics including GPU counts, utilization, and job counts.

### Jobs CRUD
```
GET  /api/jobs              # List all jobs (supports ?limit=100&offset=0)
POST /api/jobs              # Create a new job
GET  /api/jobs/{name}       # Get job details
DELETE /api/jobs/{name}     # Delete a job
GET  /api/jobs/{name}/pods   # Pods of the job
GET  /api/jobs/{name}/logs   # Pod log (?pod=&tail=200, max 2000)
GET  /api/jobs/{name}/events # Events for the job and its pods
```

- `create_job` (POST) validates `apiVersion` and `kind` against known Gryvia types and enforces the namespace server-side.
- List endpoint supports pagination via `limit` (default 100) and `offset` query parameters.

### Quotas
```
GET /api/quotas             # List all quotas (supports ?limit=100&offset=0)
GET /api/quotas/{name}      # Get quota details
```

### Nodes
```
GET /api/nodes              # List all nodes (supports ?limit=100&offset=0)
GET /api/nodes/{name}       # Get node details
```

### GPU Metrics
```
GET /api/metrics/gpu?time_range=1h
```
Returns GPU metrics over time. Time ranges: `1h`, `6h`, `24h`, `7d`.

### Cost Metrics
```
GET /api/metrics/costs
```
Returns cost analysis including breakdown by team and by GPU type. The response includes a `hasHistoricalData` field indicating whether Prometheus historical data is available. Monthly cost trend data has been removed (historical trends require a Prometheus instance).

### Job Metrics
```
GET /api/metrics/jobs?time_range=24h
```
Returns job statistics including counts by status and framework.

### Quota Usage
```
GET /api/quota/usage
```
Returns quota usage across all teams.

### Node Health
```
GET /api/nodes/health
```
Returns GPU node health status.

### Roles and tenant isolation

- The API key and dashboard sessions are the provider **admin**. An OIDC user is a **tenant** user unless its
  `groups` claim contains a value from `GRYVIA_OIDC_ADMIN_GROUPS` (comma separated, default none).
- A tenant user's `org` claim (or, without `org`, its `groups` values) must match a `GryviaTenant` by name or by
  namespace `tenant-<name>`. Its namespaces are those of the matched tenants only; no match is a 403. The tenant list is cached for 30 s.
  `GRYVIA_OIDC_LEGACY_NAMESPACES=1` keeps the old "claim = namespace" behaviour, and only while no `GryviaTenant` exists.
- `GET /api/auth/me` returns `role` (`admin`|`tenant`), `tenant`, `tenants` and `tenantNamespaces`.
- Admin only (403 for tenants): nodes, node health, `/api/metrics/gpu`, `/api/network/*`, `/api/security/*`,
  `/api/ai/*`, `/api/gpu/memory`, and all writes to `/api/skus` and `/api/tenants`.
- Tenant-scoped (a tenant sees only its own namespaces): jobs, workspaces, models, inference, workflows, tuners,
  `/api/quotas`, `/api/quota/usage` (quotas whose `spec.namespaces` intersect), `/api/metrics/costs`,
  `/api/metrics/jobs`, `/api/cluster/stats` (jobs only, no node capacity), `/api/usage`.

### GPU catalog, tenants, usage
```
GET    /api/skus[/{name}]        # any user; tenants see enabled SKUs, limited to spec.allowedSkus of their tenant
POST   /api/skus                 # admin: {name, gpuType, gpusPerUnit, hourlyRate, currency, spotDiscount, description, enabled}
PUT    /api/skus/{name}          # admin: same fields without name
DELETE /api/skus/{name}          # admin
GET    /api/tenants[/{name}]     # admin: all; tenant: its own
POST   /api/tenants              # admin: {name, displayName, allowedSkus, maxGPUs, isolated}
DELETE /api/tenants/{name}       # admin
GET    /api/usage?tenant=&from=&to=&groupBy=tenant|sku|day   # {items:[{key,gpuHours,cost,currency,jobs}], totals}
GET    /api/usage/export?format=csv|json&tenant=&from=&to=   # per-record rows, attachment download
```
Usage comes from `GryviaUsageRecord` objects (metered estimates from job wall-clock time; no billing). Tenant users are
always limited to their own tenant, whatever `tenant` says. `/api/metrics/costs` uses usage records when any exist and
otherwise computes from jobs, priced from `GryviaGpuSku` (a built-in table only when no SKU exists).

## Development

### Prerequisites

- Python 3.11+
- Access to Kubernetes cluster with Gryvia installed
- Prometheus with DCGM exporter (optional for detailed metrics)

### Local Development

```bash
# Install dependencies
pip install -r requirements.txt

# Run the service
python main.py

# Or use uvicorn directly
uvicorn main:app --reload --port 8080
```

The API will be available at `http://localhost:8080`.

API documentation (Swagger UI): `http://localhost:8080/docs`

### Docker

```bash
# Build the image
docker build -t gryvia-api-gateway:1.0.0 .

# Run the container
docker run -p 8080:8080 \
  -v ~/.kube/config:/home/apigateway/.kube/config:ro \
  gryvia-api-gateway:1.0.0
```

## Deployment

### Kubernetes

```bash
kubectl apply -f ../../manifests/deploy/api-gateway-deployment.yaml
```

The service will be exposed internally at `https://gryvia-api-gateway.gryvia-system:8080` (self-signed certificate; clients must skip verification or trust it).

### Configuration

The service is configured via environment variables:

- `PROMETHEUS_URL`: Prometheus endpoint (default: `http://prometheus-operated.gryvia-system:9090`)
- `KUBERNETES_NAMESPACE`: Default namespace for jobs (default: `default`)
- `LOG_LEVEL`: Logging level (default: `INFO`)

## Integration with Web UI

The Web UI proxies requests to this API gateway. Configure the Web UI's Vite proxy:

```typescript
// vite.config.ts
server: {
  proxy: {
    '/api': {
      target: 'http://localhost:8080',
      changeOrigin: true,
    },
  },
}
```

In production, configure nginx to proxy `/api` requests to the API gateway service.

## Implementation Notes

- All Kubernetes API calls are async, executed via `run_in_executor` to avoid blocking the event loop.
- `datetime.now(timezone.utc)` is used instead of the deprecated `datetime.utcnow()`.
- The Prometheus client is optional and initialized inside a `try/except` block; the service operates without Prometheus for basic CRUD functionality.
- All list endpoints support pagination with `limit` (default 100) and `offset` query parameters.

## Metrics Collection

The API gateway collects data from multiple sources:

1. **Kubernetes API**: CRD objects (jobs, quotas, nodes)
2. **Prometheus**: GPU metrics from DCGM exporter (optional)
3. **Node Status**: GPU health from GryviaGpuNode CRDs

## Performance

- Responses are typically < 100ms for cluster stats
- GPU metrics queries scale with number of nodes
- Cost calculations are cached for 5 minutes
- Supports 1000+ requests/second with proper resource allocation

## Security

- Runs as non-root user (UID 1000)
- Uses Kubernetes RBAC for API access
- CORS enabled for development (disable in production)
- No authentication built-in (use ingress-level auth)

## Monitoring

Health check endpoint:
```bash
curl http://localhost:8080/health
```

Prometheus metrics (if enabled):
```bash
curl http://localhost:8080/metrics
```

## License

Apache 2.0
