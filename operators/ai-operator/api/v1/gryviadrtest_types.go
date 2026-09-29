package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FabricDRTestSpec defines the desired state of FabricDRTest
type FabricDRTestSpec struct {
	// Type is the DR test type (backup-restore, failover, data-integrity, rpo-rto, full-drill, chaos-engineering)
	Type string `json:"type"`

	// Scope defines what to test
	Scope DRTestScope `json:"scope,omitempty"`

	// BackupRestore defines backup/restore test config
	BackupRestore *DRBackupRestoreSpec `json:"backupRestore,omitempty"`

	// Failover defines failover test config
	Failover *DRFailoverSpec `json:"failover,omitempty"`

	// DataIntegrity defines data integrity test config
	DataIntegrity *DRDataIntegritySpec `json:"dataIntegrity,omitempty"`

	// RpoRto defines RPO/RTO test config
	RpoRto *DRRpoRtoSpec `json:"rpoRto,omitempty"`

	// FullDrill defines full DR drill config
	FullDrill *DRFullDrillSpec `json:"fullDrill,omitempty"`

	// ChaosEngineering defines chaos test config
	ChaosEngineering *DRChaosSpec `json:"chaosEngineering,omitempty"`

	// Schedule defines test scheduling
	Schedule *DRTestSchedule `json:"schedule,omitempty"`

	// Notifications defines notification settings
	Notifications *DRTestNotifications `json:"notifications,omitempty"`

	// ApprovalRequired requires approval before running
	ApprovalRequired bool `json:"approvalRequired,omitempty"`
}

// DRTestScope defines test scope
type DRTestScope struct {
	// Components to test (control-plane, data-plane, jobs, storage, network, monitoring, all)
	Components []string `json:"components,omitempty"`

	// Environment to test in (production, staging, test)
	Environment string `json:"environment,omitempty"`
}

// DRBackupRestoreSpec defines backup/restore test
type DRBackupRestoreSpec struct {
	// Resources to backup/restore (crds, jobs, config, secrets, persistent-volumes, monitoring)
	Resources []string `json:"resources,omitempty"`

	// BackupLocation defines where backups are stored
	BackupLocation *DRBackupLocation `json:"backupLocation,omitempty"`

	// RestorePoint is the timestamp or backup ID to restore from
	RestorePoint string `json:"restorePoint,omitempty"`

	// Validation defines what to validate after restore
	Validation *DRValidation `json:"validation,omitempty"`
}

// DRBackupLocation defines backup storage location
type DRBackupLocation struct {
	// Type is the storage type (s3, gcs, azure-blob, nfs)
	Type string `json:"type,omitempty"`

	// Path is the backup path
	Path string `json:"path,omitempty"`

	// Credentials for accessing the storage
	Credentials *DRCredentials `json:"credentials,omitempty"`
}

// DRCredentials defines credentials reference
type DRCredentials struct {
	// SecretRef is the name of the Kubernetes secret
	SecretRef string `json:"secretRef,omitempty"`
}

// DRValidation defines validation checks
type DRValidation struct {
	// VerifyJobsRecovered verifies jobs are recovered
	VerifyJobsRecovered bool `json:"verifyJobsRecovered,omitempty"`

	// VerifyDataIntegrity verifies data integrity
	VerifyDataIntegrity bool `json:"verifyDataIntegrity,omitempty"`

	// VerifyCheckpoints verifies checkpoint restoration
	VerifyCheckpoints bool `json:"verifyCheckpoints,omitempty"`
}

// DRFailoverSpec defines failover test
type DRFailoverSpec struct {
	// Scenario is the failover scenario (node-failure, az-failure, region-failure, control-plane-failure)
	Scenario string `json:"scenario,omitempty"`

	// Source cluster
	Source *DRClusterRef `json:"source,omitempty"`

	// Target cluster
	Target *DRClusterRef `json:"target,omitempty"`

	// Automatic enables automatic failover
	Automatic bool `json:"automatic,omitempty"`

	// Jobs defines which jobs to failover
	Jobs *DRFailoverJobs `json:"jobs,omitempty"`
}

