package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FabricGpuNodeSpec defines the desired state of FabricGpuNode
type FabricGpuNodeSpec struct {
	// NodeName is the Kubernetes node name
	NodeName string `json:"nodeName"`

	// GpuType is the GPU model (H100, A100, L40, V100, etc.)
	GpuType string `json:"gpuType"`

	// GpuCount is the number of GPUs on this node
	GpuCount int `json:"gpuCount"`

	// RDMA support enabled
	RDMA bool `json:"rdma,omitempty"`

	// SR-IOV support enabled
	SRIOV bool `json:"sriov,omitempty"`

	// Interconnect is the GPU interconnect type
	Interconnect string `json:"interconnect,omitempty"`

	// Bandwidth is the network bandwidth (e.g., 200Gbps, 400Gbps)
	Bandwidth string `json:"bandwidth,omitempty"`

	// MemoryGB is the GPU memory per GPU in GB
	MemoryGB int `json:"memoryGB,omitempty"`

	// ComputeCapability is the CUDA compute capability
	ComputeCapability string `json:"computeCapability,omitempty"`

	// Drivers configuration
	Drivers *GpuDriversConfig `json:"drivers,omitempty"`

	// HealthCheck configuration
	HealthCheck *GpuHealthCheckConfig `json:"healthCheck,omitempty"`

	// Labels are custom labels to apply to the node
	Labels map[string]string `json:"labels,omitempty"`
}

// GpuDriversConfig defines driver configuration
type GpuDriversConfig struct {
	// Nvidia driver version
	Nvidia string `json:"nvidia,omitempty"`

	// CUDA version
	CUDA string `json:"cuda,omitempty"`
}

// GpuHealthCheckConfig defines health check configuration
type GpuHealthCheckConfig struct {
	// Enabled enables health checking
	Enabled bool `json:"enabled,omitempty"`

	// IntervalSeconds is the health check interval
	IntervalSeconds int `json:"intervalSeconds,omitempty"`
}

// FabricGpuNodeStatus defines the observed state of FabricGpuNode
type FabricGpuNodeStatus struct {
	// Phase is the current phase (Initializing, Ready, Degraded, Failed)
	Phase string `json:"phase,omitempty"`

	// Conditions represent the latest available observations
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// GpuStatus is the status of each GPU
	GpuStatus []GpuDeviceStatus `json:"gpuStatus,omitempty"`

	// DriverVersion is the installed NVIDIA driver version
	DriverVersion string `json:"driverVersion,omitempty"`

	// CudaVersion is the installed CUDA version
	CudaVersion string `json:"cudaVersion,omitempty"`

	// LastHealthCheck is the timestamp of the last health check
	LastHealthCheck *metav1.Time `json:"lastHealthCheck,omitempty"`
}

// GpuDeviceStatus represents the status of a single GPU
type GpuDeviceStatus struct {
	// Index is the GPU index
	Index int `json:"index"`

	// UUID is the GPU UUID
	UUID string `json:"uuid,omitempty"`

	// Health status
	Health string `json:"health"`

	// Temperature in Celsius
	Temperature int `json:"temperature,omitempty"`

	// PowerUsage in Watts
	PowerUsage int `json:"powerUsage,omitempty"`

	// MemoryUsed in MB
	MemoryUsed int `json:"memoryUsed,omitempty"`

	// MemoryTotal in MB
	MemoryTotal int `json:"memoryTotal,omitempty"`

	// Utilization percentage (0-100)
	Utilization int `json:"utilization,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster
//+kubebuilder:printcolumn:name="Node",type=string,JSONPath=`.spec.nodeName`
//+kubebuilder:printcolumn:name="GPU-Type",type=string,JSONPath=`.spec.gpuType`
//+kubebuilder:printcolumn:name="GPU-Count",type=integer,JSONPath=`.spec.gpuCount`
//+kubebuilder:printcolumn:name="RDMA",type=boolean,JSONPath=`.spec.rdma`
//+kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// FabricGpuNode is the Schema for the fabricgpunodes API
type FabricGpuNode struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FabricGpuNodeSpec   `json:"spec,omitempty"`
	Status FabricGpuNodeStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// FabricGpuNodeList contains a list of FabricGpuNode
type FabricGpuNodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FabricGpuNode `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FabricGpuNode{}, &FabricGpuNodeList{})
}
