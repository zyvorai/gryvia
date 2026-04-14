package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// =============================================================================
// FabricFlowPolicy - Intent-based network policy
// =============================================================================

// FabricFlowPolicySpec defines the desired state of FabricFlowPolicy
type FabricFlowPolicySpec struct {
	// Source defines the traffic source selector
	Source *FlowEndpoint `json:"source,omitempty"`

	// Destination defines the traffic destination selector
	Destination *FlowEndpoint `json:"destination,omitempty"`

	// Protocol is the network protocol (tcp, udp, icmp, any)
	Protocol string `json:"protocol,omitempty"`

	// Action specifies what to do with matching traffic (allow, deny, log)
	Action string `json:"action,omitempty"`

	// Intent describes the desired network behavior (low-latency, high-throughput, secure, default)
	Intent string `json:"intent,omitempty"`

	// Priority determines policy evaluation order (0-1000, lower is higher priority)
	Priority int `json:"priority,omitempty"`
}

// FlowEndpoint identifies a network endpoint by service, namespace, or labels
type FlowEndpoint struct {
	// Service is the Kubernetes service name
	Service string `json:"service,omitempty"`

	// Namespace is the Kubernetes namespace
	Namespace string `json:"namespace,omitempty"`

	// Port is the network port (destination only)
	Port int `json:"port,omitempty"`

	// Labels are key-value selectors for pod matching
	Labels map[string]string `json:"labels,omitempty"`
}

// FabricFlowPolicyStatus defines the observed state of FabricFlowPolicy
type FabricFlowPolicyStatus struct {
	// Phase is the current reconciliation phase
	Phase string `json:"phase,omitempty"`

	// CiliumPolicyRef is the name of the generated CiliumNetworkPolicy
	CiliumPolicyRef string `json:"ciliumPolicyRef,omitempty"`

	// Enforced indicates whether the policy is actively enforced
	Enforced bool `json:"enforced,omitempty"`

	// LastApplied is when the policy was last applied
	LastApplied metav1.Time `json:"lastApplied,omitempty"`

	// MatchedFlows is the number of flows matching this policy
	MatchedFlows int64 `json:"matchedFlows,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// FabricFlowPolicy is the Schema for the fabricflowpolicies API
type FabricFlowPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FabricFlowPolicySpec   `json:"spec,omitempty"`
	Status FabricFlowPolicyStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// FabricFlowPolicyList contains a list of FabricFlowPolicy
type FabricFlowPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FabricFlowPolicy `json:"items"`
}

// =============================================================================
// FabricTrafficInsight - Real-time traffic analysis
// =============================================================================

// FabricTrafficInsightSpec defines the desired state of FabricTrafficInsight
type FabricTrafficInsightSpec struct {
	// Service is the target service to analyze
	Service string `json:"service,omitempty"`

	// Namespace is the namespace of the target service
	Namespace string `json:"namespace,omitempty"`

	// Window is the duration window for traffic analysis (e.g., 5m, 1h)
	Window string `json:"window,omitempty"`

	// Metrics is the list of metrics to collect (latency, throughput, drops, retransmits)
	Metrics []string `json:"metrics,omitempty"`
}

// TopTalker represents a service with high traffic volume
type TopTalker struct {
	// Service is the name of the communicating service
	Service string `json:"service,omitempty"`

	// Bytes is the total bytes transferred
	Bytes int64 `json:"bytes,omitempty"`

	// Latency is the average latency to this service
	Latency string `json:"latency,omitempty"`
}

// TrafficAnomaly represents a detected traffic anomaly
type TrafficAnomaly struct {
	// Type is the anomaly classification
	Type string `json:"type,omitempty"`

	// Severity is the anomaly severity level
	Severity string `json:"severity,omitempty"`

	// Description provides human-readable details
	Description string `json:"description,omitempty"`

	// Detected is when the anomaly was first observed
	Detected metav1.Time `json:"detected,omitempty"`
}

// FabricTrafficInsightStatus defines the observed state of FabricTrafficInsight
type FabricTrafficInsightStatus struct {
	// P50Latency is the 50th percentile latency
	P50Latency string `json:"p50Latency,omitempty"`

	// P99Latency is the 99th percentile latency
	P99Latency string `json:"p99Latency,omitempty"`

	// ThroughputBps is the current throughput in bytes per second
	ThroughputBps int64 `json:"throughputBps,omitempty"`

	// DropCount is the number of dropped packets
	DropCount int64 `json:"dropCount,omitempty"`

	// TopTalkers lists services with the most traffic
	TopTalkers []TopTalker `json:"topTalkers,omitempty"`

	// Anomalies lists detected traffic anomalies
	Anomalies []TrafficAnomaly `json:"anomalies,omitempty"`

	// LastUpdated is the timestamp of the last metrics update
	LastUpdated metav1.Time `json:"lastUpdated,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// FabricTrafficInsight is the Schema for the fabrictrafficinsights API
type FabricTrafficInsight struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FabricTrafficInsightSpec   `json:"spec,omitempty"`
	Status FabricTrafficInsightStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// FabricTrafficInsightList contains a list of FabricTrafficInsight
type FabricTrafficInsightList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FabricTrafficInsight `json:"items"`
}

