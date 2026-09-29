use chrono::{DateTime, Utc};
use kube::CustomResource;
use schemars::JsonSchema;
use serde::{Deserialize, Serialize};

// GryviaAIJob CRD
#[derive(CustomResource, Clone, Debug, Serialize, Deserialize, JsonSchema)]
#[kube(
    group = "gryvia.io",
    version = "v1alpha1",
    kind = "GryviaAIJob",
    namespaced
)]
#[kube(status = "AIJobStatus")]
pub struct AIJobSpec {
    /// training, inference, fine-tuning or evaluation
    #[serde(rename = "type", default)]
    pub job_type: String,
    #[serde(default)]
    pub gpus: u32,
    #[serde(rename = "gpuType", default)]
    pub gpu_type: String,
    #[serde(default)]
    pub image: String,
    #[serde(default)]
    pub command: Vec<String>,
    #[serde(default)]
    pub env: Vec<EnvVar>,
    #[serde(default)]
    pub resources: ResourceRequirements,
    #[serde(default)]
    pub distributed: DistributedConfig,
}

/// Kubernetes-style resource requests and limits (`cpu`, `memory`, ... as strings or numbers).
#[derive(Clone, Debug, Serialize, Deserialize, Default, JsonSchema)]
pub struct ResourceRequirements {
    #[serde(default)]
    pub requests: std::collections::BTreeMap<String, serde_json::Value>,
    #[serde(default)]
    pub limits: std::collections::BTreeMap<String, serde_json::Value>,
}

impl ResourceRequirements {
    /// Requested quantity of a resource (`cpu`, `memory`), or "-" when unset.
    pub fn request(&self, name: &str) -> String {
        match self.requests.get(name) {
            Some(serde_json::Value::String(s)) => s.clone(),
            Some(other) => other.to_string(),
            None => "-".to_string(),
        }
    }
}

#[derive(Clone, Debug, Serialize, Deserialize, Default, JsonSchema)]
pub struct DistributedConfig {
    #[serde(default)]
    pub enabled: bool,
    #[serde(default)]
    pub framework: String,
    #[serde(default)]
    pub backend: String,
    #[serde(default)]
    pub nodes: u32,
    #[serde(rename = "gpusPerNode", default)]
    pub gpus_per_node: u32,
}

#[derive(Clone, Debug, Serialize, Deserialize, Default, JsonSchema)]
pub struct EnvVar {
    pub name: String,
    #[serde(default)]
    pub value: String,
}

#[derive(Clone, Debug, Serialize, Deserialize, Default, JsonSchema)]
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

// GryviaQuota CRD
#[derive(CustomResource, Clone, Debug, Serialize, Deserialize, JsonSchema)]
#[kube(group = "gryvia.io", version = "v1alpha1", kind = "GryviaQuota")]
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

#[derive(Clone, Debug, Serialize, Deserialize, JsonSchema)]
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

#[derive(Clone, Debug, Serialize, Deserialize, JsonSchema)]
pub struct BudgetSpec {
    #[serde(rename = "monthlyBudget")]
    pub monthly_budget: f64,
    #[serde(rename = "alertThreshold", default)]
    pub alert_threshold: f64,
    #[serde(rename = "hardLimit", default)]
    pub hard_limit: bool,
}

#[derive(Clone, Debug, Serialize, Deserialize, Default, JsonSchema)]
pub struct QuotaStatus {
    #[serde(default)]
    pub phase: String,
    #[serde(rename = "currentUsage", default)]
    pub current_usage: QuotaUsage,
    #[serde(rename = "budgetStatus")]
    pub budget_status: Option<BudgetStatus>,
}

#[derive(Clone, Debug, Serialize, Deserialize, Default, JsonSchema)]
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

#[derive(Clone, Debug, Serialize, Deserialize, JsonSchema)]
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

// GryviaGpuNode CRD
#[derive(CustomResource, Clone, Debug, Serialize, Deserialize, JsonSchema)]
#[kube(group = "gryvia.io", version = "v1alpha1", kind = "GryviaGpuNode")]
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

#[derive(Clone, Debug, Serialize, Deserialize, Default, JsonSchema)]
pub struct GpuNodeStatus {
    #[serde(default)]
    pub phase: String,
    #[serde(default)]
    pub gpus: Vec<GPUInfo>,
}

#[derive(Clone, Debug, Serialize, Deserialize, JsonSchema)]
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

#[cfg(test)]
mod tests {
    use super::*;

    /// A job as the API returns it: the real CRD fields, no `framework` and no `resources.gpuCount`.
    #[test]
    fn parses_a_real_job() {
        let job: GryviaAIJob = serde_json::from_value(serde_json::json!({
            "apiVersion": "gryvia.io/v1alpha1",
            "kind": "GryviaAIJob",
            "metadata": {"name": "train", "namespace": "ml"},
            "spec": {"type": "training", "gpus": 4, "gpuType": "H100", "image": "busybox:1.36",
                     "resources": {"requests": {"cpu": "4", "memory": "16Gi"}},
                     "distributed": {"enabled": true, "framework": "pytorch", "nodes": 2, "gpusPerNode": 2}},
            "status": {"phase": "Running"}
        }))
        .unwrap();
        assert_eq!(job.spec.gpus, 4);
        assert_eq!(job.spec.gpu_type, "H100");
        assert_eq!(job.spec.job_type, "training");
        assert_eq!(job.spec.distributed.nodes, 2);
        assert_eq!(job.spec.resources.request("memory"), "16Gi");
        assert_eq!(job.spec.resources.request("cpu"), "4");
        assert_eq!(job.spec.resources.request("ephemeral-storage"), "-");
    }

    #[test]
    fn a_minimal_job_still_parses() {
        let job: GryviaAIJob = serde_json::from_value(serde_json::json!({
            "apiVersion": "gryvia.io/v1alpha1", "kind": "GryviaAIJob",
            "metadata": {"name": "j"},
            "spec": {"type": "training", "gpus": 1, "image": "x"}
        }))
        .unwrap();
        assert_eq!(job.spec.gpus, 1);
        assert!(job.spec.command.is_empty());
    }
}
