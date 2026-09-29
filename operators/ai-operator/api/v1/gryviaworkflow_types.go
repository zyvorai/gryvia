package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StepType defines the kind of step in a workflow
type StepType string

const (
	StepTypeJob     StepType = "job"
	StepTypeScript  StepType = "script"
	StepTypeWebhook StepType = "webhook"
)

// FabricWorkflowSpec defines the desired state of FabricWorkflow
type FabricWorkflowSpec struct {
	// Steps is the list of workflow steps
	Steps []WorkflowStep `json:"steps"`

	// Parameters are global parameters passed to all steps
	Parameters map[string]string `json:"parameters,omitempty"`
}

// WorkflowStep defines a single step in the workflow DAG
type WorkflowStep struct {
	// Name is the unique name of this step within the workflow
	Name string `json:"name"`

	// Type of step: job, script, or webhook
	Type StepType `json:"type,omitempty"`

	// DependsOn is a list of step names that must complete before this step runs
	DependsOn []string `json:"dependsOn,omitempty"`

	// Condition is a CEL expression that must evaluate to true for this step to run.
	// The expression has access to previous step outputs and statuses.
	// Example: "steps.train.status == 'Succeeded' && steps.train.metrics.accuracy > 0.95"
	Condition string `json:"condition,omitempty"`

	// JobTemplate is the FabricAIJob spec to run for job-type steps
	JobTemplate *FabricAIJobSpec `json:"jobTemplate,omitempty"`

	// Script is the shell script to execute for script-type steps
	Script *ScriptStep `json:"script,omitempty"`

	// Webhook is the HTTP request to make for webhook-type steps
	Webhook *WebhookStep `json:"webhook,omitempty"`

	// Retries is the number of times to retry this step on failure
	Retries int32 `json:"retries,omitempty"`

	// RetryBackoffSeconds is the initial backoff duration between retries in seconds
	RetryBackoffSeconds int32 `json:"retryBackoffSeconds,omitempty"`

	// TimeoutSeconds is the maximum duration for this step in seconds
	TimeoutSeconds int64 `json:"timeoutSeconds,omitempty"`
}

// ScriptStep defines an inline script to run
type ScriptStep struct {
	// Image is the container image to run the script in
	Image string `json:"image"`

	// Command is the script content to execute
	Command []string `json:"command"`

	// Args are arguments passed to the command
	Args []string `json:"args,omitempty"`
}

// WebhookStep defines an HTTP webhook call
type WebhookStep struct {
	// URL is the webhook endpoint
	URL string `json:"url"`

	// Method is the HTTP method (GET, POST, PUT)
	Method string `json:"method,omitempty"`

	// Headers are HTTP headers to include
	Headers map[string]string `json:"headers,omitempty"`

	// Body is the request body (JSON)
	Body string `json:"body,omitempty"`

	// SuccessCondition is a JSONPath expression on the response to determine success
	SuccessCondition string `json:"successCondition,omitempty"`
}

// StepPhase represents the current state of a workflow step
type StepPhase string

const (
	StepPhasePending   StepPhase = "Pending"
	StepPhaseRunning   StepPhase = "Running"
	StepPhaseSucceeded StepPhase = "Succeeded"
	StepPhaseFailed    StepPhase = "Failed"
	StepPhaseSkipped   StepPhase = "Skipped"
)

// StepStatus holds the status of a single workflow step
type StepStatus struct {
	// Name is the step name
	Name string `json:"name"`

	// Phase is the current phase of this step
	Phase StepPhase `json:"phase"`

	// JobName is the name of the FabricAIJob created for this step (if applicable)
	JobName string `json:"jobName,omitempty"`

	// StartTime is when the step started
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is when the step finished
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// RetriesAttempted is the number of retries attempted so far
	RetriesAttempted int32 `json:"retriesAttempted,omitempty"`

	// Message provides additional status information
	Message string `json:"message,omitempty"`
}

// FabricWorkflowStatus defines the observed state of FabricWorkflow
type FabricWorkflowStatus struct {
	// Phase is the overall workflow phase (Pending, Running, Succeeded, Failed)
	Phase string `json:"phase,omitempty"`

	// Conditions represent the latest available observations
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// StepStatuses is the status of each step in the workflow
	StepStatuses []StepStatus `json:"stepStatuses,omitempty"`

	// StartTime is when the workflow started
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is when the workflow finished
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Message provides additional information about the current phase
	Message string `json:"message,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Namespaced
//+kubebuilder:printcolumn:name="Steps",type=integer,JSONPath=`.status.stepsTotal`
//+kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// FabricWorkflow is the Schema for the fabricworkflows API
type FabricWorkflow struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FabricWorkflowSpec   `json:"spec,omitempty"`
	Status FabricWorkflowStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// FabricWorkflowList contains a list of FabricWorkflow
type FabricWorkflowList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FabricWorkflow `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FabricWorkflow{}, &FabricWorkflowList{})
}
