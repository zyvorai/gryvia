package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GryviaSLASpec defines the desired state of GryviaSLA
type GryviaSLASpec struct {
	// Tier is the SLA tier (platinum, gold, silver, bronze, best-effort)
	Tier string `json:"tier"`

	// Scope defines who this SLA applies to
	Scope SLAScope `json:"scope"`

	// Performance defines performance guarantees
	Performance *SLAPerformance `json:"performance,omitempty"`

	// Availability defines availability guarantees
	Availability *SLAAvailability `json:"availability,omitempty"`

	// Resources defines resource guarantees
	Resources *SLAResources `json:"resources,omitempty"`

	// FailureHandling defines failure handling policy
	FailureHandling *SLAFailureHandling `json:"failureHandling,omitempty"`

	// Support defines support level
	Support *SLASupport `json:"support,omitempty"`

	// Monitoring defines monitoring and reporting
	Monitoring *SLAMonitoring `json:"monitoring,omitempty"`
}

// SLAScope defines the scope of the SLA
type SLAScope struct {
	// Type is the scope type (team, user, job-type, namespace)
	Type string `json:"type,omitempty"`

	// Name of the scoped entity
	Name string `json:"name,omitempty"`
}

// SLAPerformance defines performance guarantees
type SLAPerformance struct {
	// MaxQueueTime is the max time a job can wait in queue
	MaxQueueTime string `json:"maxQueueTime,omitempty"`

	// GuaranteedStart guarantees the job starts within MaxQueueTime
	GuaranteedStart bool `json:"guaranteedStart,omitempty"`

	// TargetCompletionTime is the target completion time
	TargetCompletionTime string `json:"targetCompletionTime,omitempty"`

	// MinThroughput defines minimum throughput requirements
	MinThroughput *SLAThroughput `json:"minThroughput,omitempty"`
}

// SLAThroughput defines throughput requirements
type SLAThroughput struct {
	// Value is the throughput value
	Value float64 `json:"value,omitempty"`

	// Unit is the throughput unit (samples/sec, tokens/sec, etc.)
	Unit string `json:"unit,omitempty"`
}

// SLAAvailability defines availability guarantees
type SLAAvailability struct {
	// Uptime is the uptime percentage target (e.g., 99.9)
	Uptime float64 `json:"uptime,omitempty"`

	// MaxDowntimeMinutes is max allowed downtime per month
	MaxDowntimeMinutes float64 `json:"maxDowntimeMinutes,omitempty"`

	// MaintenanceWindows defines scheduled maintenance
	MaintenanceWindows []SLAMaintenanceWindow `json:"maintenanceWindows,omitempty"`
}

// SLAMaintenanceWindow defines a maintenance window
type SLAMaintenanceWindow struct {
	// DayOfWeek for the window
	DayOfWeek string `json:"dayOfWeek,omitempty"`

	// StartTime in HH:MM format
	StartTime string `json:"startTime,omitempty"`

	// Duration of the window
	Duration string `json:"duration,omitempty"`
}

// SLAResources defines resource guarantees
type SLAResources struct {
	// GuaranteedGPUTypes lists GPU types guaranteed available
	GuaranteedGPUTypes []string `json:"guaranteedGPUTypes,omitempty"`

	// ReservedCapacity defines reserved GPU capacity
	ReservedCapacity *SLAReservedCapacity `json:"reservedCapacity,omitempty"`

	// FallbackAllowed allows fallback to different GPU type
	FallbackAllowed bool `json:"fallbackAllowed,omitempty"`
}

// SLAReservedCapacity defines reserved capacity
type SLAReservedCapacity struct {
	// GPUCount is the number of reserved GPUs
	GPUCount int `json:"gpuCount,omitempty"`

	// Always reserves capacity at all times
	Always bool `json:"always,omitempty"`
}

