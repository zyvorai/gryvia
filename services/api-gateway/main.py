"""
KubeFabric API Gateway
Provides REST API for Web UI with aggregated metrics and cluster data
"""
import asyncio
import hmac
import os
from datetime import datetime, timezone
from typing import Dict, List, Optional
from fastapi import FastAPI, HTTPException, Query, Depends, Header, Request
from fastapi.middleware.cors import CORSMiddleware
from kubernetes import client, config
from slowapi import Limiter, _rate_limit_exceeded_handler
from slowapi.util import get_remote_address
from slowapi.errors import RateLimitExceeded
import logging
from collections import defaultdict

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

limiter = Limiter(key_func=get_remote_address)

app = FastAPI(
    title="KubeFabric API Gateway",
    description="REST API for KubeFabric Web UI",
    version="1.0.0"
)
app.state.limiter = limiter
app.add_exception_handler(RateLimitExceeded, _rate_limit_exceeded_handler)

# CORS middleware - restrict origins via environment variable
ALLOWED_ORIGINS = os.environ.get("CORS_ALLOWED_ORIGINS", "").split(",")
if ALLOWED_ORIGINS == [""]:
    ALLOWED_ORIGINS = []

app.add_middleware(
    CORSMiddleware,
    allow_origins=ALLOWED_ORIGINS,
    allow_credentials=len(ALLOWED_ORIGINS) > 0,
    allow_methods=["GET", "POST", "PUT", "DELETE"],
    allow_headers=["Authorization", "Content-Type"],
)

# API key for authentication (from environment or mounted secret)
API_KEY = os.environ.get("KUBEFABRIC_API_KEY", "").strip()

async def verify_auth(authorization: Optional[str] = Header(None)):
    """Verify API key or Bearer token for all protected endpoints."""
    if not API_KEY:
        raise HTTPException(
            status_code=500,
            detail="KUBEFABRIC_API_KEY not configured. Set the environment variable to enable API access."
        )
    if not authorization:
        raise HTTPException(status_code=401, detail="Authorization header required")
    # Support both "Bearer <token>" and raw key
    token = authorization.removeprefix("Bearer ").strip()
    if not hmac.compare_digest(token, API_KEY):
        raise HTTPException(status_code=403, detail="Invalid credentials")

# Initialize Kubernetes client
try:
    config.load_incluster_config()
    logger.info("Loaded in-cluster Kubernetes config")
except config.ConfigException:
    try:
        config.load_kube_config()
        logger.info("Loaded local kubeconfig")
    except config.ConfigException as e:
        logger.error("Failed to load Kubernetes config: %s", e)
        raise

k8s_custom = client.CustomObjectsApi()
k8s_core = client.CoreV1Api()

# Prometheus client (optional - used for historical metrics when available)
PROMETHEUS_URL = os.environ.get("PROMETHEUS_URL", "http://prometheus-operated.kubefabric:9090")
prom = None
try:
    from prometheus_api_client import PrometheusConnect
    if PROMETHEUS_URL:
        prom = PrometheusConnect(url=PROMETHEUS_URL, disable_ssl=PROMETHEUS_URL.startswith("http://"))
        logger.info("Connected to Prometheus at %s", PROMETHEUS_URL)
except Exception:
    logger.warning("Failed to connect to Prometheus at %s - historical metrics unavailable", PROMETHEUS_URL)

# Namespace for job queries (configurable)
JOB_NAMESPACE = os.environ.get("KUBEFABRIC_JOB_NAMESPACE", "default")

# GPU pricing (same as quota operator)
GPU_PRICING = {
    "H100": 8.00,
    "A100-80G": 4.00,
    "A100-40G": 3.50,
    "L40": 2.50,
    "V100": 2.00,
    "T4": 1.00,
}


@app.get("/")
async def root():
    return {"status": "healthy", "service": "kubefabric-api-gateway"}


@app.get("/health")
async def health():
    return {"status": "ok"}


