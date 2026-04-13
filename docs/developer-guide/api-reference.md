# API Reference

Complete REST API reference for TensorReaper.

## Base URL

Internal (from within the cluster):
```
http://tensorreaper-api-gateway.tensorreaper.svc.cluster.local:8080
```

External (via NodePort):
```
http://<server-ip>:30088
```

Via Web UI proxy (handles auth automatically):
```
http://<server-ip>:30081/api/
```

## Authentication

All API requests require a Bearer token. Set the API key on the gateway:

```bash
# Set API key
kubectl set env deployment/tensorreaper-api-gateway -n tensorreaper \
  TENSORREAPER_API_KEY=your-secure-key

# Use in requests
curl -H "Authorization: Bearer your-secure-key" \
  http://<server-ip>:30088/api/cluster/stats
```

The Web UI's nginx proxy injects the auth header automatically for `/api/` requests.

## API Gateway CRUD Endpoints

The API gateway exposes simplified CRUD endpoints for direct resource management. All list endpoints support pagination via `limit` (default 100) and `offset` query parameters. All Kubernetes API calls are executed asynchronously via `run_in_executor`.

### Jobs

```http
GET    /api/jobs              # List jobs (?limit=100&offset=0)
POST   /api/jobs              # Create a job (validates apiVersion and kind, enforces namespace server-side)
GET    /api/jobs/{name}       # Get job by name
DELETE /api/jobs/{name}       # Delete job by name
```

### Quotas

```http
GET /api/quotas               # List quotas (?limit=100&offset=0)
GET /api/quotas/{name}        # Get quota by name
```

### Nodes

```http
GET /api/nodes                # List nodes (?limit=100&offset=0)
GET /api/nodes/{name}         # Get node by name
```

**Notes:**
- `POST /api/jobs` validates that `apiVersion` is `tensorreaper.ai/v1` and `kind` is a known TensorReaper type.
- Cost responses include a `hasHistoricalData` field indicating whether Prometheus data is available.
- Monthly cost trend data is not available through the gateway (historical trends require Prometheus).

---

## Jobs API

### List Jobs

```http
GET /api/v1/jobs
```

**Query Parameters:**
- `namespace` (string): Filter by namespace
- `status` (string): Filter by status (pending, running, succeeded, failed)
- `team` (string): Filter by team
- `limit` (int): Limit results (default: 100)
- `offset` (int): Offset for pagination

**Response:**

```json
{
  "jobs": [
    {
      "name": "pytorch-training",
      "namespace": "default",
      "status": {
        "phase": "Running",
        "startTime": "2024-01-15T10:30:00Z",
        "metrics": {
          "avgGPUUtilization": 85.5,
          "avgGPUMemoryUtilization": 72.3
        }
      },
      "spec": {
        "framework": "pytorch",
        "resources": {
          "gpuType": "A100-80G",
          "gpuCount": 8
        }
      }
    }
  ],
  "total": 1
}
```

### Get Job

```http
GET /api/v1/jobs/{namespace}/{name}
```

**Response:**

```json
{
  "metadata": {
    "name": "pytorch-training",
    "namespace": "default",
    "creationTimestamp": "2024-01-15T10:25:00Z",
    "labels": {
      "team": "ml-research",
      "project": "llama"
    }
  },
  "spec": {
    "framework": "pytorch",
    "distributed": {
      "enabled": true,
      "strategy": "ddp",
      "nodes": 1,
      "gpusPerNode": 8
    },
    "resources": {
      "gpuType": "A100-80G",
      "gpuCount": 8,
      "memory": "512Gi",
      "cpu": 64
    },
    "image": "nvcr.io/nvidia/pytorch:24.01-py3",
    "command": ["torchrun", "train.py"]
  },
  "status": {
    "phase": "Running",
    "startTime": "2024-01-15T10:30:00Z",
    "conditions": [
      {
        "type": "Scheduled",
        "status": "True",
        "observedGeneration": 1,
        "lastTransitionTime": "2024-01-15T10:28:00Z"
      }
    ],
    "metrics": {
      "avgGPUUtilization": 85.5,
      "avgGPUMemoryUtilization": 72.3,
      "runningTime": 3600
    }
  }
}
```

