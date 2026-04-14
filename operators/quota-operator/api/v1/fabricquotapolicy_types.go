package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FabricQuotaPolicySpec defines the desired state of FabricQuotaPolicy
type FabricQuotaPolicySpec struct {
	// Scope defines what this quota policy applies to
	Scope QuotaPolicyScope `json:"scope,omitempty"`

	// Hierarchy defines hierarchical quota relationships
	Hierarchy *QuotaPolicyHierarchy `json:"hierarchy,omitempty"`

	// Limits defines resource limits
	Limits QuotaPolicyLimits `json:"limits,omitempty"`

	// Allocation defines how quota is allocated among children
	Allocation *QuotaPolicyAllocation `json:"allocation,omitempty"`

	// Enforcement defines enforcement policy
	Enforcement *QuotaPolicyEnforcement `json:"enforcement,omitempty"`

	// TimeBased defines time-based quota policies
	TimeBased *QuotaPolicyTimeBased `json:"timeBased,omitempty"`

	// Alerts defines alert configuration
	Alerts *QuotaPolicyAlerts `json:"alerts,omitempty"`
}

// QuotaPolicyScope defines the scope
type QuotaPolicyScope struct {
	// Type is the scope type (organization, division, team, user, namespace)
	Type string `json:"type,omitempty"`

	// Name of the scoped entity
	Name string `json:"name,omitempty"`
}

// QuotaPolicyHierarchy defines hierarchy settings
type QuotaPolicyHierarchy struct {
	// Enabled enables hierarchical quota
	Enabled bool `json:"enabled,omitempty"`

	// ParentRef is reference to parent quota policy
	ParentRef string `json:"parentRef,omitempty"`

	// AllowChildren allows child quotas
	AllowChildren bool `json:"allowChildren,omitempty"`

	// InheritLimits inherits limits from parent
	InheritLimits bool `json:"inheritLimits,omitempty"`
}

// QuotaPolicyLimits defines resource limits
type QuotaPolicyLimits struct {
	// GPU defines GPU-related limits
	GPU *QuotaPolicyGPULimits `json:"gpu,omitempty"`

	// Cost defines cost limits
	Cost *QuotaPolicyCostLimits `json:"cost,omitempty"`

	// Storage defines storage limits
	Storage *QuotaPolicyStorageLimits `json:"storage,omitempty"`

	// Jobs defines job limits
	Jobs *QuotaPolicyJobLimits `json:"jobs,omitempty"`
}

// QuotaPolicyGPULimits defines GPU limits
type QuotaPolicyGPULimits struct {
	// HoursPerMonth is the total GPU hours allowed per month
	HoursPerMonth float64 `json:"hoursPerMonth,omitempty"`

	// Concurrent is the max concurrent GPUs
	Concurrent int `json:"concurrent,omitempty"`

	// ByType is per-GPU-type limits
	ByType map[string]GPUTypeLimits `json:"byType,omitempty"`

	// MemoryGB is GPU memory limit
	MemoryGB float64 `json:"memoryGB,omitempty"`
}

// GPUTypeLimits defines limits per GPU type
type GPUTypeLimits struct {
	// Count is the max concurrent GPUs of this type
	Count int `json:"count,omitempty"`

	// HoursPerMonth is the max hours per month for this type
	HoursPerMonth float64 `json:"hoursPerMonth,omitempty"`
}

// QuotaPolicyCostLimits defines cost limits
type QuotaPolicyCostLimits struct {
	// MonthlyBudget in USD
	MonthlyBudget float64 `json:"monthlyBudget,omitempty"`

	// QuarterlyBudget in USD
	QuarterlyBudget float64 `json:"quarterlyBudget,omitempty"`

	// AnnualBudget in USD
	AnnualBudget float64 `json:"annualBudget,omitempty"`

	// PerJobLimit in USD
	PerJobLimit float64 `json:"perJobLimit,omitempty"`
}

// QuotaPolicyStorageLimits defines storage limits
type QuotaPolicyStorageLimits struct {
	// TotalGB is the total storage in GB
	TotalGB float64 `json:"totalGB,omitempty"`

	// ByClass is per-storage-class limits in GB
	ByClass map[string]float64 `json:"byClass,omitempty"`
}

// QuotaPolicyJobLimits defines job limits
type QuotaPolicyJobLimits struct {
	// Concurrent is the max concurrent jobs
	Concurrent int `json:"concurrent,omitempty"`

	// PerDay is the max jobs per day
	PerDay int `json:"perDay,omitempty"`

	// PerMonth is the max jobs per month
	PerMonth int `json:"perMonth,omitempty"`
}

// QuotaPolicyAllocation defines allocation strategy
type QuotaPolicyAllocation struct {
	// Strategy is the allocation strategy (equal, weighted, priority-based, dynamic)
	Strategy string `json:"strategy,omitempty"`

	// Weights for weighted allocation
	Weights map[string]int `json:"weights,omitempty"`

	// BorrowingEnabled allows borrowing from siblings
	BorrowingEnabled bool `json:"borrowingEnabled,omitempty"`

	// MaxBorrowPercentage is the max percentage that can be borrowed
	MaxBorrowPercentage int `json:"maxBorrowPercentage,omitempty"`
}

// QuotaPolicyEnforcement defines enforcement policy
type QuotaPolicyEnforcement struct {
	// Type is hard, soft, or adaptive
	Type string `json:"type,omitempty"`

	// OnExceeded is the action when quota is exceeded (block, queue, throttle, notify)
	OnExceeded string `json:"onExceeded,omitempty"`

	// GracePeriod before enforcement
	GracePeriod string `json:"gracePeriod,omitempty"`

	// AllowBurst allows temporary over-quota usage
	AllowBurst bool `json:"allowBurst,omitempty"`

	// BurstLimit is the percentage above quota allowed for bursting
	BurstLimit float64 `json:"burstLimit,omitempty"`
}

