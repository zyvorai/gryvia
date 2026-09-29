package controllers

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gryviav1 "github.com/zyvorai/gryvia/operators/quota-operator/api/v1"
)

const (
	maxAuditEntries = 100
)

// GryviaAuditReconciler reconciles a GryviaAudit object
type GryviaAuditReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=gryvia.io,resources=gryviaaudits,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gryvia.io,resources=gryviaaudits/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gryvia.io,resources=gryviaaudits/finalizers,verbs=update
//+kubebuilder:rbac:groups=gryvia.io,resources=gryviaaijobs,verbs=get;list;watch
//+kubebuilder:rbac:groups=gryvia.io,resources=gryviaquotas,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *GryviaAuditReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the GryviaAudit instance
	audit := &gryviav1.GryviaAudit{}
	err := r.Get(ctx, req.NamespacedName, audit)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("GryviaAudit resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get GryviaAudit")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !audit.ObjectMeta.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	logger.Info("Reconciling GryviaAudit", "scope", audit.Spec.Scope.Type, "name", audit.Spec.Scope.Name)

	// Reconcile the audit
	result, err := r.reconcileAudit(ctx, audit)
	if err != nil {
		logger.Error(err, "Failed to reconcile audit")
		r.updateCondition(audit, "Ready", metav1.ConditionFalse, "ReconcileFailed", err.Error())
		if statusErr := r.Status().Update(ctx, audit); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after reconcile failure")
		}
		return result, err
	}

	// Update status
	if err := r.Status().Update(ctx, audit); err != nil {
		logger.Error(err, "Failed to update GryviaAudit status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: 2 * time.Minute}, nil
}

func (r *GryviaAuditReconciler) reconcileAudit(ctx context.Context, audit *gryviav1.GryviaAudit) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Capture GPU resource events by watching GryviaAIJob resources
	if audit.Spec.Events.JobCreated || audit.Spec.Events.JobModified || audit.Spec.Events.JobDeleted {
		if err := r.captureJobEvents(ctx, audit); err != nil {
			logger.Error(err, "Failed to capture job events")
		}
	}

	// Capture quota events
	if audit.Spec.Events.QuotaExceeded || audit.Spec.Events.BudgetExceeded {
		if err := r.captureQuotaEvents(ctx, audit); err != nil {
			logger.Error(err, "Failed to capture quota events")
		}
	}

	// Enforce retention policy
	r.enforceRetention(audit)

	// Calculate compliance score
	audit.Status.ComplianceScore = r.calculateComplianceScore(audit)

	now := metav1.Now()
	audit.Status.LastAuditTime = &now

	r.updateCondition(audit, "Ready", metav1.ConditionTrue, "AuditActive", "Audit is actively monitoring events")

	return ctrl.Result{}, nil
}

func (r *GryviaAuditReconciler) captureJobEvents(ctx context.Context, audit *gryviav1.GryviaAudit) error {
	logger := log.FromContext(ctx)

	// List all GryviaAIJob resources based on scope
	jobList := &gryviav1.GryviaAIJobList{}
	listOpts := []client.ListOption{}

	if audit.Spec.Scope.Type == "namespace" && audit.Spec.Scope.Name != "" {
		listOpts = append(listOpts, client.InNamespace(audit.Spec.Scope.Name))
	}

	if err := r.List(ctx, jobList, listOpts...); err != nil {
		return fmt.Errorf("failed to list GryviaAIJobs: %w", err)
	}

	for _, job := range jobList.Items {
		// Skip if this job doesn't match the audit scope
		if !r.matchesScope(audit, &job) {
			continue
		}

		// Create audit entry for running jobs
		if job.Status.Phase == "Running" && audit.Spec.Events.JobCreated {
			entry := gryviav1.AuditEntry{
				Timestamp:    metav1.Now(),
				Action:       "running",
				ResourceType: "GryviaAIJob",
				ResourceName: job.Name,
				Namespace:    job.Namespace,
				Result:       "success",
				Details:      fmt.Sprintf("GPUs: %d, Type: %s", job.Spec.GPUs, job.Spec.GpuType),
			}
			r.addAuditEntry(audit, entry)
		}

		// Track quota exceeded events
		if audit.Spec.Events.QuotaExceeded && job.Status.Phase == "Rejected" {
			entry := gryviav1.AuditEntry{
				Timestamp:    metav1.Now(),
				Action:       "rejected",
				ResourceType: "GryviaAIJob",
				ResourceName: job.Name,
				Namespace:    job.Namespace,
				Result:       "denied",
				Details:      "Job rejected due to quota exceeded",
			}
			r.addAuditEntry(audit, entry)
		}
	}

	audit.Status.TotalEvents = len(audit.Status.AuditEntries)
	logger.Info("Captured job events", "totalEvents", audit.Status.TotalEvents)

	return nil
}

