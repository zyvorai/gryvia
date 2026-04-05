use serde::{Deserialize, Serialize};
use k8s_openapi::apimachinery::pkg::apis::meta::v1::ObjectMeta;
use kube::CustomResource;
use chrono::{DateTime, Utc};

// FabricAIJob CRD
#[derive(CustomResource, Clone, Debug, Serialize, Deserialize)]
#[kube(group = "kubefabric.ai", version = "v1", kind = "FabricAIJob", namespaced)]
#[kube(status = "AIJobStatus")]
pub struct AIJobSpec {
    pub framework: String,
    pub resources: ResourceSpec,
    #[serde(default)]
    pub distributed: DistributedConfig,
    pub image: String,
    pub command: Vec<String>,
    #[serde(default)]
    pub env: Vec<EnvVar>,
}


#[derive(Clone, Debug, Serialize, Deserialize, Default)]
pub struct ResourceSpec {
    #[serde(rename = "gpuType")]
    pub gpu_type: String,
    #[serde(rename = "gpuCount")]
    pub gpu_count: u32,
    pub memory: String,
    pub cpu: u32,
}

#[derive(Clone, Debug, Serialize, Deserialize, Default)]
pub struct DistributedConfig {
    pub enabled: bool,
    #[serde(default)]
    pub strategy: String,
    #[serde(rename = "worldSize", default)]
    pub world_size: u32,
}

#[derive(Clone, Debug, Serialize, Deserialize, Default)]
pub struct EnvVar {
    pub name: String,
    pub value: String,
}

#[derive(Clone, Debug, Serialize, Deserialize, Default)]
pub struct AIJobStatus {
    #[serde(default)]
    pub phase: String,
    #[serde(default)]
    pub message: String,
    #[serde(rename = "startTime")]
    pub start_time: Option<DateTime<Utc>>,
    #[serde(rename = "completionTime")]
    pub completion_time: Option<DateTime<Utc>>,
}

// FabricQuota CRD
#[derive(CustomResource, Clone, Debug, Serialize, Deserialize)]
#[kube(group = "kubefabric.ai", version = "v1", kind = "FabricQuota")]
#[kube(status = "QuotaStatus")]
pub struct QuotaSpec {
    pub team: String,
    pub namespaces: Vec<String>,
    #[serde(rename = "gpuQuota")]
    pub gpu_quota: GPUQuotaSpec,
    #[serde(default)]
    pub budget: Option<BudgetSpec>,
    #[serde(default)]
    pub priority: i32,
}

#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct GPUQuotaSpec {
    #[serde(rename = "maxGPUs")]
    pub max_gpus: u32,
    #[serde(rename = "maxGPUsPerJob", default)]
    pub max_gpus_per_job: u32,
    #[serde(rename = "allowedGPUTypes", default)]
    pub allowed_gpu_types: Vec<String>,
    #[serde(rename = "maxRunningJobs", default)]
    pub max_running_jobs: u32,
}

#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct BudgetSpec {
    #[serde(rename = "monthlyBudget")]
    pub monthly_budget: f64,
    #[serde(rename = "alertThreshold", default)]
    pub alert_threshold: f64,
    #[serde(rename = "hardLimit", default)]
    pub hard_limit: bool,
}

#[derive(Clone, Debug, Serialize, Deserialize, Default)]
pub struct QuotaStatus {
    #[serde(default)]
    pub phase: String,
    #[serde(rename = "currentUsage", default)]
    pub current_usage: QuotaUsage,
    #[serde(rename = "budgetStatus")]
    pub budget_status: Option<BudgetStatus>,
}

#[derive(Clone, Debug, Serialize, Deserialize, Default)]
pub struct QuotaUsage {
    #[serde(rename = "allocatedGPUs", default)]
    pub allocated_gpus: u32,
    #[serde(rename = "runningJobs", default)]
    pub running_jobs: u32,
    #[serde(rename = "queuedJobs", default)]
    pub queued_jobs: u32,
    #[serde(rename = "gpuHours", default)]
    pub gpu_hours: f64,
}

#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct BudgetStatus {
    #[serde(rename = "spentThisMonth")]
    pub spent_this_month: f64,
    #[serde(rename = "remainingBudget")]
    pub remaining_budget: f64,
    #[serde(rename = "percentUsed")]
    pub percent_used: f64,
    #[serde(rename = "projectedSpend")]
    pub projected_spend: f64,
}

// FabricGpuNode CRD
#[derive(CustomResource, Clone, Debug, Serialize, Deserialize)]
#[kube(group = "kubefabric.ai", version = "v1", kind = "FabricGpuNode")]
#[kube(status = "GpuNodeStatus")]
pub struct GpuNodeSpec {
    #[serde(rename = "nodeName")]
    pub node_name: String,
    #[serde(rename = "gpuType")]
    pub gpu_type: String,
    #[serde(rename = "gpuCount")]
    pub gpu_count: u32,
    pub memory: String,
    #[serde(rename = "rdmaEnabled", default)]
    pub rdma_enabled: bool,
}

#[derive(Clone, Debug, Serialize, Deserialize, Default)]
pub struct GpuNodeStatus {
    #[serde(default)]
    pub phase: String,
    #[serde(default)]
    pub gpus: Vec<GPUInfo>,
}

#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct GPUInfo {
    pub index: i32,
    pub uuid: String,
    pub temperature: f64,
    pub utilization: f64,
    #[serde(rename = "memoryUsed")]
    pub memory_used: i64,
    #[serde(rename = "memoryTotal")]
    pub memory_total: i64,
}
