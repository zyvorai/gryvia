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

// =============================================================================
// FabricSecurityPolicy - eBPF-based security detection and enforcement
// =============================================================================

// SecurityDetectionRule defines a single security detection rule
type SecurityDetectionRule struct {
	// Type is the detection type (escape, mining, exfiltration, privesc, driver_fim)
	Type string `json:"type,omitempty"`

	// Enabled indicates whether this detection rule is active
	Enabled bool `json:"enabled,omitempty"`

	// Sensitivity controls the detection sensitivity (low, medium, high)
	Sensitivity string `json:"sensitivity,omitempty"`
}

// FabricSecurityPolicySpec defines the desired state of FabricSecurityPolicy
type FabricSecurityPolicySpec struct {
	// TargetNamespaces is the list of namespaces to monitor
	TargetNamespaces []string `json:"targetNamespaces,omitempty"`

	// DetectionRules defines the security detection rules to apply
	DetectionRules []SecurityDetectionRule `json:"detectionRules,omitempty"`

	// AlertWebhook is the URL to send security alerts to
	AlertWebhook string `json:"alertWebhook,omitempty"`

	// AutoBlock enables automatic blocking of detected threats
	AutoBlock bool `json:"autoBlock,omitempty"`
}

// FabricSecurityPolicyStatus defines the observed state of FabricSecurityPolicy
type FabricSecurityPolicyStatus struct {
	// Phase is the current reconciliation phase
	Phase string `json:"phase,omitempty"`

	// ActiveDetections is the number of active detection rules
	ActiveDetections int `json:"activeDetections,omitempty"`

	// AlertsTriggered is the total number of alerts triggered
	AlertsTriggered int `json:"alertsTriggered,omitempty"`

	// LastAlert is the timestamp of the last alert
	LastAlert metav1.Time `json:"lastAlert,omitempty"`

	// DetectionCounts maps detection type to count of triggered alerts
	DetectionCounts map[string]int `json:"detectionCounts,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// FabricSecurityPolicy is the Schema for the fabricsecuritypolicies API
type FabricSecurityPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FabricSecurityPolicySpec   `json:"spec,omitempty"`
	Status FabricSecurityPolicyStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// FabricSecurityPolicyList contains a list of FabricSecurityPolicy
type FabricSecurityPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FabricSecurityPolicy `json:"items"`
}

// =============================================================================
// FabricNetworkCost - Network cost tracking and reporting
// =============================================================================

// CostPerGB defines the cost rates for different traffic zones
type CostPerGB struct {
	// SameZone is the cost per GB for same-zone traffic
	SameZone float64 `json:"sameZone,omitempty"`

	// CrossZone is the cost per GB for cross-zone traffic
	CrossZone float64 `json:"crossZone,omitempty"`

	// InternetEgress is the cost per GB for internet egress traffic
	InternetEgress float64 `json:"internetEgress,omitempty"`
}

// CostCenterMapping maps a namespace to a team and cost center
type CostCenterMapping struct {
	// Namespace is the Kubernetes namespace
	Namespace string `json:"namespace,omitempty"`

	// Team is the team responsible for this namespace
	Team string `json:"team,omitempty"`

	// CostCenter is the cost center identifier
	CostCenter string `json:"costCenter,omitempty"`
}

// NetworkCostReport contains a periodic cost report for a namespace/team
type NetworkCostReport struct {
	// Period is the reporting period (e.g., "2024-01-15")
	Period string `json:"period,omitempty"`

	// Namespace is the Kubernetes namespace
	Namespace string `json:"namespace,omitempty"`

	// Team is the team responsible
	Team string `json:"team,omitempty"`

	// SameZoneBytes is the total bytes for same-zone traffic
	SameZoneBytes int64 `json:"sameZoneBytes,omitempty"`

	// CrossZoneBytes is the total bytes for cross-zone traffic
	CrossZoneBytes int64 `json:"crossZoneBytes,omitempty"`

	// ExternalBytes is the total bytes for external traffic
	ExternalBytes int64 `json:"externalBytes,omitempty"`

	// TotalCostUSD is the total cost in USD for this period
	TotalCostUSD float64 `json:"totalCostUSD,omitempty"`
}

// FabricNetworkCostSpec defines the desired state of FabricNetworkCost
type FabricNetworkCostSpec struct {
	// TargetNamespaces is the list of namespaces to track costs for
	TargetNamespaces []string `json:"targetNamespaces,omitempty"`

	// CostPerGB defines the cost rates for different traffic zones
	CostPerGB CostPerGB `json:"costPerGB,omitempty"`

	// ReportingInterval is the interval between cost reports (e.g., "1h", "24h")
	ReportingInterval string `json:"reportingInterval,omitempty"`

	// CostCenters maps namespaces to teams and cost centers
	CostCenters []CostCenterMapping `json:"costCenters,omitempty"`
}

// FabricNetworkCostStatus defines the observed state of FabricNetworkCost
type FabricNetworkCostStatus struct {
	// Phase is the current reconciliation phase
	Phase string `json:"phase,omitempty"`

	// Reports is the list of generated cost reports
	Reports []NetworkCostReport `json:"reports,omitempty"`

	// LastReport is the timestamp of the last cost report
	LastReport metav1.Time `json:"lastReport,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// FabricNetworkCost is the Schema for the fabricnetworkcosts API
type FabricNetworkCost struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FabricNetworkCostSpec   `json:"spec,omitempty"`
	Status FabricNetworkCostStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// FabricNetworkCostList contains a list of FabricNetworkCost
type FabricNetworkCostList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FabricNetworkCost `json:"items"`
}

