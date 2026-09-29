package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GryviaJobHookSpec defines the desired state of GryviaJobHook
type GryviaJobHookSpec struct {
	// Trigger is the lifecycle phase that triggers this hook
	Trigger string `json:"trigger"`

	// Selector restricts which jobs this hook applies to
	Selector *HookSelector `json:"selector,omitempty"`

	// Action defines what to execute when triggered
	Action HookAction `json:"action"`

	// FailurePolicy defines behavior on hook failure (ignore, fail-job, retry)
	FailurePolicy string `json:"failurePolicy,omitempty"`

	// Retry defines retry configuration for the hook
	Retry *HookRetry `json:"retry,omitempty"`

	// Condition is an expression to evaluate before executing
	Condition string `json:"condition,omitempty"`
}

// HookSelector defines which jobs a hook applies to
type HookSelector struct {
	// MatchLabels restricts by job labels
	MatchLabels map[string]string `json:"matchLabels,omitempty"`
}

// HookAction defines the hook action to execute
type HookAction struct {
	// Type is the action type (exec, webhook, k8s-job, script, notification)
	Type string `json:"type"`

	// Exec defines an exec action
	Exec *ExecAction `json:"exec,omitempty"`

	// Webhook defines a webhook action
	Webhook *WebhookAction `json:"webhook,omitempty"`

	// K8sJob defines a Kubernetes Job action
	K8sJob *K8sJobAction `json:"k8sJob,omitempty"`

	// Notification defines a notification action
	Notification *NotificationAction `json:"notification,omitempty"`
}

// ExecAction defines a command execution
type ExecAction struct {
	// Command to execute
	Command []string `json:"command"`

	// Timeout for execution
	Timeout string `json:"timeout,omitempty"`
}

// WebhookAction defines an HTTP webhook call
type WebhookAction struct {
	// URL to call
	URL string `json:"url"`

	// Method is the HTTP method (GET, POST, PUT)
	Method string `json:"method,omitempty"`

	// Headers are additional HTTP headers
	Headers map[string]string `json:"headers,omitempty"`

	// Body is the request body template
	Body string `json:"body,omitempty"`

	// Timeout for the webhook call
	Timeout string `json:"timeout,omitempty"`
}

// K8sJobAction defines a Kubernetes Job to run
type K8sJobAction struct {
	// Image is the container image
	Image string `json:"image"`

	// Command to execute
	Command []string `json:"command,omitempty"`
}

// NotificationAction defines a notification to send
type NotificationAction struct {
	// Channels to notify (email, slack, pagerduty, webhook)
	Channels []string `json:"channels,omitempty"`

	// Template is the notification message template
	Template string `json:"template,omitempty"`

	// Recipients are the notification recipients
	Recipients []string `json:"recipients,omitempty"`
}

// HookRetry defines retry configuration for hooks
type HookRetry struct {
	// Attempts is the max number of retries
	Attempts int32 `json:"attempts,omitempty"`

	// Backoff is the delay between retries
	Backoff string `json:"backoff,omitempty"`
}

// GryviaJobHookStatus defines the observed state of GryviaJobHook
type GryviaJobHookStatus struct {
	// LastExecutionTime is the time of the last hook execution
	LastExecutionTime *metav1.Time `json:"lastExecutionTime,omitempty"`

	// ExecutionCount is the total number of executions
	ExecutionCount int32 `json:"executionCount,omitempty"`

	// LastStatus is the result of the last execution (success, failed, skipped)
	LastStatus string `json:"lastStatus,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster
//+kubebuilder:printcolumn:name="Trigger",type=string,JSONPath=`.spec.trigger`
//+kubebuilder:printcolumn:name="Action",type=string,JSONPath=`.spec.action.type`
//+kubebuilder:printcolumn:name="Last-Execution",type=string,JSONPath=`.status.lastExecutionTime`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// GryviaJobHook is the Schema for the gryviajobhooks API
type GryviaJobHook struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GryviaJobHookSpec   `json:"spec,omitempty"`
	Status GryviaJobHookStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// GryviaJobHookList contains a list of GryviaJobHook
type GryviaJobHookList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GryviaJobHook `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GryviaJobHook{}, &GryviaJobHookList{})
}
