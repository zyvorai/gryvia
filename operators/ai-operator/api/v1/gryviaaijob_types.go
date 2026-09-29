package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GryviaAIJobSpec defines the desired state of GryviaAIJob
type GryviaAIJobSpec struct {
	// Type of workload (training, inference, fine-tuning, evaluation)
	Type string `json:"type"`

	// Model name (llama-70b, gpt-4, stable-diffusion, etc.)
	Model string `json:"model,omitempty"`

	// Number of GPUs required
	GPUs int32 `json:"gpus"`

	// Preferred GPU type (H100, A100, L40, V100, T4, any)
	GpuType string `json:"gpuType,omitempty"`

	// Storage backend name
	Storage string `json:"storage,omitempty"`

	// StorageRequest is the PVC size to request (e.g., "100Gi")
	StorageRequest string `json:"storageRequest,omitempty"`

	// Network type (rdma, sriov, standard)
	Network string `json:"network,omitempty"`

	// Container image
	Image string `json:"image"`

	// Image pull policy
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// Container command
	Command []string `json:"command,omitempty"`

	// Container arguments
	Args []string `json:"args,omitempty"`

	// Environment variables
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Working directory
	WorkingDir string `json:"workingDir,omitempty"`

	// Distributed training configuration
	Distributed *DistributedConfig `json:"distributed,omitempty"`

	// Resource requirements
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Volumes
	Volumes []corev1.Volume `json:"volumes,omitempty"`

	// Volume mounts
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`

	// Job priority (0-100, higher = more important)
	Priority int32 `json:"priority,omitempty"`

	// Number of retries on failure
	RetryLimit int32 `json:"retryLimit,omitempty"`

	// Job timeout (e.g., 24h, 7d)
	Timeout string `json:"timeout,omitempty"`

	// Node selector
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Tolerations
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Affinity
	Affinity *corev1.Affinity `json:"affinity,omitempty"`
}

// DistributedConfig defines distributed training configuration
type DistributedConfig struct {
	// Enable distributed training
	Enabled bool `json:"enabled,omitempty"`

	// Framework (pytorch, tensorflow, horovod, deepspeed, megatron)
	Framework string `json:"framework,omitempty"`

	// Number of nodes
	Nodes int32 `json:"nodes,omitempty"`

	// GPUs per node
	GpusPerNode int32 `json:"gpusPerNode,omitempty"`

	// Backend (nccl, gloo, mpi)
	Backend string `json:"backend,omitempty"`
}

// GryviaAIJobStatus defines the observed state of GryviaAIJob
type GryviaAIJobStatus struct {
	// Current phase (Pending, Scheduling, Running, Succeeded, Failed, Unknown)
	Phase string `json:"phase,omitempty"`

	// Conditions represent the latest available observations
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Start time
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// Completion time
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Nodes allocated
	NodesAllocated []string `json:"nodesAllocated,omitempty"`

	// GPUs allocated
	GpusAllocated int32 `json:"gpusAllocated,omitempty"`

	// Number of retries
	Retries int32 `json:"retries,omitempty"`

	// Metrics
	Metrics *JobMetrics `json:"metrics,omitempty"`

	// Ready replicas
	ReplicasReady int32 `json:"replicasReady,omitempty"`

	// Human-readable message indicating details about the current phase
	Message string `json:"message,omitempty"`
}

// JobMetrics represents job performance metrics
type JobMetrics struct {
	// Average GPU utilization %
	GpuUtilization float64 `json:"gpuUtilization,omitempty"`

	// Training throughput
	Throughput string `json:"throughput,omitempty"`

	// Current loss value
	Loss float64 `json:"loss,omitempty"`

	// Current epoch
	Epoch int32 `json:"epoch,omitempty"`

	// Current step
	Step int32 `json:"step,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Namespaced
//+kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.type`
//+kubebuilder:printcolumn:name="GPUs",type=integer,JSONPath=`.spec.gpus`
//+kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// GryviaAIJob is the Schema for the gryviaaijobs API
type GryviaAIJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GryviaAIJobSpec   `json:"spec,omitempty"`
	Status GryviaAIJobStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// GryviaAIJobList contains a list of GryviaAIJob
type GryviaAIJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GryviaAIJob `json:"items"`
}

// StorageSize returns the storage request size, defaulting to 100Gi
func (s *GryviaAIJobSpec) StorageSize() string {
	if s.StorageRequest != "" {
		return s.StorageRequest
	}
	return "100Gi"
}

// EnvVarFromCoreV1 creates a corev1.EnvVar from a key-value pair.
func EnvVarFromCoreV1(name, value string) corev1.EnvVar {
	return corev1.EnvVar{Name: name, Value: value}
}

func init() {
	SchemeBuilder.Register(&GryviaAIJob{}, &GryviaAIJobList{})
}
