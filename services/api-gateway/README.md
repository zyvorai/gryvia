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
3. **Node Status**: GPU health from FabricGpuNode CRDs

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