// =============================================================================
// FabricTrainingInsight - NCCL/training communication analysis
// =============================================================================

// RankStat contains per-rank communication statistics
type RankStat struct {
	// Rank is the distributed training rank
	Rank int `json:"rank,omitempty"`

	// AvgLatencyNs is the average communication latency in nanoseconds
	AvgLatencyNs int64 `json:"avgLatencyNs,omitempty"`

	// TotalBytes is the total bytes communicated by this rank
	TotalBytes int64 `json:"totalBytes,omitempty"`

	// IsStraggler indicates whether this rank is identified as a straggler
	IsStraggler bool `json:"isStraggler,omitempty"`
}

// StragglerInfo provides details about a detected straggler rank
type StragglerInfo struct {
	// Rank is the straggler rank
	Rank int `json:"rank,omitempty"`

	// SlowdownFactor is how much slower this rank is compared to the median
	SlowdownFactor float64 `json:"slowdownFactor,omitempty"`

	// Reason describes the suspected cause of the slowdown
	Reason string `json:"reason,omitempty"`
}

// FabricTrainingInsightSpec defines the desired state of FabricTrainingInsight
type FabricTrainingInsightSpec struct {
	// TargetJob is the name of the FabricAIJob to analyze
	TargetJob string `json:"targetJob,omitempty"`

	// AnalysisWindow is the time window for analysis (e.g., "5m", "1h")
	AnalysisWindow string `json:"analysisWindow,omitempty"`

	// Metrics is the list of analysis metrics to collect
	// Supported: collective_timing, straggler_detection, communication_ratio, pattern_analysis
	Metrics []string `json:"metrics,omitempty"`
}