### Create Job

```http
POST /api/v1/jobs/{namespace}
```

**Request Body:**

```json
{
  "apiVersion": "tensorreaper.ai/v1",
  "kind": "FabricAIJob",
  "metadata": {
    "name": "new-training-job",
    "labels": {
      "team": "ml-research"
    }
  },
  "spec": {
    "framework": "pytorch",
    "resources": {
      "gpuType": "A100-80G",
      "gpuCount": 8
    },
    "image": "nvcr.io/nvidia/pytorch:24.01-py3",
    "command": ["python", "train.py"]
  }
}
```

**Response:** HTTP 201 Created

### Delete Job

```http
DELETE /api/v1/jobs/{namespace}/{name}
```

**Response:** HTTP 204 No Content

### Get Job Logs

```http
GET /api/v1/jobs/{namespace}/{name}/logs
```

**Query Parameters:**
- `follow` (bool): Stream logs
- `tail` (int): Number of lines from end
- `since` (string): RFC3339 timestamp

**Response:**

```
Epoch 1/10: loss=2.345
Epoch 2/10: loss=1.987
...
```

### Get Job Metrics

```http
GET /api/v1/jobs/{namespace}/{name}/metrics
```

**Response:**

```json
{
  "gpuUtilization": [
    {"timestamp": "2024-01-15T10:30:00Z", "value": 82.5},
    {"timestamp": "2024-01-15T10:31:00Z", "value": 85.3}
  ],
  "gpuMemoryUtilization": [
    {"timestamp": "2024-01-15T10:30:00Z", "value": 70.2},
    {"timestamp": "2024-01-15T10:31:00Z", "value": 72.8}
  ],
  "summary": {
    "avgGPUUtilization": 85.5,
    "avgGPUMemoryUtilization": 72.3,
    "peakGPUUtilization": 95.2,
    "peakGPUMemoryUtilization": 88.5
  }
}
```

## Cluster API

### Get Cluster Status

```http
GET /api/v1/cluster/status
```

**Response:**

```json
{
  "nodes": 10,
  "gpuNodes": 8,
  "totalGPUs": 64,
  "availableGPUs": 32,
  "gpuTypes": {
    "H100": 8,
    "A100-80G": 32,
    "A100-40G": 16,
    "T4": 8
  },
  "runningJobs": 12,
  "pendingJobs": 3,
  "health": "healthy"
}
```

### List GPU Nodes

```http
GET /api/v1/cluster/nodes
```

**Response:**

```json
{
  "nodes": [
    {
      "name": "gpu-node-1",
      "gpuType": "A100-80G",
      "gpuCount": 8,
      "availableGPUs": 4,
      "memory": "512Gi",
      "status": "Ready",
      "utilization": {
        "gpu": 50.0,
        "memory": 45.2
      }
    }
  ]
}
```

### Get Node Details

```http
GET /api/v1/cluster/nodes/{name}
```

**Response:**

```json
{
  "name": "gpu-node-1",
  "labels": {
    "tensorreaper.ai/gpu-type": "A100-80G",
    "tensorreaper.ai/gpu-count": "8"
  },
  "spec": {
    "gpuType": "A100-80G",
    "gpuCount": 8,
    "gpuMemory": "80Gi",
    "totalMemory": "512Gi",
    "totalCPU": 64,
    "nvlink": true,
    "infiniband": true
  },
  "status": {
    "phase": "Ready",
    "allocatedGPUs": 4,
    "availableGPUs": 4,
    "runningJobs": 2
  },
  "metrics": {
    "avgGPUUtilization": 78.5,
    "avgGPUTemperature": 65.2,
    "avgPowerUsage": 285.3
  }
}
```

## Quotas API

### List Quotas

```http
GET /api/v1/quotas
```

**Response:**

```json
{
  "quotas": [
    {
      "name": "ml-research",
      "team": "ml-research",
      "limits": {
        "gpuHours": 1000,
        "maxGPUs": 16
      },
      "usage": {
        "gpuHours": 245,
        "currentGPUs": 8
      },
      "budget": {
        "monthly": 50000,
        "spent": 12450
      }
    }
  ]
}
```

