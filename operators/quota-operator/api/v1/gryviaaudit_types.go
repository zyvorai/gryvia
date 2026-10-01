package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GryviaAuditSpec defines the desired state of GryviaAudit
type GryviaAuditSpec struct {
	// Scope defines what this audit covers
	Scope AuditScope `json:"scope,omitempty"`

	// Retention defines how long to keep audit records
	Retention AuditRetention `json:"retention,omitempty"`

	// Events defines which events to audit
	Events AuditEvents `json:"events,omitempty"`

	// Compliance defines compliance framework requirements
	Compliance AuditCompliance `json:"compliance,omitempty"`

	// Reporting defines automated report generation
	Reporting AuditReporting `json:"reporting,omitempty"`

	// AnomalyDetection defines anomaly detection rules
	AnomalyDetection *AuditAnomalyDetection `json:"anomalyDetection,omitempty"`
}

// AuditScope defines the scope of the audit
type AuditScope struct {
	// Type is the scope type (cluster, namespace, team, user, job)
	Type string `json:"type,omitempty"`

	// Name is the name of the scoped resource
	Name string `json:"name,omitempty"`
}

// AuditRetention defines retention policy
type AuditRetention struct {
	// Duration is how long to retain audit logs (e.g., 1y, 7y)
	Duration string `json:"duration,omitempty"`

	// Archival defines archival configuration
	Archival *AuditArchival `json:"archival,omitempty"`
}

// AuditArchival defines archival configuration
type AuditArchival struct {
	// Enabled enables archival
	Enabled bool `json:"enabled,omitempty"`

	// Destination is the archive storage type (s3, gcs, azure-blob, glacier)
	Destination string `json:"destination,omitempty"`

	// Bucket is the storage bucket
	Bucket string `json:"bucket,omitempty"`

	// CompressionEnabled enables compression
	CompressionEnabled bool `json:"compressionEnabled,omitempty"`
}

// AuditEvents defines which events to audit
type AuditEvents struct {
	JobCreated           bool `json:"jobCreated,omitempty"`
	JobModified          bool `json:"jobModified,omitempty"`
	JobDeleted           bool `json:"jobDeleted,omitempty"`
	ResourceAccessDenied bool `json:"resourceAccessDenied,omitempty"`
	QuotaExceeded        bool `json:"quotaExceeded,omitempty"`
	BudgetExceeded       bool `json:"budgetExceeded,omitempty"`
	SensitiveDataAccess  bool `json:"sensitiveDataAccess,omitempty"`
	ConfigurationChanges bool `json:"configurationChanges,omitempty"`
	UserAuthentication   bool `json:"userAuthentication,omitempty"`
	RoleChanges          bool `json:"roleChanges,omitempty"`
}

// AuditCompliance defines compliance configuration
type AuditCompliance struct {
	// Frameworks is the list of compliance frameworks (SOC2, HIPAA, GDPR, etc.)
	Frameworks []string `json:"frameworks,omitempty"`

	// DataClassification is the data classification level
	DataClassification string `json:"dataClassification,omitempty"`

	// ApprovalRequired defines what needs approval
	ApprovalRequired *AuditApprovalRequired `json:"approvalRequired,omitempty"`
}

// AuditApprovalRequired defines approval requirements
type AuditApprovalRequired struct {
	SensitiveJobs         bool `json:"sensitiveJobs,omitempty"`
	HighCostJobs          bool `json:"highCostJobs,omitempty"`
	ProductionDeployments bool `json:"productionDeployments,omitempty"`
}

// AuditReporting defines report generation
type AuditReporting struct {
	// Enabled enables report generation
	Enabled bool `json:"enabled,omitempty"`

	// Schedule is the cron schedule for reports
	Schedule string `json:"schedule,omitempty"`

	// Recipients is the list of report recipients
	Recipients []string `json:"recipients,omitempty"`

	// Format is the report format (pdf, csv, json, html)
	Format string `json:"format,omitempty"`
}

// AuditAnomalyDetection defines anomaly detection
type AuditAnomalyDetection struct {
	// Enabled enables anomaly detection
	Enabled bool `json:"enabled,omitempty"`

	// Rules defines anomaly detection rules
	Rules []AnomalyRule `json:"rules,omitempty"`
}

// AnomalyRule defines an anomaly detection rule
type AnomalyRule struct {
	// Name is the rule name
	Name string `json:"name,omitempty"`

	// Condition is the rule condition expression
	Condition string `json:"condition,omitempty"`

	// Severity is the rule severity (low, medium, high, critical)
	Severity string `json:"severity,omitempty"`

	// Action is the action to take (log, alert, block)
	Action string `json:"action,omitempty"`
}

// GryviaAuditStatus defines the observed state of GryviaAudit
type GryviaAuditStatus struct {
	// TotalEvents is the total number of audit events captured
	TotalEvents int `json:"totalEvents,omitempty"`

	// LastAuditTime is the timestamp of the last audit event
	LastAuditTime *metav1.Time `json:"lastAuditTime,omitempty"`

	// ComplianceScore is the overall compliance score 0-100
	ComplianceScore float64 `json:"complianceScore,omitempty"`

	// Violations is the list of recent compliance violations
	Violations []AuditViolation `json:"violations,omitempty"`

	// LastReport tracks the last generated report
	LastReport *AuditReportRef `json:"lastReport,omitempty"`

	// AuditEntries is a rolling buffer of recent audit entries
	AuditEntries []AuditEntry `json:"auditEntries,omitempty"`

	// Conditions represent the latest available observations
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// AuditViolation represents a compliance violation
type AuditViolation struct {
	// Timestamp of the violation
	Timestamp metav1.Time `json:"timestamp,omitempty"`

	// Type is the violation type
	Type string `json:"type,omitempty"`

	// Severity is the violation severity
	Severity string `json:"severity,omitempty"`

	// Description of the violation
	Description string `json:"description,omitempty"`

	// Remediated indicates if the violation has been fixed
	Remediated bool `json:"remediated,omitempty"`
}

// AuditReportRef references a generated report
type AuditReportRef struct {
	// GeneratedAt is when the report was generated
	GeneratedAt *metav1.Time `json:"generatedAt,omitempty"`

	// Location is where the report is stored
	Location string `json:"location,omitempty"`
}

// AuditEntry represents a single audit log entry
type AuditEntry struct {
	// Timestamp of the event
	Timestamp metav1.Time `json:"timestamp"`

	// User who performed the action
	User string `json:"user,omitempty"`

	// Action performed (create, update, delete, access)
	Action string `json:"action"`

	// ResourceType is the type of resource affected
	ResourceType string `json:"resourceType"`

	// ResourceName is the name of the resource affected
	ResourceName string `json:"resourceName"`

	// Namespace of the resource
	Namespace string `json:"namespace,omitempty"`

	// Result of the action (success, denied, error)
	Result string `json:"result"`

	// Details provides additional context
	Details string `json:"details,omitempty"`
}

//+kubebuilder:deprecatedversion:warning="no controller reconciles this kind and its spec is not executed; it is kept readable for migration and will be removed in a future release"
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster

// GryviaAudit is the Schema for the gryviaaudits API
type GryviaAudit struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GryviaAuditSpec   `json:"spec,omitempty"`
	Status GryviaAuditStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// GryviaAuditList contains a list of GryviaAudit
type GryviaAuditList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GryviaAudit `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GryviaAudit{}, &GryviaAuditList{})
}