// SLAFailureHandling defines failure handling
type SLAFailureHandling struct {
	// AutoRetry enables auto-retry on failure
	AutoRetry bool `json:"autoRetry,omitempty"`

	// MaxRetries is the max number of retries
	MaxRetries int `json:"maxRetries,omitempty"`

	// Compensation defines SLA breach compensation
	Compensation *SLACompensation `json:"compensation,omitempty"`
}

// SLACompensation defines compensation for SLA breaches
type SLACompensation struct {
	// Enabled enables compensation
	Enabled bool `json:"enabled,omitempty"`

	// Type of compensation (credit, refund, priority-boost)
	Type string `json:"type,omitempty"`

	// Percentage of compensation
	Percentage float64 `json:"percentage,omitempty"`
}

// SLASupport defines support level
type SLASupport struct {
	// ResponseTime is the max response time
	ResponseTime string `json:"responseTime,omitempty"`

	// Channels available (email, slack, phone, pagerduty)
	Channels []string `json:"channels,omitempty"`

	// BusinessHours indicates business hours only support
	BusinessHours bool `json:"businessHours,omitempty"`
}

// SLAMonitoring defines monitoring configuration
type SLAMonitoring struct {
	// MetricsRetention defines how long to keep metrics
	MetricsRetention string `json:"metricsRetention,omitempty"`

	// Reports defines report generation
	Reports *SLAReports `json:"reports,omitempty"`
}

// SLAReports defines SLA report generation
type SLAReports struct {
	// Frequency of reports (daily, weekly, monthly, quarterly)
	Frequency string `json:"frequency,omitempty"`

	// Recipients for reports
	Recipients []string `json:"recipients,omitempty"`
}

// GryviaSLAStatus defines the observed state of GryviaSLA
type GryviaSLAStatus struct {
	// Compliance tracks SLA compliance
	Compliance *SLAComplianceStatus `json:"compliance,omitempty"`

	// Metrics tracks SLA performance metrics
	Metrics *SLAMetricsStatus `json:"metrics,omitempty"`

	// Breaches is the list of SLA breaches
	Breaches []SLABreach `json:"breaches,omitempty"`

	// Conditions represent the latest available observations
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// SLAComplianceStatus tracks compliance
type SLAComplianceStatus struct {
	// Current compliance percentage
	Current float64 `json:"current,omitempty"`

	// Trend (improving, stable, degrading)
	Trend string `json:"trend,omitempty"`

	// LastMonth compliance
	LastMonth float64 `json:"lastMonth,omitempty"`
}

// SLAMetricsStatus tracks performance metrics
type SLAMetricsStatus struct {
	// AvgQueueTime is the average queue time
	AvgQueueTime string `json:"avgQueueTime,omitempty"`

	// P95QueueTime is the 95th percentile queue time
	P95QueueTime string `json:"p95QueueTime,omitempty"`

	// P99QueueTime is the 99th percentile queue time
	P99QueueTime string `json:"p99QueueTime,omitempty"`

	// Uptime is the current uptime percentage
	Uptime float64 `json:"uptime,omitempty"`

	// Breaches is the total number of breaches
	Breaches int `json:"breaches,omitempty"`
}

// SLABreach records an SLA breach
type SLABreach struct {
	// Timestamp of the breach
	Timestamp metav1.Time `json:"timestamp,omitempty"`

	// Type of breach (queue-time-exceeded, downtime-exceeded, etc.)
	Type string `json:"type,omitempty"`

	// Severity of the breach
	Severity string `json:"severity,omitempty"`

	// Duration of the breach
	Duration string `json:"duration,omitempty"`

	// Compensated indicates if compensation was applied
	Compensated bool `json:"compensated,omitempty"`
}

//+kubebuilder:deprecatedversion:warning="no controller reconciles this kind and its spec is not executed; it is kept readable for migration and will be removed in a future release"
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster

// GryviaSLA is the Schema for the gryviaslas API
type GryviaSLA struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GryviaSLASpec   `json:"spec,omitempty"`
	Status GryviaSLAStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// GryviaSLAList contains a list of GryviaSLA
type GryviaSLAList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GryviaSLA `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GryviaSLA{}, &GryviaSLAList{})
}
