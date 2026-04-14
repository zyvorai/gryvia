package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FabricBenchmarkSpec defines the desired state of FabricBenchmark
type FabricBenchmarkSpec struct {
	// Type is the benchmark type (mlperf, nccl, gpu-memory, io-throughput, custom)
	Type string `json:"type"`

	// Target defines the resources to benchmark
	Target BenchmarkTarget `json:"target"`

	// MLPerf defines MLPerf benchmark configuration
	MLPerf *MLPerfConfig `json:"mlperf,omitempty"`

	// NCCL defines NCCL benchmark configuration
	NCCL *NCCLConfig `json:"nccl,omitempty"`

	// GPUMemory defines GPU memory benchmark configuration
	GPUMemory *GPUMemoryConfig `json:"gpuMemory,omitempty"`

	// IOThroughput defines I/O benchmark configuration
	IOThroughput *IOThroughputConfig `json:"ioThroughput,omitempty"`

	// Custom defines custom benchmark configuration
	Custom *CustomBenchmarkConfig `json:"custom,omitempty"`

	// Baseline defines comparison baseline configuration
	Baseline *BenchmarkBaseline `json:"baseline,omitempty"`

	// Schedule defines benchmark scheduling
	Schedule *BenchmarkSchedule `json:"schedule,omitempty"`
}

// BenchmarkTarget defines resources to benchmark
type BenchmarkTarget struct {
	// GpuType is the GPU model to benchmark
	GpuType string `json:"gpuType,omitempty"`

	// GpuCount is the number of GPUs to use
	GpuCount int32 `json:"gpuCount,omitempty"`

	// NodeSelector selects specific nodes
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// NetworkType is the network type (infiniband, roce, ethernet)
	NetworkType string `json:"networkType,omitempty"`
}

// MLPerfConfig defines MLPerf benchmark parameters
type MLPerfConfig struct {
	// Workload is the MLPerf workload
	Workload string `json:"workload,omitempty"`

	// BatchSize is the batch size
	BatchSize int32 `json:"batchSize,omitempty"`

	// Precision is the numeric precision
	Precision string `json:"precision,omitempty"`

	// TargetMetric is the metric to optimize for
	TargetMetric string `json:"targetMetric,omitempty"`
}

// NCCLConfig defines NCCL benchmark parameters
type NCCLConfig struct {
	// Operations are the NCCL operations to test
	Operations []string `json:"operations,omitempty"`

	// MessageSizes are the message sizes in MB
	MessageSizes []int32 `json:"messageSizes,omitempty"`

	// Iterations is the number of iterations
	Iterations int32 `json:"iterations,omitempty"`

	// WarmupIters is the number of warmup iterations
	WarmupIters int32 `json:"warmupIters,omitempty"`
}

// GPUMemoryConfig defines GPU memory benchmark parameters
type GPUMemoryConfig struct {
	// TestType is the test type (bandwidth, latency, both)
	TestType string `json:"testType,omitempty"`

	// Sizes are the transfer sizes
	Sizes []string `json:"sizes,omitempty"`

	// Directions are the transfer directions (h2d, d2h, d2d)
	Directions []string `json:"directions,omitempty"`
}

// IOThroughputConfig defines I/O benchmark parameters
type IOThroughputConfig struct {
	// StorageClass is the storage backend
	StorageClass string `json:"storageClass,omitempty"`

	// FileSize is the test file size
	FileSize string `json:"fileSize,omitempty"`

	// BlockSize is the I/O block size
	BlockSize string `json:"blockSize,omitempty"`

	// Operations are the I/O operations to test
	Operations []string `json:"operations,omitempty"`

	// Threads is the number of I/O threads
	Threads int32 `json:"threads,omitempty"`
}

// CustomBenchmarkConfig defines custom benchmark parameters
type CustomBenchmarkConfig struct {
	// Image is the container image
	Image string `json:"image"`

	// Command to execute
	Command []string `json:"command,omitempty"`

	// Args are command arguments
	Args []string `json:"args,omitempty"`

	// Metrics are expected output metrics
	Metrics []CustomMetricDef `json:"metrics,omitempty"`
}