@app.get("/api/cluster/stats")
@limiter.limit("30/minute")
async def get_cluster_stats(request: Request, _=Depends(verify_auth)):
    """Get overall cluster statistics"""
    try:
        loop = asyncio.get_running_loop()

        # Get all GPU nodes
        nodes = await loop.run_in_executor(
            None,
            lambda: k8s_custom.list_cluster_custom_object(
                group="kubefabric.ai",
                version="v1",
                plural="fabricgpunodes"
            )
        )

        # Get all jobs
        jobs = await loop.run_in_executor(
            None,
            lambda: k8s_custom.list_namespaced_custom_object(
                group="kubefabric.ai",
                version="v1",
                namespace=JOB_NAMESPACE,
                plural="fabricaijobs"
            )
        )

        total_gpus = 0
        available_gpus = 0
        allocated_gpus = 0
        gpu_utilization_sum = 0
        gpu_count = 0

        for node in nodes.get("items", []):
            node_gpus = node.get("spec", {}).get("gpuCount", 0)
            total_gpus += node_gpus

            # Get GPU metrics from status
            status = node.get("status", {})
            for gpu in status.get("gpus", []):
                gpu_utilization_sum += gpu.get("utilization", 0)
                gpu_count += 1

        # Count allocated GPUs from running jobs
        for job in jobs.get("items", []):
            status = job.get("status", {})
            if status.get("phase") == "Running":
                allocated_gpus += job.get("spec", {}).get("gpus", job.get("spec", {}).get("resources", {}).get("gpuCount", 0))

        available_gpus = max(0, total_gpus - allocated_gpus)
        avg_utilization = gpu_utilization_sum / gpu_count if gpu_count > 0 else 0

        # Count jobs by status
        job_counts = defaultdict(int)
        for job in jobs.get("items", []):
            phase = job.get("status", {}).get("phase", "Unknown")
            job_counts[phase] += 1

        return {
            "totalGPUs": total_gpus,
            "availableGPUs": available_gpus,
            "allocatedGPUs": allocated_gpus,
            "utilizationPercent": round(avg_utilization, 1),
            "totalJobs": len(jobs.get("items", [])),
            "runningJobs": job_counts.get("Running", 0),
            "pendingJobs": job_counts.get("Pending", 0) + job_counts.get("Queued", 0),
            "completedJobs": job_counts.get("Completed", 0),
            "failedJobs": job_counts.get("Failed", 0),
            "totalNodes": len(nodes.get("items", [])),
            "timestamp": datetime.now(timezone.utc).isoformat()
        }
    except Exception as e:
        logger.error("Error getting cluster stats: %s", e, exc_info=True)
        raise HTTPException(status_code=500, detail="Failed to retrieve cluster stats")


@app.get("/api/metrics/gpu")
@limiter.limit("30/minute")
async def get_gpu_metrics(
    request: Request,
    time_range: str = Query("1h", description="Time range (1h, 6h, 24h, 7d) - reserved for Prometheus integration"),
    _=Depends(verify_auth),
):
    """Get GPU utilization metrics over time"""
    try:
        loop = asyncio.get_running_loop()

        # Query GPU node status for current metrics
        nodes = await loop.run_in_executor(
            None,
            lambda: k8s_custom.list_cluster_custom_object(
                group="kubefabric.ai",
                version="v1",
                plural="fabricgpunodes"
            )
        )

        metrics = []
        for node in nodes.get("items", []):
            spec = node.get("spec", {})
            status = node.get("status", {})
            for gpu in status.get("gpus", []):
                metrics.append({
                    "node": spec.get("nodeName", "unknown"),
                    "gpuIndex": gpu.get("index", 0),
                    "utilization": gpu.get("utilization", 0),
                    "temperature": gpu.get("temperature", 0),
                    "memoryUsed": gpu.get("memoryUsed", 0),
                    "memoryTotal": gpu.get("memoryTotal", 0),
                    "timestamp": datetime.now(timezone.utc).isoformat()
                })

        return {
            "timeRange": time_range,
            "metrics": metrics
        }
    except Exception as e:
        logger.error("Error getting GPU metrics: %s", e, exc_info=True)
        raise HTTPException(status_code=500, detail="Failed to retrieve GPU metrics")