// DRClusterRef references a cluster
type DRClusterRef struct {
	// Cluster name
	Cluster string `json:"cluster,omitempty"`

	// Region of the cluster
	Region string `json:"region,omitempty"`
}

// DRFailoverJobs defines jobs to failover
type DRFailoverJobs struct {
	// IncludeAll includes all jobs
	IncludeAll bool `json:"includeAll,omitempty"`

	// Priorities limits failover to specific priorities
	Priorities []string `json:"priorities,omitempty"`
}

// DRDataIntegritySpec defines data integrity test
type DRDataIntegritySpec struct {
	// StorageSystems to test
	StorageSystems []string `json:"storageSystems,omitempty"`

	// VerifyChecksums enables checksum verification
	VerifyChecksums bool `json:"verifyChecksums,omitempty"`

	// CompareReplicas compares source and replica data
	CompareReplicas bool `json:"compareReplicas,omitempty"`
}

// DRRpoRtoSpec defines RPO/RTO test
type DRRpoRtoSpec struct {
	// Targets defines RPO/RTO targets
	Targets *DRRpoRtoTargets `json:"targets,omitempty"`

	// Scenarios to test
	Scenarios []DRScenario `json:"scenarios,omitempty"`
}

// DRRpoRtoTargets defines RPO/RTO targets
type DRRpoRtoTargets struct {
	// RpoMinutes is the Recovery Point Objective in minutes
	RpoMinutes int `json:"rpoMinutes,omitempty"`

	// RtoMinutes is the Recovery Time Objective in minutes
	RtoMinutes int `json:"rtoMinutes,omitempty"`
}

// DRScenario defines a test scenario
type DRScenario struct {
	// Name of the scenario
	Name string `json:"name,omitempty"`

	// Description of the scenario
	Description string `json:"description,omitempty"`

	// FailureType simulated
	FailureType string `json:"failureType,omitempty"`
}

// DRFullDrillSpec defines full DR drill
type DRFullDrillSpec struct {
	// Scenario description
	Scenario string `json:"scenario,omitempty"`

	// Steps to execute
	Steps []DRDrillStep `json:"steps,omitempty"`

	// SuccessCriteria defines criteria for success
	SuccessCriteria []DRSuccessCriterion `json:"successCriteria,omitempty"`

	// RollbackEnabled enables rollback
	RollbackEnabled bool `json:"rollbackEnabled,omitempty"`
}

// DRDrillStep defines a drill step
type DRDrillStep struct {
	// Name of the step
	Name string `json:"name,omitempty"`

	// Action to perform
	Action string `json:"action,omitempty"`

	// Timeout for the step
	Timeout string `json:"timeout,omitempty"`

	// Validation to check
	Validation string `json:"validation,omitempty"`
}

// DRSuccessCriterion defines a success criterion
type DRSuccessCriterion struct {
	// Metric to evaluate
	Metric string `json:"metric,omitempty"`

	// Operator for comparison (lt, lte, gt, gte, eq)
	Operator string `json:"operator,omitempty"`

	// Value to compare against
	Value float64 `json:"value,omitempty"`
}

// DRChaosSpec defines chaos engineering test
type DRChaosSpec struct {
	// Experiments to run
	Experiments []DRChaosExperiment `json:"experiments,omitempty"`

	// BlastRadius controls the impact scope (minimal, limited, moderate, extensive)
	BlastRadius string `json:"blastRadius,omitempty"`
}

// DRChaosExperiment defines a chaos experiment
type DRChaosExperiment struct {
	// Type of experiment (pod-kill, node-drain, network-partition, cpu-stress, memory-stress, disk-fill)
	Type string `json:"type,omitempty"`

	// Target for the experiment
	Target *DRChaosTarget `json:"target,omitempty"`

	// Duration of the experiment
	Duration string `json:"duration,omitempty"`

	// Parameters for the experiment
	Parameters map[string]string `json:"parameters,omitempty"`
}