// =============================================================================
// FabricAutoPolicy - Self-healing firewall / auto-policy generation
// =============================================================================

// FabricAutoPolicySpec defines the desired state of FabricAutoPolicy
type FabricAutoPolicySpec struct {
	// Mode determines the operational mode (learn, suggest, enforce)
	Mode string `json:"mode,omitempty"`

	// LearningWindow is the duration for the learning phase (e.g., 24h, 7d)
	LearningWindow string `json:"learningWindow,omitempty"`

	// TargetNamespaces is the list of namespaces to monitor
	TargetNamespaces []string `json:"targetNamespaces,omitempty"`

	// ExcludeServices is the list of services to exclude from policy generation
	ExcludeServices []string `json:"excludeServices,omitempty"`

	// ApprovalRequired indicates whether human approval is needed before enforcement
	ApprovalRequired bool `json:"approvalRequired,omitempty"`
}

// SuggestedPolicy represents a policy recommendation based on learned traffic
type SuggestedPolicy struct {
	// Source is the traffic source service
	Source string `json:"source,omitempty"`

	// Destination is the traffic destination service
	Destination string `json:"destination,omitempty"`

	// Port is the destination port
	Port int `json:"port,omitempty"`

	// Confidence is the confidence score (0.0 - 1.0)
	Confidence float64 `json:"confidence,omitempty"`
}

