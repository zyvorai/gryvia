package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// InferenceBackend defines the supported inference serving backends
type InferenceBackend string

const (
	BackendTriton      InferenceBackend = "triton"
	BackendVLLM        InferenceBackend = "vllm"
	BackendTensorRTLLM InferenceBackend = "tensorrt-llm"
	BackendTorchServe  InferenceBackend = "torchserve"
)

// GryviaInferenceServiceSpec defines the desired state of GryviaInferenceService
type GryviaInferenceServiceSpec struct {
	// ModelRef is the name of the GryviaModelRegistry resource to serve
	ModelRef string `json:"modelRef"`

	// Backend is the inference backend (triton, vllm, tensorrt-llm, torchserve)
	Backend InferenceBackend `json:"backend"`

	// Replicas is the desired number of serving replicas
	Replicas int32 `json:"replicas,omitempty"`

	// GPUCount is the number of GPUs per replica
	GPUCount int32 `json:"gpuCount,omitempty"`

	// GPUType is the preferred GPU type for inference
	GPUType string `json:"gpuType,omitempty"`

	// Image overrides the default container image for the backend
	Image string `json:"image,omitempty"`

	// Args are additional arguments passed to the serving container
	Args []string `json:"args,omitempty"`

	// Autoscaling configures horizontal pod autoscaling
	Autoscaling *AutoscalingConfig `json:"autoscaling,omitempty"`

	// Canary configures canary deployment and traffic splitting
	Canary *CanaryConfig `json:"canary,omitempty"`

	// HealthCheck configures health checking and automatic rollback
	HealthCheck *HealthCheckConfig `json:"healthCheck,omitempty"`

	// ServicePort is the port the inference service listens on (default: 8080)
	ServicePort int32 `json:"servicePort,omitempty"`
}

// AutoscalingConfig defines HPA configuration for inference services
type AutoscalingConfig struct {
	// Enabled turns on horizontal pod autoscaling
	Enabled bool `json:"enabled"`

	// MinReplicas is the minimum number of replicas
	MinReplicas int32 `json:"minReplicas,omitempty"`

	// MaxReplicas is the maximum number of replicas
	MaxReplicas int32 `json:"maxReplicas"`

	// TargetGPUUtilization is the target GPU utilization percentage for scaling
	TargetGPUUtilization int32 `json:"targetGPUUtilization,omitempty"`

	// TargetRequestsPerSecond is the target request rate per replica for scaling
	TargetRequestsPerSecond int32 `json:"targetRequestsPerSecond,omitempty"`
}

// CanaryConfig defines canary deployment settings
type CanaryConfig struct {
	// Enabled turns on canary deployment
	Enabled bool `json:"enabled"`

	// Weight is the percentage of traffic routed to the canary (0-100)
	Weight int32 `json:"weight"`

	// ModelVersion is the model version (GryviaModelRegistry name) to deploy as canary
	ModelVersion string `json:"modelVersion"`

	// AutoPromote automatically promotes canary to primary if health checks pass
	AutoPromote bool `json:"autoPromote,omitempty"`

	// PromoteAfterSeconds is the duration in seconds to wait before auto-promoting
	PromoteAfterSeconds int64 `json:"promoteAfterSeconds,omitempty"`
}

// HealthCheckConfig defines health checking for inference services
type HealthCheckConfig struct {
	// Path is the HTTP health check path (default: /health)
	Path string `json:"path,omitempty"`

	// IntervalSeconds is the interval between health checks
	IntervalSeconds int32 `json:"intervalSeconds,omitempty"`

	// FailureThreshold is the number of consecutive failures before marking unhealthy
	FailureThreshold int32 `json:"failureThreshold,omitempty"`

	// AutoRollback enables automatic rollback to the previous version on failure
	AutoRollback bool `json:"autoRollback,omitempty"`
}

// GryviaInferenceServiceStatus defines the observed state of GryviaInferenceService
type GryviaInferenceServiceStatus struct {
	// Phase is the current phase (Pending, Deploying, Ready, Failed, RollingBack)
	Phase string `json:"phase,omitempty"`

	// Conditions represent the latest available observations
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Endpoint is the URL at which the inference service can be reached
	Endpoint string `json:"endpoint,omitempty"`

	// ReadyReplicas is the number of ready serving replicas
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// CanaryStatus holds the status of the canary deployment
	CanaryStatus *CanaryStatus `json:"canaryStatus,omitempty"`

	// DeploymentName is the name of the Kubernetes Deployment
	DeploymentName string `json:"deploymentName,omitempty"`

	// ServiceName is the name of the Kubernetes Service
	ServiceName string `json:"serviceName,omitempty"`

	// LastHealthCheck is the timestamp of the last health check
	LastHealthCheck *metav1.Time `json:"lastHealthCheck,omitempty"`

	// HealthStatus is the current health (Healthy, Unhealthy, Unknown)
	HealthStatus string `json:"healthStatus,omitempty"`

	// ConsecutiveFailures is the number of consecutive health check failures
	ConsecutiveFailures int32 `json:"consecutiveFailures,omitempty"`

	// Message provides additional status information
	Message string `json:"message,omitempty"`
}

// CanaryStatus holds the observed state of a canary deployment
type CanaryStatus struct {
	// Active indicates whether a canary is currently deployed
	Active bool `json:"active"`

	// Weight is the current traffic weight for the canary
	Weight int32 `json:"weight,omitempty"`

	// DeploymentName is the name of the canary Deployment
	DeploymentName string `json:"deploymentName,omitempty"`

	// ReadyReplicas is the number of ready canary replicas
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// Health is the canary health status
	Health string `json:"health,omitempty"`

	// StartedAt is when the canary deployment started
	StartedAt *metav1.Time `json:"startedAt,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Namespaced
//+kubebuilder:printcolumn:name="ModelRef",type=string,JSONPath=`.spec.modelRef`
//+kubebuilder:printcolumn:name="Backend",type=string,JSONPath=`.spec.backend`
//+kubebuilder:printcolumn:name="Replicas",type=integer,JSONPath=`.status.readyReplicas`
//+kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// GryviaInferenceService is the Schema for the gryviainferenceservices API
type GryviaInferenceService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GryviaInferenceServiceSpec   `json:"spec,omitempty"`
	Status GryviaInferenceServiceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// GryviaInferenceServiceList contains a list of GryviaInferenceService
type GryviaInferenceServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GryviaInferenceService `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GryviaInferenceService{}, &GryviaInferenceServiceList{})
}