// CustomMetricDef defines a custom metric definition
type CustomMetricDef struct {
	// Name of the metric
	Name string `json:"name"`

	// Unit of measurement
	Unit string `json:"unit,omitempty"`

	// HigherIsBetter indicates if higher values are better
	HigherIsBetter bool `json:"higherIsBetter,omitempty"`
}

// BenchmarkBaseline defines comparison baseline
type BenchmarkBaseline struct {
	// Enabled enables baseline comparison
	Enabled bool `json:"enabled,omitempty"`

	// BenchmarkRef references a previous benchmark
	BenchmarkRef string `json:"benchmarkRef,omitempty"`

	// Values are explicit baseline values
	Values map[string]float64 `json:"values,omitempty"`

	// RegressionThreshold is the percentage drop that triggers regression
	RegressionThreshold float64 `json:"regressionThreshold,omitempty"`
}

// BenchmarkSchedule defines benchmark scheduling
type BenchmarkSchedule struct {
	// Type is once or recurring
	Type string `json:"type,omitempty"`

	// Cron is the schedule expression for recurring benchmarks
	Cron string `json:"cron,omitempty"`

	// Suspend pauses scheduling
	Suspend bool `json:"suspend,omitempty"`
}

// BenchmarkResultValue holds a single result metric
type BenchmarkResultValue struct {
	// Value is the metric value
	Value float64 `json:"value"`

	// Unit is the measurement unit
	Unit string `json:"unit,omitempty"`

	// Percentile95 is the 95th percentile
	Percentile95 float64 `json:"percentile95,omitempty"`

	// Percentile99 is the 99th percentile
	Percentile99 float64 `json:"percentile99,omitempty"`
}

// BenchmarkComparison holds comparison results
type BenchmarkComparison struct {
	// Improved is the number of improved metrics
	Improved int32 `json:"improved,omitempty"`

	// Regressed is the number of regressed metrics
	Regressed int32 `json:"regressed,omitempty"`

	// Stable is the number of stable metrics
	Stable int32 `json:"stable,omitempty"`
}

// BenchmarkHardwareInfo holds hardware details
type BenchmarkHardwareInfo struct {
	// GpuModel is the GPU model name
	GpuModel string `json:"gpuModel,omitempty"`

	// GpuCount is the number of GPUs used
	GpuCount int32 `json:"gpuCount,omitempty"`

	// DriverVersion is the NVIDIA driver version
	DriverVersion string `json:"driverVersion,omitempty"`

	// CudaVersion is the CUDA version
	CudaVersion string `json:"cudaVersion,omitempty"`
}

// FabricBenchmarkStatus defines the observed state of FabricBenchmark
type FabricBenchmarkStatus struct {
	// State is the current state (pending, running, completed, failed, regressed)
	State string `json:"state,omitempty"`

	// StartTime is when the benchmark started
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is when the benchmark completed
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Duration is the human-readable duration
	Duration string `json:"duration,omitempty"`

	// Results holds benchmark result metrics
	Results map[string]BenchmarkResultValue `json:"results,omitempty"`

	// Comparison holds comparison with baseline
	Comparison *BenchmarkComparison `json:"comparison,omitempty"`

	// Hardware holds hardware information
	Hardware *BenchmarkHardwareInfo `json:"hardware,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Namespaced
//+kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.type`
//+kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`
//+kubebuilder:printcolumn:name="GPUs",type=integer,JSONPath=`.spec.target.gpuCount`
//+kubebuilder:printcolumn:name="Duration",type=string,JSONPath=`.status.duration`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// FabricBenchmark is the Schema for the fabricbenchmarks API
type FabricBenchmark struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FabricBenchmarkSpec   `json:"spec,omitempty"`
	Status FabricBenchmarkStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// FabricBenchmarkList contains a list of FabricBenchmark
type FabricBenchmarkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FabricBenchmark `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FabricBenchmark{}, &FabricBenchmarkList{})
}