// DRChaosTarget defines a chaos target
type DRChaosTarget struct {
	// Selector to identify targets
	Selector map[string]string `json:"selector,omitempty"`
}

// DRTestSchedule defines test scheduling
type DRTestSchedule struct {
	// Type is once or recurring
	Type string `json:"type,omitempty"`

	// Cron expression for recurring tests
	Cron string `json:"cron,omitempty"`

	// NextRun is the next scheduled run
	NextRun *metav1.Time `json:"nextRun,omitempty"`
}

// DRTestNotifications defines notifications
type DRTestNotifications struct {
	// Enabled enables notifications
	Enabled bool `json:"enabled,omitempty"`

	// Events to notify on (start, completion, failure, metrics-exceeded)
	Events []string `json:"events,omitempty"`

	// Recipients for notifications
	Recipients []string `json:"recipients,omitempty"`

	// Channels for notifications (email, slack, pagerduty, webhook)
	Channels []string `json:"channels,omitempty"`
}

// FabricDRTestStatus defines the observed state of FabricDRTest
type FabricDRTestStatus struct {
	// State of the test (pending-approval, approved, running, completed, failed, cancelled)
	State string `json:"state,omitempty"`

	// Execution details
	Execution *DRTestExecution `json:"execution,omitempty"`

	// Results of the test
	Results *DRTestResults `json:"results,omitempty"`

	// Report tracks generated report
	Report *DRTestReport `json:"report,omitempty"`

	// Conditions represent the latest available observations
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// DRTestExecution tracks execution details
type DRTestExecution struct {
	// StartTime of the test
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// EndTime of the test
	EndTime *metav1.Time `json:"endTime,omitempty"`

	// Duration of the test
	Duration string `json:"duration,omitempty"`
}

// DRTestResults holds test results
type DRTestResults struct {
	// Passed indicates if the test passed
	Passed bool `json:"passed,omitempty"`

	// Metrics holds measured metrics
	Metrics *DRTestMetrics `json:"metrics,omitempty"`

	// Details holds per-step results
	Details []DRTestStepResult `json:"details,omitempty"`

	// Issues found during the test
	Issues []DRTestIssue `json:"issues,omitempty"`
}

// DRTestMetrics holds test metrics
type DRTestMetrics struct {
	// RpoAchieved is the actual RPO
	RpoAchieved string `json:"rpoAchieved,omitempty"`

	// RtoAchieved is the actual RTO
	RtoAchieved string `json:"rtoAchieved,omitempty"`

	// DataIntegrity percentage
	DataIntegrity string `json:"dataIntegrity,omitempty"`

	// JobsRecovered count
	JobsRecovered int `json:"jobsRecovered,omitempty"`

	// JobsFailed count
	JobsFailed int `json:"jobsFailed,omitempty"`
}

// DRTestStepResult tracks per-step results
type DRTestStepResult struct {
	// Step name
	Step string `json:"step,omitempty"`

	// Status of the step
	Status string `json:"status,omitempty"`

	// Duration of the step
	Duration string `json:"duration,omitempty"`

	// Message describing the result
	Message string `json:"message,omitempty"`
}

// DRTestIssue records an issue found during testing
type DRTestIssue struct {
	// Severity of the issue (critical, high, medium, low)
	Severity string `json:"severity,omitempty"`

	// Description of the issue
	Description string `json:"description,omitempty"`

	// Recommendation to fix the issue
	Recommendation string `json:"recommendation,omitempty"`
}

// DRTestReport tracks generated report
type DRTestReport struct {
	// Generated indicates if a report was generated
	Generated bool `json:"generated,omitempty"`

	// Path where the report is stored
	Path string `json:"path,omitempty"`

	// Format of the report
	Format string `json:"format,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster

// FabricDRTest is the Schema for the fabricdrtests API
type FabricDRTest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FabricDRTestSpec   `json:"spec,omitempty"`
	Status FabricDRTestStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// FabricDRTestList contains a list of FabricDRTest
type FabricDRTestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FabricDRTest `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FabricDRTest{}, &FabricDRTestList{})
}