// FabricAutoPolicyStatus defines the observed state of FabricAutoPolicy
type FabricAutoPolicyStatus struct {
	// Phase is the current operational phase (learning, suggesting, enforcing)
	Phase string `json:"phase,omitempty"`

	// LearnedPolicies is the number of traffic patterns learned
	LearnedPolicies int `json:"learnedPolicies,omitempty"`

	// SuggestedPolicies is the list of suggested network policies
	SuggestedPolicies []SuggestedPolicy `json:"suggestedPolicies,omitempty"`

	// AppliedPolicies is the number of policies that have been enforced
	AppliedPolicies int `json:"appliedPolicies,omitempty"`

	// LastLearned is the timestamp of the last learning cycle
	LastLearned metav1.Time `json:"lastLearned,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// FabricAutoPolicy is the Schema for the fabricautopolicies API
type FabricAutoPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FabricAutoPolicySpec   `json:"spec,omitempty"`
	Status FabricAutoPolicyStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// FabricAutoPolicyList contains a list of FabricAutoPolicy
type FabricAutoPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FabricAutoPolicy `json:"items"`
}

// =============================================================================
// FabricTraceSession - Network trace/debug session
// =============================================================================

// FabricTraceSessionSpec defines the desired state of FabricTraceSession
type FabricTraceSessionSpec struct {
	// Service is the target service to trace
	Service string `json:"service,omitempty"`

	// Namespace is the namespace of the target service
	Namespace string `json:"namespace,omitempty"`

	// Duration is the trace session lifetime (e.g., 5m, 30m)
	Duration string `json:"duration,omitempty"`

	// Level is the trace depth (l3, l4, l7)
	Level string `json:"level,omitempty"`

	// Filters define traffic filters for the trace session
	Filters *TraceFilters `json:"filters,omitempty"`

	// CaptureHeaders enables HTTP header capture (L7 only)
	CaptureHeaders bool `json:"captureHeaders,omitempty"`
}

// TraceFilters define packet matching criteria for trace sessions
type TraceFilters struct {
	// SrcIP filters by source IP address
	SrcIP string `json:"srcIP,omitempty"`

	// DstIP filters by destination IP address
	DstIP string `json:"dstIP,omitempty"`

	// Port filters by destination port
	Port int `json:"port,omitempty"`

	// Protocol filters by network protocol (tcp, udp, icmp, any)
	Protocol string `json:"protocol,omitempty"`
}

// TraceResultRef is a reference to stored trace data
type TraceResultRef struct {
	// Kind is the type of resource storing trace data (ConfigMap, PersistentVolumeClaim)
	Kind string `json:"kind,omitempty"`

	// Name is the name of the storage resource
	Name string `json:"name,omitempty"`

	// Namespace is the namespace of the storage resource
	Namespace string `json:"namespace,omitempty"`
}

// FabricTraceSessionStatus defines the observed state of FabricTraceSession
type FabricTraceSessionStatus struct {
	// Phase is the current session phase (active, completed, expired)
	Phase string `json:"phase,omitempty"`

	// FlowsCaptured is the number of network flows captured
	FlowsCaptured int64 `json:"flowsCaptured,omitempty"`

	// StartTime is when the trace session began
	StartTime metav1.Time `json:"startTime,omitempty"`

	// EndTime is when the trace session ended
	EndTime metav1.Time `json:"endTime,omitempty"`

	// ResultRef references the stored trace data
	ResultRef *TraceResultRef `json:"resultRef,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// FabricTraceSession is the Schema for the fabrictracesessions API
type FabricTraceSession struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FabricTraceSessionSpec   `json:"spec,omitempty"`
	Status FabricTraceSessionStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// FabricTraceSessionList contains a list of FabricTraceSession
type FabricTraceSessionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FabricTraceSession `json:"items"`
}

// =============================================================================
// FabricServiceGraph - Service dependency graph
// =============================================================================

// FabricServiceGraphSpec defines the desired state of FabricServiceGraph
type FabricServiceGraphSpec struct {
	// Namespaces is the list of namespaces to include in the graph
	Namespaces []string `json:"namespaces,omitempty"`

	// RefreshInterval is the interval between graph refreshes (e.g., 30s, 5m)
	RefreshInterval string `json:"refreshInterval,omitempty"`

	// IncludeExternal includes traffic to/from external endpoints
	IncludeExternal bool `json:"includeExternal,omitempty"`

	// Depth is the maximum graph traversal depth (1-10)
	Depth int `json:"depth,omitempty"`
}

// ServiceGraphNode represents a service in the dependency graph
type ServiceGraphNode struct {
	// Name is the service name
	Name string `json:"name,omitempty"`

	// Namespace is the service namespace
	Namespace string `json:"namespace,omitempty"`

	// Type is the node type (service, deployment, external)
	Type string `json:"type,omitempty"`

	// Health is the service health status
	Health string `json:"health,omitempty"`
}

// ServiceGraphEdge represents a connection between services
type ServiceGraphEdge struct {
	// Source is the source service name
	Source string `json:"source,omitempty"`

	// Destination is the destination service name
	Destination string `json:"destination,omitempty"`

	// Protocol is the network protocol used
	Protocol string `json:"protocol,omitempty"`

	// Port is the destination port
	Port int `json:"port,omitempty"`

	// LatencyP99 is the 99th percentile latency
	LatencyP99 string `json:"latencyP99,omitempty"`

	// Throughput is the traffic throughput
	Throughput string `json:"throughput,omitempty"`

	// Verdict indicates the traffic verdict (forwarded, dropped, error)
	Verdict string `json:"verdict,omitempty"`
}

