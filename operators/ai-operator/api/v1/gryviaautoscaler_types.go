package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GryviaAutoScalerSpec defines the desired state of GryviaAutoScaler
type GryviaAutoScalerSpec struct {
	// QueueRef is the name of GryviaQueue to autoscale
	QueueRef string `json:"queueRef"`

	// GpuType is the GPU type to add/remove
	GpuType string `json:"gpuType"`

	// MinNodes is the minimum number of nodes
	MinNodes int32 `json:"minNodes,omitempty"`

	// MaxNodes is the maximum number of nodes
	MaxNodes int32 `json:"maxNodes"`

	// ScaleUpPolicy defines when and how to scale up
	ScaleUpPolicy *ScaleUpPolicy `json:"scaleUpPolicy,omitempty"`

	// ScaleDownPolicy defines when and how to scale down
	ScaleDownPolicy *ScaleDownPolicy `json:"scaleDownPolicy,omitempty"`

	// NodeProvider defines how to provision nodes
	NodeProvider *NodeProvider `json:"nodeProvider,omitempty"`

	// CostControls defines cost limits for autoscaling
	CostControls *CostControls `json:"costControls,omitempty"`

	// Advanced defines advanced scaling options
	Advanced *AdvancedScaling `json:"advanced,omitempty"`
}

// ScaleUpPolicy defines scale up triggers and configuration
type ScaleUpPolicy struct {
	// PendingJobs triggers scale up if pending jobs exceed threshold
	PendingJobs int32 `json:"pendingJobs,omitempty"`

	// QueueTimeMinutes triggers scale up if avg queue time exceeds threshold
	QueueTimeMinutes int32 `json:"queueTimeMinutes,omitempty"`

	// UtilizationPercent triggers scale up if utilization exceeds threshold
	UtilizationPercent int32 `json:"utilizationPercent,omitempty"`

	// Increment is the number of nodes to add at a time
	Increment int32 `json:"increment,omitempty"`

	// CooldownMinutes is the cooldown period after a scale up
	CooldownMinutes int32 `json:"cooldownMinutes,omitempty"`
}

// ScaleDownPolicy defines scale down triggers and configuration
type ScaleDownPolicy struct {
	// IdleTimeMinutes triggers scale down if idle exceeds threshold
	IdleTimeMinutes int32 `json:"idleTimeMinutes,omitempty"`

	// UtilizationPercent triggers scale down if utilization is below threshold
	UtilizationPercent int32 `json:"utilizationPercent,omitempty"`

	// Decrement is the number of nodes to remove at a time
	Decrement int32 `json:"decrement,omitempty"`

	// CooldownMinutes is the cooldown period after a scale down
	CooldownMinutes int32 `json:"cooldownMinutes,omitempty"`
}

// NodeProvider defines the node provisioning backend
type NodeProvider struct {
	// Type is the provider type (bare-metal, cloud-provision)
	Type string `json:"type"`

	// Cloud is the cloud provider configuration
	Cloud *CloudProvider `json:"cloud,omitempty"`
}

// CloudProvider defines cloud-specific provisioning
type CloudProvider struct {
	// Provider is the cloud provider (aws, gcp, azure)
	Provider string `json:"provider"`

	// InstanceType is the cloud instance type
	InstanceType string `json:"instanceType,omitempty"`

	// Region is the cloud region
	Region string `json:"region,omitempty"`

	// Zone is the cloud zone
	Zone string `json:"zone,omitempty"`

	// SpotInstances enables spot/preemptible instances
	SpotInstances bool `json:"spotInstances,omitempty"`
}

// CostControls defines cost limits
type CostControls struct {
	// MaxHourlyCost is the maximum hourly cost for autoscaled nodes
	MaxHourlyCost float64 `json:"maxHourlyCost,omitempty"`

	// PreferSpot enables preference for spot instances
	PreferSpot bool `json:"preferSpot,omitempty"`

	// MaxSpotPrice is the maximum spot price per instance
	MaxSpotPrice float64 `json:"maxSpotPrice,omitempty"`
}

// AdvancedScaling defines advanced scaling options
type AdvancedScaling struct {
	// Predictive enables ML-based predictive scaling
	Predictive bool `json:"predictive,omitempty"`

	// ScaleToZero allows scaling down to 0 nodes
	ScaleToZero bool `json:"scaleToZero,omitempty"`
}

// AutoScalerMetrics holds current autoscaler metrics
type AutoScalerMetrics struct {
	// PendingJobs is the current number of pending jobs
	PendingJobs int32 `json:"pendingJobs,omitempty"`

	// AvgQueueTimeMinutes is the average queue wait time
	AvgQueueTimeMinutes float64 `json:"avgQueueTimeMinutes,omitempty"`

	// UtilizationPercent is the current GPU utilization
	UtilizationPercent float64 `json:"utilizationPercent,omitempty"`
}

// GryviaAutoScalerStatus defines the observed state of GryviaAutoScaler
type GryviaAutoScalerStatus struct {
	// CurrentNodes is the current number of nodes
	CurrentNodes int32 `json:"currentNodes,omitempty"`

	// DesiredNodes is the desired number of nodes
	DesiredNodes int32 `json:"desiredNodes,omitempty"`

	// PendingNodes is the number of nodes being provisioned
	PendingNodes int32 `json:"pendingNodes,omitempty"`

	// LastScaleTime is the time of the last scaling action
	LastScaleTime *metav1.Time `json:"lastScaleTime,omitempty"`

	// LastScaleAction is the type of the last scaling action
	LastScaleAction string `json:"lastScaleAction,omitempty"`

	// Metrics holds current autoscaler metrics
	Metrics *AutoScalerMetrics `json:"metrics,omitempty"`

	// Conditions represent the latest available observations
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster
//+kubebuilder:printcolumn:name="Queue",type=string,JSONPath=`.spec.queueRef`
//+kubebuilder:printcolumn:name="GPU-Type",type=string,JSONPath=`.spec.gpuType`
//+kubebuilder:printcolumn:name="Current",type=integer,JSONPath=`.status.currentNodes`
//+kubebuilder:printcolumn:name="Desired",type=integer,JSONPath=`.status.desiredNodes`
//+kubebuilder:printcolumn:name="Min",type=integer,JSONPath=`.spec.minNodes`
//+kubebuilder:printcolumn:name="Max",type=integer,JSONPath=`.spec.maxNodes`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// GryviaAutoScaler is the Schema for the gryviaautoscalers API
type GryviaAutoScaler struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GryviaAutoScalerSpec   `json:"spec,omitempty"`
	Status GryviaAutoScalerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// GryviaAutoScalerList contains a list of GryviaAutoScaler
type GryviaAutoScalerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GryviaAutoScaler `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GryviaAutoScaler{}, &GryviaAutoScalerList{})
}