func (r *GryviaAuditReconciler) captureQuotaEvents(ctx context.Context, audit *gryviav1.GryviaAudit) error {
	// List all GryviaQuota resources
	quotaList := &gryviav1.GryviaQuotaList{}
	if err := r.List(ctx, quotaList); err != nil {
		return fmt.Errorf("failed to list GryviaQuotas: %w", err)
	}

	for _, quota := range quotaList.Items {
		// Check for quota exceeded
		if audit.Spec.Events.QuotaExceeded && quota.Status.Phase == "QuotaExceeded" {
			entry := gryviav1.AuditEntry{
				Timestamp:    metav1.Now(),
				Action:       "quota-exceeded",
				ResourceType: "GryviaQuota",
				ResourceName: quota.Name,
				Result:       "warning",
				Details:      fmt.Sprintf("Team %s exceeded GPU quota", quota.Spec.Team),
			}
			r.addAuditEntry(audit, entry)

			// Record as violation
			violation := gryviav1.AuditViolation{
				Timestamp:   metav1.Now(),
				Type:        "quota-exceeded",
				Severity:    "medium",
				Description: fmt.Sprintf("Team %s exceeded GPU quota: %d/%d GPUs", quota.Spec.Team, quota.Status.CurrentUsage.AllocatedGPUs, quota.Spec.GPUQuota.MaxGPUs),
				Remediated:  false,
			}
			r.addViolation(audit, violation)
		}

		// Check for budget exceeded
		if audit.Spec.Events.BudgetExceeded && quota.Status.Phase == "BudgetExceeded" {
			entry := gryviav1.AuditEntry{
				Timestamp:    metav1.Now(),
				Action:       "budget-exceeded",
				ResourceType: "GryviaQuota",
				ResourceName: quota.Name,
				Result:       "warning",
				Details:      fmt.Sprintf("Team %s exceeded budget", quota.Spec.Team),
			}
			r.addAuditEntry(audit, entry)

			violation := gryviav1.AuditViolation{
				Timestamp:   metav1.Now(),
				Type:        "budget-exceeded",
				Severity:    "medium",
				Description: fmt.Sprintf("Team %s exceeded monthly budget", quota.Spec.Team),
				Remediated:  false,
			}
			r.addViolation(audit, violation)
		}
	}

	return nil
}

func (r *GryviaAuditReconciler) matchesScope(audit *gryviav1.GryviaAudit, job *gryviav1.GryviaAIJob) bool {
	switch audit.Spec.Scope.Type {
	case "cluster":
		return true
	case "namespace":
		return job.Namespace == audit.Spec.Scope.Name
	case "team":
		// Check if the job's namespace has the team label
		if teamLabel, ok := job.Labels["gryvia.io/team"]; ok {
			return teamLabel == audit.Spec.Scope.Name
		}
		return false
	default:
		return true
	}
}

func (r *GryviaAuditReconciler) addAuditEntry(audit *gryviav1.GryviaAudit, entry gryviav1.AuditEntry) {
	audit.Status.AuditEntries = append(audit.Status.AuditEntries, entry)

	// Maintain rolling buffer
	if len(audit.Status.AuditEntries) > maxAuditEntries {
		audit.Status.AuditEntries = audit.Status.AuditEntries[len(audit.Status.AuditEntries)-maxAuditEntries:]
	}
}