// FabricServiceGraphStatus defines the observed state of FabricServiceGraph
type FabricServiceGraphStatus struct {
	// Nodes is the list of services in the graph
	Nodes []ServiceGraphNode `json:"nodes,omitempty"`

	// Edges is the list of connections between services
	Edges []ServiceGraphEdge `json:"edges,omitempty"`

	// LastUpdated is the timestamp of the last graph refresh
	LastUpdated metav1.Time `json:"lastUpdated,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// FabricServiceGraph is the Schema for the fabricservicegraphs API
type FabricServiceGraph struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FabricServiceGraphSpec   `json:"spec,omitempty"`
	Status FabricServiceGraphStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// FabricServiceGraphList contains a list of FabricServiceGraph
type FabricServiceGraphList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FabricServiceGraph `json:"items"`
}

// =============================================================================
// FabricNetworkAnomaly - Network anomaly detection
// =============================================================================

// FabricNetworkAnomalySpec defines the desired state of FabricNetworkAnomaly
type FabricNetworkAnomalySpec struct {
	// TargetService is the service to monitor for anomalies
	TargetService string `json:"targetService,omitempty"`

	// DetectionRules define the anomaly detection thresholds
	DetectionRules []DetectionRule `json:"detectionRules,omitempty"`

	// AlertWebhook is the URL to send anomaly alerts to
	AlertWebhook string `json:"alertWebhook,omitempty"`

	// AutoMitigate enables automatic mitigation of detected anomalies
	AutoMitigate bool `json:"autoMitigate,omitempty"`
}

// DetectionRule defines a threshold-based anomaly detection rule
type DetectionRule struct {
	// Metric is the network metric to monitor (latency, throughput, connections, errors, drops)
	Metric string `json:"metric,omitempty"`

	// Operator is the comparison operator (gt, lt, gte, lte, eq)
	Operator string `json:"operator,omitempty"`

	// Threshold is the threshold value for the metric
	Threshold float64 `json:"threshold,omitempty"`

	// Window is the time window for metric evaluation (e.g., 5m, 1h)
	Window string `json:"window,omitempty"`
}

// NetworkAnomalyEvent represents a detected network anomaly
type NetworkAnomalyEvent struct {
	// Type is the anomaly classification
	Type string `json:"type,omitempty"`

	// Severity is the anomaly severity (low, medium, high, critical)
	Severity string `json:"severity,omitempty"`

	// Detected is when the anomaly was detected
	Detected metav1.Time `json:"detected,omitempty"`

	// Description provides human-readable details
	Description string `json:"description,omitempty"`

	// Mitigated indicates whether automatic mitigation was applied
	Mitigated bool `json:"mitigated,omitempty"`
}

// FabricNetworkAnomalyStatus defines the observed state of FabricNetworkAnomaly
type FabricNetworkAnomalyStatus struct {
	// Anomalies is the list of detected anomalies
	Anomalies []NetworkAnomalyEvent `json:"anomalies,omitempty"`

	// LastCheck is the timestamp of the last anomaly check
	LastCheck metav1.Time `json:"lastCheck,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// FabricNetworkAnomaly is the Schema for the fabricnetworkanomalies API
type FabricNetworkAnomaly struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FabricNetworkAnomalySpec   `json:"spec,omitempty"`
	Status FabricNetworkAnomalyStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// FabricNetworkAnomalyList contains a list of FabricNetworkAnomaly
type FabricNetworkAnomalyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FabricNetworkAnomaly `json:"items"`
}

func init() {
	SchemeBuilder.Register(
		&FabricFlowPolicy{}, &FabricFlowPolicyList{},
		&FabricTrafficInsight{}, &FabricTrafficInsightList{},
		&FabricAutoPolicy{}, &FabricAutoPolicyList{},
		&FabricTraceSession{}, &FabricTraceSessionList{},
		&FabricServiceGraph{}, &FabricServiceGraphList{},
		&FabricNetworkAnomaly{}, &FabricNetworkAnomalyList{},
	)
}
