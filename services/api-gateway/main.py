"""
KubeFabric API Gateway
Provides REST API for Web UI with aggregated metrics and cluster data
"""
import asyncio
import os
from datetime import datetime
from typing import Dict, List, Optional
from fastapi import FastAPI, HTTPException, Query, Depends, Header, Request
from fastapi.middleware.cors import CORSMiddleware
from kubernetes import client, config
from prometheus_api_client import PrometheusConnect
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
API_KEY = os.environ.get("KUBEFABRIC_API_KEY", "")

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
    import hmac
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

# Prometheus client
PROMETHEUS_URL = os.environ.get("PROMETHEUS_URL", "http://prometheus-operated.kubefabric:9090")
prom = PrometheusConnect(url=PROMETHEUS_URL, disable_ssl=PROMETHEUS_URL.startswith("http://"))

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
        # Get all GPU nodes
        nodes = k8s_custom.list_cluster_custom_object(
            group="kubefabric.ai",
            version="v1",
            plural="fabricgpunodes"
        )

        # Get all jobs
        jobs = k8s_custom.list_namespaced_custom_object(
            group="kubefabric.ai",
            version="v1",
            namespace=JOB_NAMESPACE,
            plural="fabricaijobs"
        )

        total_gpus = 0
        available_gpus = 0
        allocated_gpus = 0
        gpu_utilization_sum = 0
        gpu_count = 0

        for node in nodes.get("items", []):
            node_gpus = node["spec"]["gpuCount"]
            total_gpus += node_gpus

            # Get GPU metrics from status
            if "status" in node and "gpus" in node["status"]:
                for gpu in node["status"]["gpus"]:
                    gpu_utilization_sum += gpu.get("utilization", 0)
                    gpu_count += 1

        # Count allocated GPUs from running jobs
        for job in jobs.get("items", []):
            status = job.get("status", {})
            if status.get("phase") == "Running":
                allocated_gpus += job["spec"]["resources"]["gpuCount"]

        available_gpus = total_gpus - allocated_gpus
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
            "timestamp": datetime.utcnow().isoformat()
        }
    except Exception as e:
        logger.error("Error getting cluster stats: %s", e, exc_info=True)
        raise HTTPException(status_code=500, detail="Failed to retrieve cluster stats")