@app.get("/api/metrics/costs")
@limiter.limit("30/minute")
async def get_cost_metrics(request: Request, _=Depends(verify_auth)):
    """Get cost metrics and analysis"""
    try:
        loop = asyncio.get_running_loop()

        # Get all quotas with budget info
        quotas = await loop.run_in_executor(
            None,
            lambda: k8s_custom.list_cluster_custom_object(
                group="kubefabric.ai",
                version="v1",
                plural="fabricquotas"
            )
        )

        # Get all jobs to calculate costs
        jobs = await loop.run_in_executor(
            None,
            lambda: k8s_custom.list_namespaced_custom_object(
                group="kubefabric.ai",
                version="v1",
                namespace=JOB_NAMESPACE,
                plural="fabricaijobs"
            )
        )

        # Calculate costs by team
        team_costs = defaultdict(float)
        gpu_type_costs = defaultdict(lambda: {"cost": 0.0, "hours": 0.0})

        for job in jobs.get("items", []):
            status = job.get("status", {})
            spec = job.get("spec", {})

            if status.get("phase") in ["Running", "Completed"]:
                gpu_type = spec.get("gpuType", spec.get("resources", {}).get("gpuType", "unknown"))
                gpu_count = spec.get("gpus", spec.get("resources", {}).get("gpuCount", 0))

                # Calculate hours
                start_time = status.get("startTime")
                end_time = status.get("completionTime") or datetime.now(timezone.utc).isoformat()

                if start_time:
                    start = datetime.fromisoformat(start_time.replace('Z', '+00:00'))
                    end = datetime.fromisoformat(end_time.replace('Z', '+00:00'))
                    hours = (end - start).total_seconds() / 3600

                    cost = hours * gpu_count * GPU_PRICING.get(gpu_type, 1.0)

                    # Add to team costs (use namespace or label as team identifier)
                    team = job.get("metadata", {}).get("labels", {}).get("team", "default")
                    team_costs[team] += cost

                    # Add to GPU type costs
                    gpu_type_costs[gpu_type]["cost"] += cost
                    gpu_type_costs[gpu_type]["hours"] += hours * gpu_count

        current_month_cost = sum(team_costs.values())

        # Historical monthly data requires Prometheus integration
        monthly_data = []
        has_historical_data = False

        # Format team costs
        by_team = [
            {"team": team, "cost": round(cost, 2)}
            for team, cost in team_costs.items()
        ]

        # Format GPU type costs
        by_gpu_type = [
            {
                "type": gpu_type,
                "cost": round(data["cost"], 2),
                "hours": round(data["hours"], 1)
            }
            for gpu_type, data in gpu_type_costs.items()
        ]

        return {
            "monthly": monthly_data,
            "hasHistoricalData": has_historical_data,
            "byTeam": by_team,
            "byGPUType": by_gpu_type,
            "totalCost": round(current_month_cost, 2),
            "timestamp": datetime.now(timezone.utc).isoformat()
        }
    except Exception as e:
        logger.error("Error getting cost metrics: %s", e, exc_info=True)
        raise HTTPException(status_code=500, detail="Failed to retrieve cost metrics")


@app.get("/api/metrics/jobs")
@limiter.limit("30/minute")
async def get_job_metrics(
    request: Request,
    time_range: str = Query("24h", description="Time range"),
    _=Depends(verify_auth),
):
    """Get job metrics over time"""
    try:
        loop = asyncio.get_running_loop()

        jobs = await loop.run_in_executor(
            None,
            lambda: k8s_custom.list_namespaced_custom_object(
                group="kubefabric.ai",
                version="v1",
                namespace=JOB_NAMESPACE,
                plural="fabricaijobs"
            )
        )

        # Calculate job statistics
        total_jobs = len(jobs.get("items", []))
        by_status = defaultdict(int)
        by_framework = defaultdict(int)
        avg_duration = 0
        duration_count = 0

        for job in jobs.get("items", []):
            status = job.get("status", {})
            spec = job.get("spec", {})

            phase = status.get("phase", "Unknown")
            by_status[phase] += 1

            framework = spec.get("framework", spec.get("type", "unknown"))
            by_framework[framework] += 1

            # Calculate duration for completed jobs
            if status.get("startTime") and status.get("completionTime"):
                start = datetime.fromisoformat(status["startTime"].replace('Z', '+00:00'))
                end = datetime.fromisoformat(status["completionTime"].replace('Z', '+00:00'))
                duration = (end - start).total_seconds() / 3600
                avg_duration += duration
                duration_count += 1

        avg_duration = avg_duration / duration_count if duration_count > 0 else 0

        return {
            "totalJobs": total_jobs,
            "byStatus": dict(by_status),
            "byFramework": dict(by_framework),
            "averageDurationHours": round(avg_duration, 2),
            "timestamp": datetime.now(timezone.utc).isoformat()
        }
    except Exception as e:
        logger.error("Error getting job metrics: %s", e, exc_info=True)
        raise HTTPException(status_code=500, detail="Failed to retrieve job metrics")


