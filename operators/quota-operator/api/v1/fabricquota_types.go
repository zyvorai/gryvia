package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FabricQuotaSpec defines the desired state of FabricQuota
type FabricQuotaSpec struct {
	// Team is the name of the team this quota applies to
	Team string `json:"team"`

	// Namespaces that this quota applies to
	Namespaces []string `json:"namespaces"`

	// GPUQuota defines GPU allocation limits
	GPUQuota GPUQuotaSpec `json:"gpuQuota"`

	// Budget defines cost limits (optional)
	Budget *BudgetSpec `json:"budget,omitempty"`

	// Priority for this team (1-100, higher is more important)
	Priority int `json:"priority,omitempty"`
}

// GPUQuotaSpec defines GPU resource limits
type GPUQuotaSpec struct {
	// MaxGPUs is the maximum number of GPUs this team can use
	MaxGPUs int `json:"maxGPUs"`

	// MaxGPUsPerJob limits GPUs per individual job
	MaxGPUsPerJob int `json:"maxGPUsPerJob,omitempty"`

	// AllowedGPUTypes restricts which GPU models can be used
	AllowedGPUTypes []string `json:"allowedGPUTypes,omitempty"`

	// MaxRunningJobs limits concurrent jobs
	MaxRunningJobs int `json:"maxRunningJobs,omitempty"`

	// MaxQueuedJobs limits queued jobs
	MaxQueuedJobs int `json:"maxQueuedJobs,omitempty"`
}

// BudgetSpec defines cost limits
type BudgetSpec struct {
	// MonthlyBudget in USD
	MonthlyBudget float64 `json:"monthlyBudget"`

	// AlertThreshold sends notification at this percentage (0-100)
	AlertThreshold float64 `json:"alertThreshold,omitempty"`

	// HardLimit stops new jobs when budget is exceeded
	HardLimit bool `json:"hardLimit,omitempty"`
}

// FabricQuotaStatus defines the observed state of FabricQuota
type FabricQuotaStatus struct {
	// Phase is the current phase (Active, BudgetExceeded, Suspended)
	Phase string `json:"phase,omitempty"`

	// CurrentUsage tracks current resource consumption
	CurrentUsage QuotaUsage `json:"currentUsage,omitempty"`

	// BudgetStatus tracks spending
	BudgetStatus *BudgetStatus `json:"budgetStatus,omitempty"`

	// Conditions represent the latest available observations
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// LastUpdated is the timestamp of last status update
	LastUpdated metav1.Time `json:"lastUpdated,omitempty"`
}

// QuotaUsage tracks current resource consumption
type QuotaUsage struct {
	// AllocatedGPUs is the current number of GPUs in use
	AllocatedGPUs int `json:"allocatedGPUs"`

	// RunningJobs is the current number of running jobs
	RunningJobs int `json:"runningJobs"`

	// QueuedJobs is the current number of queued jobs
	QueuedJobs int `json:"queuedJobs"`

	// GPUHours consumed this month
	GPUHours float64 `json:"gpuHours"`
}

// BudgetStatus tracks spending
type BudgetStatus struct {
	// SpentThisMonth in USD
	SpentThisMonth float64 `json:"spentThisMonth"`

	// RemainingBudget in USD
	RemainingBudget float64 `json:"remainingBudget"`

	// PercentUsed of monthly budget (0-100)
	PercentUsed float64 `json:"percentUsed"`

	// ProjectedSpend for the month based on current usage
	ProjectedSpend float64 `json:"projectedSpend"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster

// FabricQuota is the Schema for the fabricquotas API
type FabricQuota struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FabricQuotaSpec   `json:"spec,omitempty"`
	Status FabricQuotaStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// FabricQuotaList contains a list of FabricQuota
type FabricQuotaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FabricQuota `json:"items"`
}

// FabricAIJobSpec is a minimal struct for quota tracking and cost prediction.
// Go JSON unmarshaling ignores unknown fields by default, so the full
// FabricAIJob spec from the API server deserializes correctly into this type.
type FabricAIJobSpec struct {
	GPUs    int32  `json:"gpus"`
	GpuType string `json:"gpuType,omitempty"`

	// Additional fields used by the cost predictor
	Model          string                      `json:"model,omitempty"`
	StorageRequest string                      `json:"storageRequest,omitempty"`
	Network        string                      `json:"network,omitempty"`
	Priority       int32                       `json:"priority,omitempty"`
	Distributed    *FabricAIJobDistributedSpec  `json:"distributed,omitempty"`
}

// FabricAIJobDistributedSpec is a minimal distributed config used by the predictor
type FabricAIJobDistributedSpec struct {
	Enabled    bool   `json:"enabled,omitempty"`
	Framework  string `json:"framework,omitempty"`
	Nodes      int32  `json:"nodes,omitempty"`
	GpusPerNode int32 `json:"gpusPerNode,omitempty"`
	Backend    string `json:"backend,omitempty"`
}

// FabricAIJobStatus is a minimal struct for quota tracking and cost prediction.
// Go JSON unmarshaling ignores unknown fields by default.
type FabricAIJobStatus struct {
	Phase          string       `json:"phase,omitempty"`
	StartTime      *metav1.Time `json:"startTime,omitempty"`
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`
	Conditions     []metav1.Condition `json:"conditions,omitempty"`
	NodesAllocated []string     `json:"nodesAllocated,omitempty"`
	Retries        int32        `json:"retries,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:scope=Namespaced

// FabricAIJob is a minimal duplicate of the AI operator's FabricAIJob type,
// used by the quota operator to track AI job resources. It intentionally omits
// fields not needed for quota enforcement. Go JSON unmarshaling ignores
// unknown fields by default, so the full resource deserializes correctly.
type FabricAIJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FabricAIJobSpec   `json:"spec,omitempty"`
	Status FabricAIJobStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// FabricAIJobList contains a list of FabricAIJob
type FabricAIJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FabricAIJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FabricQuota{}, &FabricQuotaList{}, &FabricAIJob{}, &FabricAIJobList{})
}