// FabricTrainingInsightStatus defines the observed state of FabricTrainingInsight
type FabricTrainingInsightStatus struct {
	// Phase is the current analysis phase
	Phase string `json:"phase,omitempty"`

	// RankStats contains per-rank communication statistics
	RankStats []RankStat `json:"rankStats,omitempty"`

	// CommPattern is the detected communication pattern (ring_allreduce, tree_allreduce, pipelined)
	CommPattern string `json:"commPattern,omitempty"`

	// CommComputeRatio is the ratio of communication time to compute time
	CommComputeRatio float64 `json:"commComputeRatio,omitempty"`

	// Stragglers lists detected straggler ranks
	Stragglers []StragglerInfo `json:"stragglers,omitempty"`

	// Bottleneck identifies the primary bottleneck (compute, communication, data_loading)
	Bottleneck string `json:"bottleneck,omitempty"`

	// LastAnalysis is the timestamp of the last analysis
	LastAnalysis metav1.Time `json:"lastAnalysis,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// FabricTrainingInsight is the Schema for the fabrictraininginsights API
type FabricTrainingInsight struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FabricTrainingInsightSpec   `json:"spec,omitempty"`
	Status FabricTrainingInsightStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// FabricTrainingInsightList contains a list of FabricTrainingInsight
type FabricTrainingInsightList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FabricTrainingInsight `json:"items"`
}

// =============================================================================
// FabricInferenceInsight - Inference latency breakdown analysis
// =============================================================================

// LatencyBreakdown provides per-phase latency measurements
type LatencyBreakdown struct {
	// DNSNs is the DNS resolution latency in nanoseconds
	DNSNs int64 `json:"dnsNs,omitempty"`

	// TCPConnectNs is the TCP connection establishment latency in nanoseconds
	TCPConnectNs int64 `json:"tcpConnectNs,omitempty"`

	// TLSHandshakeNs is the TLS handshake latency in nanoseconds
	TLSHandshakeNs int64 `json:"tlsHandshakeNs,omitempty"`

	// GPUQueueNs is the GPU queue waiting latency in nanoseconds
	GPUQueueNs int64 `json:"gpuQueueNs,omitempty"`

	// GPUExecNs is the GPU execution latency in nanoseconds
	GPUExecNs int64 `json:"gpuExecNs,omitempty"`

	// PostprocessNs is the post-processing latency in nanoseconds
	PostprocessNs int64 `json:"postprocessNs,omitempty"`

	// TotalNs is the total end-to-end latency in nanoseconds
	TotalNs int64 `json:"totalNs,omitempty"`
}

// FabricInferenceInsightSpec defines the desired state of FabricInferenceInsight
type FabricInferenceInsightSpec struct {
	// TargetService is the name of the FabricInferenceService to analyze
	TargetService string `json:"targetService,omitempty"`

	// AnalysisWindow is the time window for analysis (e.g., "5m", "1h")
	AnalysisWindow string `json:"analysisWindow,omitempty"`
}

// FabricInferenceInsightStatus defines the observed state of FabricInferenceInsight
type FabricInferenceInsightStatus struct {
	// Phase is the current analysis phase
	Phase string `json:"phase,omitempty"`

	// LatencyBreakdown provides per-phase latency measurements
	LatencyBreakdown LatencyBreakdown `json:"latencyBreakdown,omitempty"`

	// P50TotalNs is the 50th percentile total latency in nanoseconds
	P50TotalNs int64 `json:"p50TotalNs,omitempty"`

	// P95TotalNs is the 95th percentile total latency in nanoseconds
	P95TotalNs int64 `json:"p95TotalNs,omitempty"`

	// P99TotalNs is the 99th percentile total latency in nanoseconds
	P99TotalNs int64 `json:"p99TotalNs,omitempty"`

	// Bottleneck identifies the phase with the highest latency contribution
	Bottleneck string `json:"bottleneck,omitempty"`

	// LastAnalysis is the timestamp of the last analysis
	LastAnalysis metav1.Time `json:"lastAnalysis,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// FabricInferenceInsight is the Schema for the fabricinferenceinsights API
type FabricInferenceInsight struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FabricInferenceInsightSpec   `json:"spec,omitempty"`
	Status FabricInferenceInsightStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// FabricInferenceInsightList contains a list of FabricInferenceInsight
type FabricInferenceInsightList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FabricInferenceInsight `json:"items"`
}

func init() {
	SchemeBuilder.Register(
		&FabricFlowPolicy{}, &FabricFlowPolicyList{},
		&FabricTrafficInsight{}, &FabricTrafficInsightList{},
		&FabricAutoPolicy{}, &FabricAutoPolicyList{},
		&FabricTraceSession{}, &FabricTraceSessionList{},
		&FabricServiceGraph{}, &FabricServiceGraphList{},
		&FabricNetworkAnomaly{}, &FabricNetworkAnomalyList{},
		&FabricSecurityPolicy{}, &FabricSecurityPolicyList{},
		&FabricNetworkCost{}, &FabricNetworkCostList{},
		&FabricTrainingInsight{}, &FabricTrainingInsightList{},
		&FabricInferenceInsight{}, &FabricInferenceInsightList{},
	)
}