@app.get("/api/metrics/gpu")
@limiter.limit("30/minute")
async def get_gpu_metrics(
    request: Request,
    time_range: str = Query("1h", description="Time range (1h, 6h, 24h, 7d)"),
    _=Depends(verify_auth),
):
    """Get GPU utilization metrics over time"""
    try:
        # Parse time range
        range_map = {
            "1h": "1h",
            "6h": "6h",
            "24h": "1d",
            "7d": "7d"
        }
        prom_range = range_map.get(time_range, "1h")

        # Query Prometheus for GPU metrics
        nodes = k8s_custom.list_cluster_custom_object(
            group="kubefabric.ai",
            version="v1",
            plural="fabricgpunodes"
        )

        metrics = []
        for node in nodes.get("items", []):
            if "status" in node and "gpus" in node["status"]:
                for gpu in node["status"]["gpus"]:
                    metrics.append({
                        "node": node["spec"]["nodeName"],
                        "gpuIndex": gpu["index"],
                        "utilization": gpu.get("utilization", 0),
                        "temperature": gpu.get("temperature", 0),
                        "memoryUsed": gpu.get("memoryUsed", 0),
                        "memoryTotal": gpu.get("memoryTotal", 0),
                        "timestamp": datetime.utcnow().isoformat()
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
        # Get all quotas with budget info
        quotas = k8s_custom.list_cluster_custom_object(
            group="kubefabric.ai",
            version="v1",
            plural="fabricquotas"
        )

        # Get all jobs to calculate costs
        jobs = k8s_custom.list_namespaced_custom_object(
            group="kubefabric.ai",
            version="v1",
            namespace=JOB_NAMESPACE,
            plural="fabricaijobs"
        )

        # Calculate costs by team
        team_costs = defaultdict(float)
        gpu_type_costs = defaultdict(lambda: {"cost": 0.0, "hours": 0.0})

        for job in jobs.get("items", []):
            status = job.get("status", {})
            spec = job["spec"]

            if status.get("phase") in ["Running", "Completed"]:
                gpu_type = spec["resources"]["gpuType"]
                gpu_count = spec["resources"]["gpuCount"]

                # Calculate hours
                start_time = status.get("startTime")
                end_time = status.get("completionTime") or datetime.utcnow().isoformat()

                if start_time:
                    start = datetime.fromisoformat(start_time.replace('Z', '+00:00'))
                    end = datetime.fromisoformat(end_time.replace('Z', '+00:00'))
                    hours = (end - start).total_seconds() / 3600

                    cost = hours * gpu_count * GPU_PRICING.get(gpu_type, 1.0)

                    # Add to team costs (use namespace or label as team identifier)
                    team = job["metadata"].get("labels", {}).get("team", "default")
                    team_costs[team] += cost

                    # Add to GPU type costs
                    gpu_type_costs[gpu_type]["cost"] += cost
                    gpu_type_costs[gpu_type]["hours"] += hours * gpu_count

        # Generate monthly data (mock for now - in production, query historical data)
        current_month_cost = sum(team_costs.values())
        monthly_data = [
            {"month": "Jan", "cost": current_month_cost * 0.7},
            {"month": "Feb", "cost": current_month_cost * 0.8},
            {"month": "Mar", "cost": current_month_cost * 0.75},
            {"month": "Apr", "cost": current_month_cost * 0.9},
            {"month": "May", "cost": current_month_cost * 0.95},
            {"month": "Jun", "cost": current_month_cost},
        ]

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
            "byTeam": by_team,
            "byGPUType": by_gpu_type,
            "totalCost": round(current_month_cost, 2),
            "timestamp": datetime.utcnow().isoformat()
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
        jobs = k8s_custom.list_namespaced_custom_object(
            group="kubefabric.ai",
            version="v1",
            namespace=JOB_NAMESPACE,
            plural="fabricaijobs"
        )

        # Calculate job statistics
        total_jobs = len(jobs.get("items", []))
        by_status = defaultdict(int)
        by_framework = defaultdict(int)
        avg_duration = 0
        duration_count = 0

        for job in jobs.get("items", []):
            status = job.get("status", {})
            spec = job["spec"]

            phase = status.get("phase", "Unknown")
            by_status[phase] += 1

            framework = spec.get("framework", "unknown")
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
            "timestamp": datetime.utcnow().isoformat()
        }
    except Exception as e:
        logger.error("Error getting job metrics: %s", e, exc_info=True)
        raise HTTPException(status_code=500, detail="Failed to retrieve job metrics")


@app.get("/api/quota/usage")
@limiter.limit("30/minute")
async def get_quota_usage(request: Request, _=Depends(verify_auth)):
    """Get quota usage across all teams"""
    try:
        quotas = k8s_custom.list_cluster_custom_object(
            group="kubefabric.ai",
            version="v1",
            plural="fabricquotas"
        )

        usage_data = []
        for quota in quotas.get("items", []):
            spec = quota["spec"]
            status = quota.get("status", {})
            current_usage = status.get("currentUsage", {})
            budget_status = status.get("budgetStatus", {})

            usage_data.append({
                "team": spec["team"],
                "maxGPUs": spec["gpuQuota"]["maxGPUs"],
                "allocatedGPUs": current_usage.get("allocatedGPUs", 0),
                "utilizationPercent": round(
                    (current_usage.get("allocatedGPUs", 0) / spec["gpuQuota"]["maxGPUs"]) * 100, 1
                ) if spec["gpuQuota"]["maxGPUs"] > 0 else 0,
                "runningJobs": current_usage.get("runningJobs", 0),
                "queuedJobs": current_usage.get("queuedJobs", 0),
                "monthlyBudget": spec.get("budget", {}).get("monthlyBudget", 0),
                "spentThisMonth": budget_status.get("spentThisMonth", 0),
                "remainingBudget": budget_status.get("remainingBudget", 0)
            })

        return {
            "quotas": usage_data,
            "timestamp": datetime.utcnow().isoformat()
        }
    except Exception as e:
        logger.error("Error getting quota usage: %s", e, exc_info=True)
        raise HTTPException(status_code=500, detail="Failed to retrieve quota usage")


@app.get("/api/nodes/health")
@limiter.limit("30/minute")
async def get_node_health(request: Request, _=Depends(verify_auth)):
    """Get GPU node health status"""
    try:
        nodes = k8s_custom.list_cluster_custom_object(
            group="kubefabric.ai",
            version="v1",
            plural="fabricgpunodes"
        )

        health_data = []
        for node in nodes.get("items", []):
            spec = node["spec"]
            status = node.get("status", {})

            # Calculate node health based on GPU metrics
            gpu_health = "Healthy"
            issues = []

            if "gpus" in status:
                for gpu in status["gpus"]:
                    temp = gpu.get("temperature", 0)
                    if temp > 90:
                        gpu_health = "Critical"
                        issues.append(f"GPU {gpu['index']} critical temperature: {temp}C")
                    elif temp > 85:
                        gpu_health = "Warning"
                        issues.append(f"GPU {gpu['index']} high temperature: {temp}C")

            health_data.append({
                "nodeName": spec["nodeName"],
                "gpuType": spec["gpuType"],
                "gpuCount": spec["gpuCount"],
                "phase": status.get("phase", "Unknown"),
                "health": gpu_health,
                "issues": issues,
                "rdmaEnabled": spec.get("rdmaEnabled", False)
            })

        return {
            "nodes": health_data,
            "timestamp": datetime.utcnow().isoformat()
        }
    except Exception as e:
        logger.error("Error getting node health: %s", e, exc_info=True)
        raise HTTPException(status_code=500, detail="Failed to retrieve node health")


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8080)