// QuotaPolicyTimeBased defines time-based policies
type QuotaPolicyTimeBased struct {
	// Enabled enables time-based quotas
	Enabled bool `json:"enabled,omitempty"`

	// Schedules defines different quota periods
	Schedules []QuotaPolicySchedule `json:"schedules,omitempty"`
}

// QuotaPolicySchedule defines a time-based quota schedule
type QuotaPolicySchedule struct {
	// Name of the schedule
	Name string `json:"name,omitempty"`

	// Window defines the time window
	Window *QuotaPolicyTimeWindow `json:"window,omitempty"`

	// Multiplier adjusts quota for this window (e.g., 1.5 = 50% more)
	Multiplier float64 `json:"multiplier,omitempty"`
}

// QuotaPolicyTimeWindow defines a time window
type QuotaPolicyTimeWindow struct {
	// DaysOfWeek (Mon, Tue, Wed, Thu, Fri, Sat, Sun)
	DaysOfWeek []string `json:"daysOfWeek,omitempty"`

	// StartTime in HH:MM format
	StartTime string `json:"startTime,omitempty"`

	// EndTime in HH:MM format
	EndTime string `json:"endTime,omitempty"`
}

// QuotaPolicyAlerts defines alerts
type QuotaPolicyAlerts struct {
	// Enabled enables alerts
	Enabled bool `json:"enabled,omitempty"`

	// Thresholds are percentages at which to alert
	Thresholds []int `json:"thresholds,omitempty"`

	// Channels for notifications (email, slack, pagerduty, webhook)
	Channels []string `json:"channels,omitempty"`

	// Recipients for notifications
	Recipients []string `json:"recipients,omitempty"`
}

// FabricQuotaPolicyStatus defines the observed state of FabricQuotaPolicy
type FabricQuotaPolicyStatus struct {
	// Usage tracks current resource consumption
	Usage *QuotaPolicyUsage `json:"usage,omitempty"`

	// Utilization tracks utilization percentages
	Utilization *QuotaPolicyUtilization `json:"utilization,omitempty"`

	// State is the quota state (under-quota, near-limit, exceeded, blocked)
	State string `json:"state,omitempty"`

	// Children tracks child quota allocations
	Children []QuotaPolicyChildStatus `json:"children,omitempty"`

	// ActiveSchedule is the currently active time-based schedule
	ActiveSchedule string `json:"activeSchedule,omitempty"`

	// EffectiveMultiplier is the current quota multiplier from time-based policy
	EffectiveMultiplier float64 `json:"effectiveMultiplier,omitempty"`

	// Conditions represent the latest available observations
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// QuotaPolicyUsage tracks current usage
type QuotaPolicyUsage struct {
	// GPU usage
	GPU *QuotaPolicyGPUUsage `json:"gpu,omitempty"`

	// Cost usage
	Cost *QuotaPolicyCostUsage `json:"cost,omitempty"`

	// Jobs usage
	Jobs *QuotaPolicyJobUsage `json:"jobs,omitempty"`
}

// QuotaPolicyGPUUsage tracks GPU usage
type QuotaPolicyGPUUsage struct {
	// HoursThisMonth is GPU hours consumed this month
	HoursThisMonth float64 `json:"hoursThisMonth,omitempty"`

	// Concurrent is currently allocated GPUs
	Concurrent int `json:"concurrent,omitempty"`

	// ByType is per-GPU-type usage
	ByType map[string]float64 `json:"byType,omitempty"`
}

// QuotaPolicyCostUsage tracks cost usage
type QuotaPolicyCostUsage struct {
	// MonthToDate spending
	MonthToDate float64 `json:"monthToDate,omitempty"`

	// QuarterToDate spending
	QuarterToDate float64 `json:"quarterToDate,omitempty"`

	// YearToDate spending
	YearToDate float64 `json:"yearToDate,omitempty"`
}

// QuotaPolicyJobUsage tracks job usage
type QuotaPolicyJobUsage struct {
	// Concurrent is currently running jobs
	Concurrent int `json:"concurrent,omitempty"`

	// Today is jobs run today
	Today int `json:"today,omitempty"`

	// ThisMonth is jobs run this month
	ThisMonth int `json:"thisMonth,omitempty"`
}

// QuotaPolicyUtilization tracks utilization percentages
type QuotaPolicyUtilization struct {
	// GPU utilization percentage
	GPU float64 `json:"gpu,omitempty"`

	// Cost utilization percentage
	Cost float64 `json:"cost,omitempty"`

	// Storage utilization percentage
	Storage float64 `json:"storage,omitempty"`
}

// QuotaPolicyChildStatus tracks child quota status
type QuotaPolicyChildStatus struct {
	// Name of the child quota policy
	Name string `json:"name,omitempty"`

	// Allocated quota
	Allocated float64 `json:"allocated,omitempty"`

	// Used quota
	Used float64 `json:"used,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster

// FabricQuotaPolicy is the Schema for the fabricquotapolicies API
type FabricQuotaPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FabricQuotaPolicySpec   `json:"spec,omitempty"`
	Status FabricQuotaPolicyStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// FabricQuotaPolicyList contains a list of FabricQuotaPolicy
type FabricQuotaPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FabricQuotaPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FabricQuotaPolicy{}, &FabricQuotaPolicyList{})
}
