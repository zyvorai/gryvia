package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ModelStage represents the promotion stage of a model
type ModelStage string

const (
	ModelStageDev        ModelStage = "dev"
	ModelStageStaging    ModelStage = "staging"
	ModelStageProduction ModelStage = "production"
	ModelStageArchived   ModelStage = "archived"
)

// FabricModelRegistrySpec defines the desired state of FabricModelRegistry
type FabricModelRegistrySpec struct {
	// ModelName is the name of the model
	ModelName string `json:"modelName"`

	// Version is the semantic version of this model entry
	Version string `json:"version"`

	// Source identifies where this model was produced
	Source ModelSource `json:"source,omitempty"`

	// Artifacts defines where the model files are stored
	Artifacts ModelArtifacts `json:"artifacts"`

	// Stage is the current promotion stage (dev, staging, production, archived)
	Stage ModelStage `json:"stage,omitempty"`

	// Description is a human-readable description of this model version
	Description string `json:"description,omitempty"`

	// Metadata holds arbitrary key-value metadata (e.g., framework, metrics)
	Metadata map[string]string `json:"metadata,omitempty"`

	// AutoServe enables automatic inference deployment when the model reaches production stage
	AutoServe bool `json:"autoServe,omitempty"`

	// ServingConfig defines the configuration for auto-serving when stage is production
	ServingConfig *ServingConfig `json:"servingConfig,omitempty"`
}

// ModelSource identifies the training job or pipeline that produced the model
type ModelSource struct {
	// JobRef is the name of the FabricAIJob that produced this model
	JobRef string `json:"jobRef,omitempty"`

	// TunerRef is the name of the FabricAutoTuner that produced this model
	TunerRef string `json:"tunerRef,omitempty"`

	// WorkflowRef is the name of the FabricWorkflow that produced this model
	WorkflowRef string `json:"workflowRef,omitempty"`
}

// ModelArtifacts defines the storage location of model files
type ModelArtifacts struct {
	// S3Path is the S3 URI (e.g., s3://bucket/path/to/model)
	S3Path string `json:"s3Path,omitempty"`

	// PVCName is the PersistentVolumeClaim storing the model
	PVCName string `json:"pvcName,omitempty"`

	// SubPath is the path within the PVC or S3 bucket
	SubPath string `json:"subPath,omitempty"`

	// Format is the model format (pytorch, onnx, tensorrt, safetensors)
	Format string `json:"format,omitempty"`

	// SizeBytes is the total size of the model artifacts in bytes
	SizeBytes int64 `json:"sizeBytes,omitempty"`
}

// ServingConfig defines how a model should be served when auto-deployed
type ServingConfig struct {
	// Backend is the inference backend to use (triton, vllm, tensorrt-llm, torchserve)
	Backend string `json:"backend,omitempty"`

	// Replicas is the number of serving replicas
	Replicas int32 `json:"replicas,omitempty"`

	// GPUCount is the number of GPUs per replica
	GPUCount int32 `json:"gpuCount,omitempty"`

	// GPUType is the preferred GPU type for serving
	GPUType string `json:"gpuType,omitempty"`
}

// FabricModelRegistryStatus defines the observed state of FabricModelRegistry
type FabricModelRegistryStatus struct {
	// Phase is the current phase (Registered, Deploying, Serving, Failed)
	Phase string `json:"phase,omitempty"`

	// Conditions represent the latest available observations
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ServingEndpoint is the URL of the deployed inference service
	ServingEndpoint string `json:"servingEndpoint,omitempty"`

	// DeployedAt is when the model was deployed for serving
	DeployedAt *metav1.Time `json:"deployedAt,omitempty"`

	// Health is the health status of the serving deployment (Healthy, Unhealthy, Unknown)
	Health string `json:"health,omitempty"`

	// InferenceServiceName is the name of the FabricInferenceService created for this model
	InferenceServiceName string `json:"inferenceServiceName,omitempty"`

	// PreviousVersion is the version that was previously in production (for rollback)
	PreviousVersion string `json:"previousVersion,omitempty"`

	// RegisteredAt is when the model was first registered
	RegisteredAt *metav1.Time `json:"registeredAt,omitempty"`

	// Message provides additional status information
	Message string `json:"message,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Namespaced
//+kubebuilder:printcolumn:name="Model",type=string,JSONPath=`.spec.modelName`
//+kubebuilder:printcolumn:name="Version",type=string,JSONPath=`.spec.version`
//+kubebuilder:printcolumn:name="Stage",type=string,JSONPath=`.spec.stage`
//+kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// FabricModelRegistry is the Schema for the fabricmodelregistries API
type FabricModelRegistry struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FabricModelRegistrySpec   `json:"spec,omitempty"`
	Status FabricModelRegistryStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// FabricModelRegistryList contains a list of FabricModelRegistry
type FabricModelRegistryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FabricModelRegistry `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FabricModelRegistry{}, &FabricModelRegistryList{})
}