@app.get("/api/jobs")
@limiter.limit("30/minute")
async def list_jobs(
    request: Request,
    limit: int = Query(100, ge=1, le=1000),
    offset: int = Query(0, ge=0),
    _=Depends(verify_auth),
):
    """List all jobs with pagination"""
    try:
        loop = asyncio.get_running_loop()

        jobs = await loop.run_in_executor(
            None,
            lambda: k8s_custom.list_namespaced_custom_object(
                group="kubefabric.ai",
                version="v1",
                namespace=JOB_NAMESPACE,
                plural="fabricaijobs"
            )
        )

        all_items = jobs.get("items", [])
        total = len(all_items)
        items = all_items[offset:offset + limit]

        return {
            "items": items,
            "total": total,
            "limit": limit,
            "offset": offset,
        }
    except Exception as e:
        logger.error("Error listing jobs: %s", e, exc_info=True)
        raise HTTPException(status_code=500, detail="Failed to list jobs")


@app.get("/api/jobs/{name}")
@limiter.limit("30/minute")
async def get_job(request: Request, name: str, _=Depends(verify_auth)):
    """Get a specific job by name"""
    try:
        loop = asyncio.get_running_loop()

        job = await loop.run_in_executor(
            None,
            lambda: k8s_custom.get_namespaced_custom_object(
                group="kubefabric.ai",
                version="v1",
                namespace=JOB_NAMESPACE,
                plural="fabricaijobs",
                name=name,
            )
        )
        return job
    except client.ApiException as e:
        if e.status == 404:
            raise HTTPException(status_code=404, detail=f"Job '{name}' not found")
        raise HTTPException(status_code=500, detail="Failed to get job")
    except Exception as e:
        logger.error("Error getting job %s: %s", name, e, exc_info=True)
        raise HTTPException(status_code=500, detail="Failed to get job")


@app.post("/api/jobs")
@limiter.limit("10/minute")
async def create_job(request: Request, _=Depends(verify_auth)):
    """Create a new job"""
    try:
        loop = asyncio.get_running_loop()
        body = await request.json()

        # Validate required fields
        if not isinstance(body, dict):
            raise HTTPException(status_code=400, detail="Request body must be a JSON object")
        if body.get("apiVersion") != "kubefabric.ai/v1":
            raise HTTPException(status_code=400, detail="apiVersion must be kubefabric.ai/v1")
        if body.get("kind") != "FabricAIJob":
            raise HTTPException(status_code=400, detail="kind must be FabricAIJob")

        # Enforce namespace server-side to prevent namespace bypass
        body.setdefault("metadata", {})["namespace"] = JOB_NAMESPACE

        job = await loop.run_in_executor(
            None,
            lambda: k8s_custom.create_namespaced_custom_object(
                group="kubefabric.ai",
                version="v1",
                namespace=JOB_NAMESPACE,
                plural="fabricaijobs",
                body=body,
            )
        )
        return job
    except HTTPException:
        raise
    except client.ApiException as e:
        raise HTTPException(status_code=e.status or 500, detail="Failed to create job")
    except Exception as e:
        logger.error("Error creating job: %s", e, exc_info=True)
        raise HTTPException(status_code=500, detail="Failed to create job")