### Get Quota

```http
GET /api/v1/quotas/{name}
```

**Response:**

```json
{
  "metadata": {
    "name": "ml-research"
  },
  "spec": {
    "team": "ml-research",
    "limits": {
      "gpuHours": 1000,
      "maxGPUs": 16
    },
    "budget": {
      "monthly": 50000,
      "currency": "USD"
    }
  },
  "status": {
    "usage": {
      "gpuHours": 245,
      "currentGPUs": 8
    },
    "budget": {
      "spent": 12450,
      "remaining": 37550,
      "percentUsed": 24.9
    }
  }
}
```

### Create Quota

```http
POST /api/v1/quotas
```

**Request Body:**

```json
{
  "apiVersion": "tensorreaper.ai/v1",
  "kind": "FabricQuota",
  "metadata": {
    "name": "new-team"
  },
  "spec": {
    "team": "new-team",
    "limits": {
      "gpuHours": 500,
      "maxGPUs": 8
    },
    "budget": {
      "monthly": 25000
    }
  }
}
```

## Cost API

### Get Cost Summary

```http
GET /api/v1/costs
```

**Query Parameters:**
- `days` (int): Number of days to analyze (default: 30)
- `team` (string): Filter by team
- `namespace` (string): Filter by namespace

**Response:**

```json
{
  "period": {
    "start": "2024-01-01T00:00:00Z",
    "end": "2024-01-31T23:59:59Z",
    "days": 30
  },
  "summary": {
    "totalCost": 45234.50,
    "totalJobs": 342,
    "avgCostPerJob": 132.26,
    "totalGPUHours": 1886.4
  },
  "byTeam": [
    {
      "team": "ml-research",
      "cost": 25432.10,
      "jobs": 156,
      "gpuHours": 1059.7
    }
  ],
  "byGPUType": [
    {
      "gpuType": "A100-80G",
      "cost": 35678.20,
      "gpuHours": 1486.6
    }
  ]
}
```

### Get Job Cost

```http
GET /api/v1/costs/jobs/{namespace}/{name}
```

**Response:**

```json
{
  "jobName": "pytorch-training",
  "namespace": "default",
  "cost": 192.00,
  "breakdown": {
    "gpuCost": 168.00,
    "storageCost": 12.00,
    "networkCost": 8.00,
    "otherCost": 4.00
  },
  "duration": {
    "hours": 7.0,
    "gpuHours": 56.0
  },
  "resources": {
    "gpuType": "A100-80G",
    "gpuCount": 8,
    "hourlyRate": 24.00
  }
}
```

### Get Cost Projection

```http
GET /api/v1/costs/projection
```

**Response:**

```json
{
  "currentMonth": {
    "elapsed": 15,
    "remaining": 15,
    "spent": 18234.50,
    "projected": 36469.00,
    "budget": 50000.00,
    "onTrack": true
  },
  "trend": {
    "lastMonth": 42158.32,
    "changePercent": -13.5
  }
}
```

## Metrics API

### Get System Metrics

```http
GET /api/v1/metrics
```

**Response:**

```json
{
  "timestamp": "2024-01-15T12:00:00Z",
  "cluster": {
    "gpuUtilization": 72.5,
    "gpuMemoryUtilization": 68.3,
    "totalGPUs": 64,
    "allocatedGPUs": 48
  },
  "jobs": {
    "running": 12,
    "pending": 3,
    "succeeded": 1245,
    "failed": 23
  },
  "costs": {
    "daily": 1507.82,
    "monthly": 45234.50
  }
}
```

### Prometheus Metrics

```http
GET /metrics
```

Returns Prometheus-formatted metrics:

```
# HELP tensorreaper_jobs_total Total number of jobs
# TYPE tensorreaper_jobs_total counter
tensorreaper_jobs_total{status="succeeded"} 1245
tensorreaper_jobs_total{status="failed"} 23

# HELP tensorreaper_gpu_utilization GPU utilization percentage
# TYPE tensorreaper_gpu_utilization gauge
tensorreaper_gpu_utilization{node="gpu-node-1",gpu="0"} 85.5

# HELP tensorreaper_cost_total Total cost in USD
# TYPE tensorreaper_cost_total counter
tensorreaper_cost_total{team="ml-research"} 25432.10
```

