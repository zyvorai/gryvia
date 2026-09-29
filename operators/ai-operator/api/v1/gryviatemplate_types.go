package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FabricTemplateSpec defines the desired state of FabricTemplate
type FabricTemplateSpec struct {
	// Description is a human-readable description of the template
	Description string `json:"description,omitempty"`

	// Category is the template category (training, inference, development, benchmark)
	Category string `json:"category"`

	// Tags are searchable labels for the template
	Tags []string `json:"tags,omitempty"`

	// Defaults defines the default job configuration
	Defaults TemplateDefaults `json:"defaults"`

	// Parameters defines the customizable parameters
	Parameters []TemplateParameter `json:"parameters,omitempty"`
}

// TemplateDefaults defines default job configuration values
type TemplateDefaults struct {
	// Framework is the ML framework
	Framework string `json:"framework,omitempty"`

	// Resources defines default resource requirements
	Resources *TemplateResources `json:"resources,omitempty"`

	// Image is the container image
	Image string `json:"image,omitempty"`

	// Command is the container command
	Command []string `json:"command,omitempty"`

	// Env defines default environment variables
	Env []TemplateEnvVar `json:"env,omitempty"`
}

// TemplateResources defines template resource defaults
type TemplateResources struct {
	// GpuType is the GPU model
	GpuType string `json:"gpuType,omitempty"`

	// GpuCount is the number of GPUs
	GpuCount int32 `json:"gpuCount,omitempty"`

	// Memory is the memory request
	Memory string `json:"memory,omitempty"`

	// CPU is the CPU request
	CPU int32 `json:"cpu,omitempty"`
}

// TemplateEnvVar defines an environment variable
type TemplateEnvVar struct {
	// Name of the environment variable
	Name string `json:"name"`

	// Value of the environment variable
	Value string `json:"value"`
}

// TemplateParameter defines a customizable parameter
type TemplateParameter struct {
	// Name of the parameter
	Name string `json:"name"`

	// Description of the parameter
	Description string `json:"description,omitempty"`

	// Type is the parameter type (string, integer, number, boolean, array)
	Type string `json:"type,omitempty"`

	// Default is the default value
	Default string `json:"default,omitempty"`

	// Required indicates whether the parameter must be provided
	Required bool `json:"required,omitempty"`

	// Validation defines validation rules
	Validation *ParameterValidation `json:"validation,omitempty"`
}

// ParameterValidation defines parameter validation rules
type ParameterValidation struct {
	// Min is the minimum allowed value
	Min *float64 `json:"min,omitempty"`

	// Max is the maximum allowed value
	Max *float64 `json:"max,omitempty"`

	// Pattern is a regex pattern for string validation
	Pattern string `json:"pattern,omitempty"`

	// Enum lists allowed values
	Enum []string `json:"enum,omitempty"`
}

// FabricTemplateStatus defines the observed state of FabricTemplate
type FabricTemplateStatus struct {
	// InstantiatedJobs tracks the number of jobs created from this template
	InstantiatedJobs int32 `json:"instantiatedJobs,omitempty"`

	// LastInstantiatedTime is the time of the last job creation
	LastInstantiatedTime *metav1.Time `json:"lastInstantiatedTime,omitempty"`

	// Conditions represent the latest available observations
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster
//+kubebuilder:printcolumn:name="Category",type=string,JSONPath=`.spec.category`
//+kubebuilder:printcolumn:name="Description",type=string,JSONPath=`.spec.description`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// FabricTemplate is the Schema for the fabrictemplates API
type FabricTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FabricTemplateSpec   `json:"spec,omitempty"`
	Status FabricTemplateStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// FabricTemplateList contains a list of FabricTemplate
type FabricTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FabricTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FabricTemplate{}, &FabricTemplateList{})
}