@app.delete("/api/jobs/{name}")
@limiter.limit("10/minute")
async def delete_job(request: Request, name: str, _=Depends(verify_auth)):
    """Delete a job by name"""
    try:
        loop = asyncio.get_running_loop()

        await loop.run_in_executor(
            None,
            lambda: k8s_custom.delete_namespaced_custom_object(
                group="kubefabric.ai",
                version="v1",
                namespace=JOB_NAMESPACE,
                plural="fabricaijobs",
                name=name,
            )
        )
        return {"status": "deleted", "name": name}
    except client.ApiException as e:
        if e.status == 404:
            raise HTTPException(status_code=404, detail=f"Job '{name}' not found")
        raise HTTPException(status_code=500, detail="Failed to delete job")
    except Exception as e:
        logger.error("Error deleting job %s: %s", name, e, exc_info=True)
        raise HTTPException(status_code=500, detail="Failed to delete job")


@app.get("/api/quotas")
@limiter.limit("30/minute")
async def list_quotas(
    request: Request,
    limit: int = Query(100, ge=1, le=1000),
    offset: int = Query(0, ge=0),
    _=Depends(verify_auth),
):
    """List all quotas with pagination"""
    try:
        loop = asyncio.get_running_loop()

        quotas = await loop.run_in_executor(
            None,
            lambda: k8s_custom.list_cluster_custom_object(
                group="kubefabric.ai",
                version="v1",
                plural="fabricquotas"
            )
        )

        all_items = quotas.get("items", [])
        total = len(all_items)
        items = all_items[offset:offset + limit]

        return {
            "items": items,
            "total": total,
            "limit": limit,
            "offset": offset,
        }
    except Exception as e:
        logger.error("Error listing quotas: %s", e, exc_info=True)
        raise HTTPException(status_code=500, detail="Failed to list quotas")


@app.get("/api/quotas/{name}")
@limiter.limit("30/minute")
async def get_quota(request: Request, name: str, _=Depends(verify_auth)):
    """Get a specific quota by name"""
    try:
        loop = asyncio.get_running_loop()

        quota = await loop.run_in_executor(
            None,
            lambda: k8s_custom.get_cluster_custom_object(
                group="kubefabric.ai",
                version="v1",
                plural="fabricquotas",
                name=name,
            )
        )
        return quota
    except client.ApiException as e:
        if e.status == 404:
            raise HTTPException(status_code=404, detail=f"Quota '{name}' not found")
        raise HTTPException(status_code=500, detail="Failed to get quota")
    except Exception as e:
        logger.error("Error getting quota %s: %s", name, e, exc_info=True)
        raise HTTPException(status_code=500, detail="Failed to get quota")


@app.get("/api/nodes")
@limiter.limit("30/minute")
async def list_nodes(
    request: Request,
    limit: int = Query(100, ge=1, le=1000),
    offset: int = Query(0, ge=0),
    _=Depends(verify_auth),
):
    """List all GPU nodes with pagination"""
    try:
        loop = asyncio.get_running_loop()

        nodes = await loop.run_in_executor(
            None,
            lambda: k8s_custom.list_cluster_custom_object(
                group="kubefabric.ai",
                version="v1",
                plural="fabricgpunodes"
            )
        )

        all_items = nodes.get("items", [])
        total = len(all_items)
        items = all_items[offset:offset + limit]

        return {
            "items": items,
            "total": total,
            "limit": limit,
            "offset": offset,
        }
    except Exception as e:
        logger.error("Error listing nodes: %s", e, exc_info=True)
        raise HTTPException(status_code=500, detail="Failed to list nodes")


@app.get("/api/nodes/{name}")
@limiter.limit("30/minute")
async def get_node(request: Request, name: str, _=Depends(verify_auth)):
    """Get a specific GPU node by name"""
    try:
        loop = asyncio.get_running_loop()

        node = await loop.run_in_executor(
            None,
            lambda: k8s_custom.get_cluster_custom_object(
                group="kubefabric.ai",
                version="v1",
                plural="fabricgpunodes",
                name=name,
            )
        )
        return node
    except client.ApiException as e:
        if e.status == 404:
            raise HTTPException(status_code=404, detail=f"Node '{name}' not found")
        raise HTTPException(status_code=500, detail="Failed to get node")
    except Exception as e:
        logger.error("Error getting node %s: %s", name, e, exc_info=True)
        raise HTTPException(status_code=500, detail="Failed to get node")


