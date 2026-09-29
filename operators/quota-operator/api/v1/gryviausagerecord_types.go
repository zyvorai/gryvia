package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GryviaUsageRecordSpec is the metered usage of one GryviaAIJob. The quota operator
// owns and rewrites it until final is true. Values are estimates from job wall-clock
// time (start to end); they are not invoices.
type GryviaUsageRecordSpec struct {
	// Tenant the usage is attributed to
	Tenant string `json:"tenant"`

	// Job is the GryviaAIJob name
	Job string `json:"job"`

	// JobUID is the GryviaAIJob UID (the record is named usage-<jobUID>)
	JobUID string `json:"jobUID"`

	// GpuType of the job
	GpuType string `json:"gpuType,omitempty"`

	// Sku is the name of the matching GryviaGpuSku, empty when the default price table was used
	Sku string `json:"sku,omitempty"`

	// Gpus is the number of GPUs the job holds
	Gpus int32 `json:"gpus"`

	// Start is when the job started running
	Start metav1.Time `json:"start"`

	// End is when the job finished; nil while it is running
	End *metav1.Time `json:"end,omitempty"`

	// GpuHours is wall-clock hours multiplied by gpus
	GpuHours float64 `json:"gpuHours"`

	// Rate is the price per GPU-hour used for cost
	Rate float64 `json:"rate"`

	// Cost is gpuHours multiplied by rate
	Cost float64 `json:"cost"`

	// Currency of rate and cost
	Currency string `json:"currency,omitempty"`

	// Final is true once the job finished; a final record is never modified again
	Final bool `json:"final"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:scope=Namespaced
//+kubebuilder:printcolumn:name="Tenant",type=string,JSONPath=`.spec.tenant`
//+kubebuilder:printcolumn:name="Job",type=string,JSONPath=`.spec.job`
//+kubebuilder:printcolumn:name="GPU-Hours",type=number,JSONPath=`.spec.gpuHours`
//+kubebuilder:printcolumn:name="Cost",type=number,JSONPath=`.spec.cost`
//+kubebuilder:printcolumn:name="Final",type=boolean,JSONPath=`.spec.final`

// GryviaUsageRecord is the Schema for the gryviausagerecords API
type GryviaUsageRecord struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec GryviaUsageRecordSpec `json:"spec,omitempty"`
}

//+kubebuilder:object:root=true

// GryviaUsageRecordList contains a list of GryviaUsageRecord
type GryviaUsageRecordList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GryviaUsageRecord `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GryviaUsageRecord{}, &GryviaUsageRecordList{})
}
