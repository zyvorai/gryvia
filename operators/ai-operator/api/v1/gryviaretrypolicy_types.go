package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GryviaRetryPolicySpec defines the desired state of GryviaRetryPolicy
type GryviaRetryPolicySpec struct {
	// MaxRetries is the maximum number of retries
	MaxRetries int32 `json:"maxRetries,omitempty"`

	// Backoff defines the backoff strategy
	Backoff *BackoffStrategy `json:"backoff,omitempty"`

	// RetryOn defines conditions that trigger a retry
	RetryOn *RetryConditions `json:"retryOn,omitempty"`

	// NoRetryOn defines conditions that prevent a retry
	NoRetryOn *NoRetryConditions `json:"noRetryOn,omitempty"`

	// ResourceAdjustment defines resource changes on retry
	ResourceAdjustment *ResourceAdjustment `json:"resourceAdjustment,omitempty"`

	// Budget defines cost and time limits for retries
	Budget *RetryBudget `json:"budget,omitempty"`

	// CircuitBreaker defines circuit breaker configuration
	CircuitBreaker *CircuitBreaker `json:"circuitBreaker,omitempty"`
}

// BackoffStrategy defines retry backoff behavior
type BackoffStrategy struct {
	// Type is the backoff type (fixed, exponential, fibonacci, random)
	Type string `json:"type,omitempty"`

	// InitialDelay is the initial delay between retries
	InitialDelay string `json:"initialDelay,omitempty"`

	// MaxDelay is the maximum delay between retries
	MaxDelay string `json:"maxDelay,omitempty"`

	// Multiplier is the backoff multiplier for exponential backoff
	Multiplier float64 `json:"multiplier,omitempty"`
}

// RetryConditions defines when to retry
type RetryConditions struct {
	// ExitCodes lists exit codes that trigger a retry
	ExitCodes []int32 `json:"exitCodes,omitempty"`

	// Errors lists error message substrings that trigger a retry
	Errors []string `json:"errors,omitempty"`

	// Conditions lists failure types that trigger a retry
	Conditions []RetryConditionType `json:"conditions,omitempty"`
}

// RetryConditionType defines a typed retry condition
type RetryConditionType struct {
	// Type is the condition type (OutOfMemory, NodeFailure, NetworkError, etc.)
	Type string `json:"type"`

	// Enabled controls whether this condition triggers retries
	Enabled bool `json:"enabled"`
}

// NoRetryConditions defines when not to retry
type NoRetryConditions struct {
	// ExitCodes lists exit codes that prevent retry
	ExitCodes []int32 `json:"exitCodes,omitempty"`

	// Errors lists error message substrings that prevent retry
	Errors []string `json:"errors,omitempty"`
}

// ResourceAdjustment defines resource changes on retry
type ResourceAdjustment struct {
	// Enabled enables resource adjustment
	Enabled bool `json:"enabled,omitempty"`

	// OnOutOfMemory defines adjustments for OOM failures
	OnOutOfMemory *OOMResourceAdjustment `json:"onOutOfMemory,omitempty"`
}

// OOMResourceAdjustment defines adjustments for out-of-memory failures
type OOMResourceAdjustment struct {
	// IncreaseMemory is the percentage to increase memory
	IncreaseMemory string `json:"increaseMemory,omitempty"`

	// IncreaseGPUMemory switches to a larger GPU type
	IncreaseGPUMemory bool `json:"increaseGPUMemory,omitempty"`
}

// RetryBudget defines cost and time limits for retries
type RetryBudget struct {
	// MaxCostUSD is the maximum cost for all retries
	MaxCostUSD float64 `json:"maxCostUSD,omitempty"`

	// MaxTotalTime is the maximum total time for all attempts
	MaxTotalTime string `json:"maxTotalTime,omitempty"`
}

// CircuitBreaker defines circuit breaker behavior
type CircuitBreaker struct {
	// Enabled enables circuit breaker
	Enabled bool `json:"enabled,omitempty"`

	// FailureThreshold is the number of consecutive failures before opening circuit
	FailureThreshold int32 `json:"failureThreshold,omitempty"`

	// ResetTimeout is the time to wait before closing the circuit
	ResetTimeout string `json:"resetTimeout,omitempty"`
}

// GryviaRetryPolicyStatus defines the observed state of GryviaRetryPolicy
type GryviaRetryPolicyStatus struct {
	// Conditions report runtime capability and remediation state.
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Attempts is the total number of retry attempts
	Attempts int32 `json:"attempts,omitempty"`

	// SuccessRate is the retry success rate
	SuccessRate float64 `json:"successRate,omitempty"`

	// LastAttemptTime is the time of the last retry attempt
	LastAttemptTime *metav1.Time `json:"lastAttemptTime,omitempty"`

	// NextRetryTime is the scheduled time for the next retry
	NextRetryTime *metav1.Time `json:"nextRetryTime,omitempty"`

	// State is the current state (active, circuit-open, budget-exceeded)
	State string `json:"state,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster
//+kubebuilder:printcolumn:name="Max-Retries",type=integer,JSONPath=`.spec.maxRetries`
//+kubebuilder:printcolumn:name="Backoff",type=string,JSONPath=`.spec.backoff.type`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// GryviaRetryPolicy is the Schema for the gryviaretrypolicies API
type GryviaRetryPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GryviaRetryPolicySpec   `json:"spec,omitempty"`
	Status GryviaRetryPolicyStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// GryviaRetryPolicyList contains a list of GryviaRetryPolicy
type GryviaRetryPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GryviaRetryPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GryviaRetryPolicy{}, &GryviaRetryPolicyList{})
}