func (r *GryviaAuditReconciler) addViolation(audit *gryviav1.GryviaAudit, violation gryviav1.AuditViolation) {
	// Only add if not a duplicate of the most recent violation
	if len(audit.Status.Violations) > 0 {
		last := audit.Status.Violations[len(audit.Status.Violations)-1]
		if last.Type == violation.Type && last.Description == violation.Description {
			return
		}
	}

	audit.Status.Violations = append(audit.Status.Violations, violation)

	// Keep max 50 violations
	if len(audit.Status.Violations) > 50 {
		audit.Status.Violations = audit.Status.Violations[len(audit.Status.Violations)-50:]
	}
}

func (r *GryviaAuditReconciler) enforceRetention(audit *gryviav1.GryviaAudit) {
	if audit.Spec.Retention.Duration == "" {
		return
	}

	// Parse retention duration (simplified: support days format like "30d", "365d")
	retentionDays := 365 * 7 // Default 7 years
	var durationVal int
	var unit string
	if _, err := fmt.Sscanf(audit.Spec.Retention.Duration, "%d%s", &durationVal, &unit); err == nil {
		switch unit {
		case "d":
			retentionDays = durationVal
		case "y":
			retentionDays = durationVal * 365
		}
	}

	cutoff := time.Now().Add(-time.Duration(retentionDays) * 24 * time.Hour)

	// Remove old entries
	var retained []gryviav1.AuditEntry
	for _, entry := range audit.Status.AuditEntries {
		if entry.Timestamp.Time.After(cutoff) {
			retained = append(retained, entry)
		}
	}
	audit.Status.AuditEntries = retained

	// Remove old violations
	var retainedViolations []gryviav1.AuditViolation
	for _, v := range audit.Status.Violations {
		if v.Timestamp.Time.After(cutoff) {
			retainedViolations = append(retainedViolations, v)
		}
	}
	audit.Status.Violations = retainedViolations
}

func (r *GryviaAuditReconciler) calculateComplianceScore(audit *gryviav1.GryviaAudit) float64 {
	if len(audit.Spec.Compliance.Frameworks) == 0 {
		return 0
	}

	// Base score starts at 100
	score := 100.0

	// Deduct for unresolved violations
	unresolvedCount := 0
	for _, v := range audit.Status.Violations {
		if !v.Remediated {
			unresolvedCount++
			switch v.Severity {
			case "critical":
				score -= 10
			case "high":
				score -= 5
			case "medium":
				score -= 2
			case "low":
				score -= 1
			}
		}
	}

	// Clamp to 0-100
	if score < 0 {
		score = 0
	}

	return score
}

func (r *GryviaAuditReconciler) updateCondition(audit *gryviav1.GryviaAudit, condType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}
	meta.SetStatusCondition(&audit.Status.Conditions, condition)
}

// SetupWithManager sets up the controller with the Manager.
func (r *GryviaAuditReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gryviav1.GryviaAudit{}).
		Watches(&gryviav1.GryviaAIJob{}, handler.EnqueueRequestsFromMapFunc(
			func(ctx context.Context, obj client.Object) []reconcile.Request {
				// When a GryviaAIJob changes, enqueue all GryviaAudit objects
				auditList := &gryviav1.GryviaAuditList{}
				if err := mgr.GetClient().List(ctx, auditList); err != nil {
					return nil
				}
				var requests []reconcile.Request
				for _, audit := range auditList.Items {
					requests = append(requests, reconcile.Request{
						NamespacedName: types.NamespacedName{
							Name: audit.Name,
						},
					})
				}
				return requests
			},
		)).
		Watches(&gryviav1.GryviaQuota{}, handler.EnqueueRequestsFromMapFunc(
			func(ctx context.Context, obj client.Object) []reconcile.Request {
				auditList := &gryviav1.GryviaAuditList{}
				if err := mgr.GetClient().List(ctx, auditList); err != nil {
					return nil
				}
				var requests []reconcile.Request
				for _, audit := range auditList.Items {
					requests = append(requests, reconcile.Request{
						NamespacedName: types.NamespacedName{
							Name: audit.Name,
						},
					})
				}
				return requests
			},
		)).
		Complete(r)
}