@app.get("/api/quota/usage")
@limiter.limit("30/minute")
async def get_quota_usage(
    request: Request,
    limit: int = Query(100, ge=1, le=1000),
    offset: int = Query(0, ge=0),
    _=Depends(verify_auth),
):
    """Get quota usage across all teams"""
    try:
        loop = asyncio.get_running_loop()

        quotas = await loop.run_in_executor(
            None,
            lambda: k8s_custom.list_cluster_custom_object(
                group="kubefabric.ai",
                version="v1",
                plural="fabricquotas"
            )
        )

        usage_data = []
        for quota in quotas.get("items", []):
            spec = quota.get("spec", {})
            status = quota.get("status", {})
            current_usage = status.get("currentUsage", {})
            budget_status = status.get("budgetStatus", {})
            gpu_quota = spec.get("gpuQuota", {})
            max_gpus = gpu_quota.get("maxGPUs", 0)

            usage_data.append({
                "team": spec.get("team", "unknown"),
                "maxGPUs": max_gpus,
                "allocatedGPUs": current_usage.get("allocatedGPUs", 0),
                "utilizationPercent": round(
                    (current_usage.get("allocatedGPUs", 0) / max_gpus) * 100, 1
                ) if max_gpus > 0 else 0,
                "runningJobs": current_usage.get("runningJobs", 0),
                "queuedJobs": current_usage.get("queuedJobs", 0),
                "monthlyBudget": spec.get("budget", {}).get("monthlyBudget", 0),
                "spentThisMonth": budget_status.get("spentThisMonth", 0),
                "remainingBudget": budget_status.get("remainingBudget", 0)
            })

        total = len(usage_data)
        usage_data = usage_data[offset:offset + limit]

        return {
            "quotas": usage_data,
            "total": total,
            "limit": limit,
            "offset": offset,
            "timestamp": datetime.now(timezone.utc).isoformat()
        }
    except Exception as e:
        logger.error("Error getting quota usage: %s", e, exc_info=True)
        raise HTTPException(status_code=500, detail="Failed to retrieve quota usage")


@app.get("/api/nodes/health")
@limiter.limit("30/minute")
async def get_node_health(
    request: Request,
    limit: int = Query(100, ge=1, le=1000),
    offset: int = Query(0, ge=0),
    _=Depends(verify_auth),
):
    """Get GPU node health status"""
    try:
        loop = asyncio.get_running_loop()

        nodes = await loop.run_in_executor(
            None,
            lambda: k8s_custom.list_cluster_custom_object(
                group="kubefabric.ai",
                version="v1",
                plural="fabricgpunodes"
            )
        )

        health_data = []
        for node in nodes.get("items", []):
            spec = node.get("spec", {})
            status = node.get("status", {})

            # Calculate node health based on GPU metrics
            gpu_health = "Healthy"
            issues = []

            for gpu in status.get("gpus", []):
                temp = gpu.get("temperature", 0)
                if temp > 90:
                    gpu_health = "Critical"
                    issues.append(f"GPU {gpu.get('index', '?')} critical temperature: {temp}C")
                elif temp > 85:
                    gpu_health = "Warning"
                    issues.append(f"GPU {gpu.get('index', '?')} high temperature: {temp}C")

            health_data.append({
                "nodeName": spec.get("nodeName", "unknown"),
                "gpuType": spec.get("gpuType", "unknown"),
                "gpuCount": spec.get("gpuCount", 0),
                "phase": status.get("phase", "Unknown"),
                "health": gpu_health,
                "issues": issues,
                "rdmaEnabled": spec.get("rdmaEnabled", False)
            })

        total = len(health_data)
        health_data = health_data[offset:offset + limit]

        return {
            "nodes": health_data,
            "total": total,
            "limit": limit,
            "offset": offset,
            "timestamp": datetime.now(timezone.utc).isoformat()
        }
    except Exception as e:
        logger.error("Error getting node health: %s", e, exc_info=True)
        raise HTTPException(status_code=500, detail="Failed to retrieve node health")


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8080)
