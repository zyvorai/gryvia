package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GryviaFabricSignalSpec defines the desired state of GryviaFabricSignal
type GryviaFabricSignalSpec struct {
	// JobRef is the name of the GryviaAIJob in the same namespace whose fabric
	// signals (NCCL stragglers, RDMA retries, GPUDirect Storage) are tracked
	JobRef string `json:"jobRef,omitempty"`

	// ObserveOnly keeps the signals advisory: nothing is enforced (no lease
	// revocation, no fail-open) based on them
	// +kubebuilder:default=true
	ObserveOnly *bool `json:"observeOnly,omitempty"`
}

// GryviaFabricSignalStatus defines the observed state of GryviaFabricSignal
type GryviaFabricSignalStatus struct {
	// StragglerRank is the rank of the most recent NCCL straggler
	StragglerRank int32 `json:"stragglerRank,omitempty"`

	// NCCLP99Ms is the p99 latency in milliseconds of straggler-flagged NCCL collectives
	NCCLP99Ms float64 `json:"ncclP99ms,omitempty"`

	// RDMARetryRate is the ratio of RDMA retry/RNR error completions to posted sends
	RDMARetryRate float64 `json:"rdmaRetryRate,omitempty"`

	// GDSHitRatio is the fraction of cuFile bytes served over the direct GPUDirect Storage path
	GDSHitRatio float64 `json:"gdsHitRatio,omitempty"`

	// OverlapIdleRatio is the fraction of the window the job spent in cudaDeviceSynchronize
	// inside an in-flight ncclAllReduce (GPU idle while communicating)
	OverlapIdleRatio float64 `json:"overlapIdleRatio,omitempty"`

	// CNPRate is the RoCEv2 congestion notification packets per second seen on the node
	CNPRate float64 `json:"cnpRate,omitempty"`

	// InferWaitP99Ms is the p99 wait in milliseconds from accept to first read on inference ports
	InferWaitP99Ms float64 `json:"inferWaitP99ms,omitempty"`

	// PFCRate is the 802.1Qbb priority-flow-control pause frames per second seen on the node
	PFCRate float64 `json:"pfcRate,omitempty"`

	// ExfilEvents is the number of large model-file reads followed by a connect to a non-internal
	// address from the same process in the window (observe only, never part of scoreDelta)
	ExfilEvents int64 `json:"exfilEvents,omitempty"`

	// CollectiveMaxSkewMs is the largest slowest-minus-fastest host-side NCCL call duration in
	// milliseconds among collectives matched by (communicator ordinal, sequence, op) across the
	// job's ranks on the publishing node (0 = none matched; informational, never part of scoreDelta)
	CollectiveMaxSkewMs float64 `json:"collectiveMaxSkewMs,omitempty"`

	// GPUIdleDuringCommRatio is the fraction of NCCL communication time (covered by DCGM samples)
	// during which SM_ACTIVE (else GPU_UTIL) was below the idle threshold. Set only when the
	// collector runs with -dcgm-correlate and measured it; informational, never part of scoreDelta
	GPUIdleDuringCommRatio float64 `json:"gpuIdleDuringCommRatio,omitempty"`

	// SMActiveDuringCompute is the mean DCGM SM activity (0-1) outside the NCCL call windows,
	// between the job's first and last observed collective (set only when measured)
	SMActiveDuringCompute float64 `json:"smActiveDuringCompute,omitempty"`

	// GPUCorrelationCoverage is the fraction of NCCL communication time covered by a DCGM sample;
	// a low value means the dcgm-exporter collect interval is too coarse for the ratios above
	GPUCorrelationCoverage float64 `json:"gpuCorrelationCoverage,omitempty"`

	// ScoreDelta is the penalty in [0,1] for the topology scorer (0 = healthy fabric)
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=1
	ScoreDelta float64 `json:"scoreDelta,omitempty"`

	// UpdatedAt is the time the status was last refreshed from collector signals
	UpdatedAt *metav1.Time `json:"updatedAt,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Namespaced,shortName=gfs
//+kubebuilder:printcolumn:name="Job",type=string,JSONPath=`.spec.jobRef`
//+kubebuilder:printcolumn:name="Straggler",type=integer,JSONPath=`.status.stragglerRank`
//+kubebuilder:printcolumn:name="NCCL p99 ms",type=number,JSONPath=`.status.ncclP99ms`
//+kubebuilder:printcolumn:name="RDMA retry",type=number,JSONPath=`.status.rdmaRetryRate`
//+kubebuilder:printcolumn:name="GDS hit",type=number,JSONPath=`.status.gdsHitRatio`

// GryviaFabricSignal is the Schema for the gryviafabricsignals API
type GryviaFabricSignal struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GryviaFabricSignalSpec   `json:"spec,omitempty"`
	Status GryviaFabricSignalStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// GryviaFabricSignalList contains a list of GryviaFabricSignal
type GryviaFabricSignalList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GryviaFabricSignal `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GryviaFabricSignal{}, &GryviaFabricSignalList{})
}