## WebSocket API

### Stream Job Logs

```javascript
const ws = new WebSocket('ws://tensorreaper-api:8000/api/v1/jobs/default/my-job/logs/stream');

ws.onmessage = (event) => {
  console.log(event.data);
};
```

### Stream Metrics

```javascript
const ws = new WebSocket('ws://tensorreaper-api:8000/api/v1/metrics/stream');

ws.onmessage = (event) => {
  const metrics = JSON.parse(event.data);
  console.log(metrics);
};
```

## Error Responses

### Standard Error Format

```json
{
  "error": {
    "code": "QUOTA_EXCEEDED",
    "message": "Team ml-research has exceeded GPU quota",
    "details": {
      "team": "ml-research",
      "limit": 16,
      "current": 16,
      "requested": 8
    }
  }
}
```

### Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `INVALID_REQUEST` | 400 | Invalid request body or parameters |
| `UNAUTHORIZED` | 401 | Missing or invalid authentication |
| `FORBIDDEN` | 403 | Insufficient permissions |
| `NOT_FOUND` | 404 | Resource not found |
| `QUOTA_EXCEEDED` | 429 | Quota limit exceeded |
| `INTERNAL_ERROR` | 500 | Internal server error |

## Rate Limiting

- **Rate Limit:** 1000 requests per hour per API key
- **Headers:**
  - `X-RateLimit-Limit`: Total requests allowed
  - `X-RateLimit-Remaining`: Remaining requests
  - `X-RateLimit-Reset`: Time when limit resets (Unix timestamp)

## SDKs

### Python SDK

```python
from tensorreaper import TensorReaperClient

client = TensorReaperClient(
    api_url="http://tensorreaper-api:8000",
    api_key="your-api-key"
)

# List jobs
jobs = client.jobs.list(namespace="default")

# Create job
job = client.jobs.create(
    namespace="default",
    spec={
        "framework": "pytorch",
        "resources": {"gpuType": "A100-80G", "gpuCount": 8},
        "image": "nvcr.io/nvidia/pytorch:24.01-py3",
        "command": ["python", "train.py"]
    }
)

# Get job status
status = client.jobs.get(namespace="default", name="my-job")

# Stream logs
for line in client.jobs.logs(namespace="default", name="my-job", follow=True):
    print(line)

# Get costs
costs = client.costs.summary(days=30, team="ml-research")
```

### Go SDK

```go
import "github.com/ssahani/tensor-reaper/sdk/go/tensorreaper"

client := tensorreaper.NewClient(tensorreaper.Config{
    APIURL: "http://tensorreaper-api:8000",
    APIKey: "your-api-key",
})

// List jobs
jobs, err := client.Jobs.List(ctx, "default", nil)

// Create job
job, err := client.Jobs.Create(ctx, "default", &tensorreaper.JobSpec{
    Framework: "pytorch",
    Resources: tensorreaper.Resources{
        GPUType:  "A100-80G",
        GPUCount: 8,
    },
    Image:   "nvcr.io/nvidia/pytorch:24.01-py3",
    Command: []string{"python", "train.py"},
})

// Get costs
costs, err := client.Costs.Summary(ctx, tensorreaper.CostOptions{
    Days: 30,
    Team: "ml-research",
})
```

## Webhooks

### Job Status Webhook

Register a webhook to receive job status updates:

```http
POST /api/v1/webhooks
```

**Request:**

```json
{
  "url": "https://my-service.com/webhook",
  "events": ["job.started", "job.completed", "job.failed"],
  "filters": {
    "namespace": "default",
    "team": "ml-research"
  },
  "secret": "webhook-secret-for-hmac"
}
```

**Webhook Payload:**

```json
{
  "event": "job.completed",
  "timestamp": "2024-01-15T12:30:00Z",
  "job": {
    "name": "pytorch-training",
    "namespace": "default",
    "status": "Succeeded"
  },
  "signature": "sha256=..."
}
```

## Support

- API Issues: https://github.com/ssahani/tensor-reaper/issues
- SDK Documentation: https://github.com/ssahani/tensor-reaper/sdk
