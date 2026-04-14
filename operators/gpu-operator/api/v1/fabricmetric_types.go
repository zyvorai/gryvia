package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FabricMetricSpec defines the desired state of FabricMetric
type FabricMetricSpec struct {
	// Name is the metric name
	Name string `json:"name"`

	// Description is a human-readable description
	Description string `json:"description,omitempty"`

	// Type is the metric type (gauge, counter, histogram, summary)
	Type string `json:"type,omitempty"`

	// Unit is the unit of measurement
	Unit string `json:"unit,omitempty"`

	// Source defines how to collect this metric
	Source MetricSource `json:"source"`

	// Thresholds define warning and critical thresholds
	Thresholds *MetricThresholds `json:"thresholds,omitempty"`

	// Visualization defines how to display the metric
	Visualization *MetricVisualization `json:"visualization,omitempty"`

	// Retention defines how long to keep metric data
	Retention string `json:"retention,omitempty"`

	// Labels are grouping labels
	Labels map[string]string `json:"labels,omitempty"`
}

// MetricSource defines how to collect a metric
type MetricSource struct {
	// Type is the source type (prometheus, job-output, webhook, script, log-parser)
	Type string `json:"type"`

	// Prometheus defines a Prometheus query source
	Prometheus *PrometheusSource `json:"prometheus,omitempty"`

	// JobOutput defines a job output parsing source
	JobOutput *JobOutputSource `json:"jobOutput,omitempty"`

	// Webhook defines a webhook callback source
	Webhook *WebhookSource `json:"webhook,omitempty"`

	// Script defines a custom script source
	Script *ScriptSource `json:"script,omitempty"`
}

// PrometheusSource defines a Prometheus query
type PrometheusSource struct {
	// Query is the PromQL query
	Query string `json:"query"`

	// Interval is the query interval
	Interval string `json:"interval,omitempty"`
}

// JobOutputSource defines job output parsing
type JobOutputSource struct {
	// Pattern is a regex to extract the metric
	Pattern string `json:"pattern,omitempty"`

	// File is the file to parse
	File string `json:"file,omitempty"`

	// JSONPath is the JSON path to extract
	JSONPath string `json:"jsonPath,omitempty"`
}

// WebhookSource defines a webhook callback
type WebhookSource struct {
	// URL to call
	URL string `json:"url"`

	// Interval is the polling interval
	Interval string `json:"interval,omitempty"`
}

// ScriptSource defines a custom script for metric collection
type ScriptSource struct {
	// Command to execute
	Command []string `json:"command"`

	// Interval is the execution interval
	Interval string `json:"interval,omitempty"`
}

// MetricThresholds defines warning and critical thresholds
type MetricThresholds struct {
	// Warning defines warning thresholds
	Warning *ThresholdRange `json:"warning,omitempty"`

	// Critical defines critical thresholds
	Critical *ThresholdRange `json:"critical,omitempty"`

	// Actions are actions to take when thresholds are breached
	Actions []string `json:"actions,omitempty"`
}

// ThresholdRange defines a min/max threshold range
type ThresholdRange struct {
	// Min is the minimum acceptable value
	Min *float64 `json:"min,omitempty"`

	// Max is the maximum acceptable value
	Max *float64 `json:"max,omitempty"`
}

// MetricVisualization defines display options
type MetricVisualization struct {
	// Dashboard is the Grafana dashboard name
	Dashboard string `json:"dashboard,omitempty"`

	// Panel is the Grafana panel name
	Panel string `json:"panel,omitempty"`

	// Format is the chart format (line, bar, gauge, table)
	Format string `json:"format,omitempty"`
}

// MetricHistoryEntry holds a single metric data point
type MetricHistoryEntry struct {
	// Timestamp of the measurement
	Timestamp *metav1.Time `json:"timestamp,omitempty"`

	// Value of the measurement
	Value float64 `json:"value"`

	// Job that produced this measurement
	Job string `json:"job,omitempty"`
}

// FabricMetricStatus defines the observed state of FabricMetric
type FabricMetricStatus struct {
	// CurrentValue is the latest metric value
	CurrentValue float64 `json:"currentValue,omitempty"`

	// LastUpdate is the time of the last metric update
	LastUpdate *metav1.Time `json:"lastUpdate,omitempty"`

	// State is the current threshold state (normal, warning, critical)
	State string `json:"state,omitempty"`

	// History holds recent metric data points
	History []MetricHistoryEntry `json:"history,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster
//+kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.type`
//+kubebuilder:printcolumn:name="Current-Value",type=string,JSONPath=`.status.currentValue`
//+kubebuilder:printcolumn:name="Unit",type=string,JSONPath=`.spec.unit`
//+kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// FabricMetric is the Schema for the fabricmetrics API
type FabricMetric struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FabricMetricSpec   `json:"spec,omitempty"`
	Status FabricMetricStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// FabricMetricList contains a list of FabricMetric
type FabricMetricList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FabricMetric `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FabricMetric{}, &FabricMetricList{})
}
