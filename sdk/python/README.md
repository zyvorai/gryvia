# TensorReaper Python SDK

Python SDK for [TensorReaper](https://github.com/tensorreaper/kube-fabric) -- a Kubernetes-native GPU compute platform for AI/ML workloads.

## Installation

```bash
pip install tensorreaper
```

Or install from source:

```bash
cd sdk/python
pip install -e .
```

## Quick Start

```python
import asyncio
from tensorreaper import TensorReaper

async def main():
    async with TensorReaper(
        api_url="https://tensorreaper.example.com",
        token="your-api-token",
    ) as tr:
        # Cluster overview
        stats = await tr.metrics.cluster_stats()
        print(f"GPUs: {stats.available_gpus}/{stats.total_gpus} available")
        print(f"Running jobs: {stats.running_jobs}")

        # List jobs
        jobs = await tr.jobs.list(limit=10)
        for job in jobs.items:
            print(f"  {job.metadata.name}: {job.status.phase}")

asyncio.run(main())
```

## Configuration

The client accepts explicit parameters or reads from environment variables:

| Parameter | Env Var | Description |
|-----------|---------|-------------|
| `api_url` | `TENSORREAPER_API_URL` | Base URL of the API gateway |
| `token` | `TENSORREAPER_TOKEN` | Bearer token for authentication |

```python
# Explicit
tr = TensorReaper(api_url="https://...", token="...")

# From environment
# export TENSORREAPER_API_URL=https://...
# export TENSORREAPER_TOKEN=...
tr = TensorReaper()
```

## Usage

### Job Management

```python
async with TensorReaper(...) as tr:
    # Submit a job from a dict
    job = await tr.jobs.create({
        "apiVersion": "tensorreaper.ai/v1",
        "kind": "FabricAIJob",
        "metadata": {"name": "my-training"},
        "spec": {
            "image": "nvcr.io/nvidia/pytorch:24.01-py3",
            "gpus": 4,
            "gpuType": "A100-80G",
            "framework": "pytorch",
            "command": ["torchrun", "--nproc_per_node=4", "train.py"],
        },
    })

    # Submit from a YAML file
    job = await tr.jobs.create("jobs/my-training.yaml")

    # Wait for completion (polls every 5s, times out after 1h)
    final = await tr.jobs.wait_for_completion("my-training", timeout=7200)
    print(f"Job finished: {final.status.phase}")

    # Get job details
    job = await tr.jobs.get("my-training")

    # Delete a job
    await tr.jobs.delete("my-training")

    # Job metrics
    metrics = await tr.jobs.metrics(time_range="24h")
    print(f"Average duration: {metrics.average_duration_hours}h")
```

### GPU Metrics

```python
async with TensorReaper(...) as tr:
    # Cluster stats
    stats = await tr.metrics.cluster_stats()

    # Per-GPU metrics
    gpu = await tr.metrics.gpu(time_range="1h")
    for m in gpu.metrics:
        print(f"  {m.node} GPU{m.gpu_index}: {m.utilization}% util, {m.temperature}C")
```

### Nodes

```python
async with TensorReaper(...) as tr:
    # List nodes
    nodes = await tr.nodes.list()

    # Get a specific node
    node = await tr.nodes.get("gpu-node-01")

    # Node health checks
    health = await tr.nodes.health()
    for n in health.nodes:
        print(f"  {n.node_name}: {n.health} ({n.gpu_type} x{n.gpu_count})")
        for issue in n.issues:
            print(f"    WARNING: {issue}")
```

### Quotas

```python
async with TensorReaper(...) as tr:
    # List quotas
    quotas = await tr.quotas.list()

    # Get a specific quota
    quota = await tr.quotas.get("team-alpha")

    # Quota usage summary
    usage = await tr.quotas.usage()
    for q in usage.quotas:
        print(f"  {q.team}: {q.allocated_gpus}/{q.max_gpus} GPUs, "
              f"${q.spent_this_month:.2f}/${q.monthly_budget:.2f} budget")
```

### Cost Analysis

```python
async with TensorReaper(...) as tr:
    # Full cost metrics
    costs = await tr.costs.get()
    print(f"Total cost: ${costs.total_cost:.2f}")

    # Convenience methods
    total = await tr.costs.total()
    by_team = await tr.costs.by_team()
    by_gpu = await tr.costs.by_gpu_type()
```

## Error Handling

```python
from tensorreaper import TensorReaper, NotFoundError, AuthenticationError

async with TensorReaper(...) as tr:
    try:
        job = await tr.jobs.get("nonexistent")
    except NotFoundError as e:
        print(f"Not found: {e}")
    except AuthenticationError:
        print("Bad token")
```

All exceptions inherit from `TensorReaperError`:

- `AuthenticationError` (401)
- `AuthorizationError` (403)
- `NotFoundError` (404)
- `ValidationError` (400)
- `RateLimitError` (429)
- `ServerError` (5xx)
- `TimeoutError` (polling timeout)
- `ConfigurationError` (missing api_url/token)

## Development

```bash
pip install -e ".[dev]"
pytest
```
