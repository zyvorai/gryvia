package controllers

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kubefabricv1 "github.com/yourusername/kubefabric/operators/quota-operator/api/v1"
	"github.com/yourusername/kubefabric/operators/quota-operator/pkg/budget"
	"github.com/yourusername/kubefabric/operators/quota-operator/pkg/usage"
)

// FabricQuotaReconciler reconciles a FabricQuota object
type FabricQuotaReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=kubefabric.io,resources=fabricquotas,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kubefabric.io,resources=fabricquotas/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kubefabric.io,resources=fabricquotas/finalizers,verbs=update
//+kubebuilder:rbac:groups=kubefabric.io,resources=fabricaijobs,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups="",resources=resourcequotas,verbs=get;list;watch;create;update;patch;delete

func (r *FabricQuotaReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the FabricQuota instance
	quota := &kubefabricv1.FabricQuota{}
	err := r.Get(ctx, req.NamespacedName, quota)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("FabricQuota resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get FabricQuota")
		return ctrl.Result{}, err
	}

	logger.Info("Reconciling FabricQuota", "team", quota.Spec.Team, "maxGPUs", quota.Spec.GPUQuota.MaxGPUs)

	// Reconcile the quota
	result, err := r.reconcileQuota(ctx, quota)
	if err != nil {
		logger.Error(err, "Failed to reconcile quota")
		r.updateStatus(ctx, quota, "Failed", err.Error())
		return result, err
	}

	// Update status
	r.updateStatus(ctx, quota, quota.Status.Phase, "Quota reconciled successfully")

	return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
}

func (r *FabricQuotaReconciler) reconcileQuota(ctx context.Context, quota *kubefabricv1.FabricQuota) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Ensure namespaces have correct labels
	if err := r.labelNamespaces(ctx, quota); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to label namespaces: %w", err)
	}

	// Calculate current usage
	currentUsage, err := usage.CalculateUsage(ctx, r.Client, quota)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to calculate usage: %w", err)
	}

	quota.Status.CurrentUsage = *currentUsage

	// Check quota violations
	phase := "Active"
	if currentUsage.AllocatedGPUs > quota.Spec.GPUQuota.MaxGPUs {
		phase = "QuotaExceeded"
		logger.Info("GPU quota exceeded", "allocated", currentUsage.AllocatedGPUs, "max", quota.Spec.GPUQuota.MaxGPUs)
	}

	if quota.Spec.GPUQuota.MaxRunningJobs > 0 && currentUsage.RunningJobs > quota.Spec.GPUQuota.MaxRunningJobs {
		phase = "QuotaExceeded"
		logger.Info("Running jobs quota exceeded", "running", currentUsage.RunningJobs, "max", quota.Spec.GPUQuota.MaxRunningJobs)
	}

	// Calculate budget if specified
	if quota.Spec.Budget != nil {
		budgetStatus, err := budget.CalculateBudget(ctx, r.Client, quota, currentUsage)
		if err != nil {
			logger.Error(err, "Failed to calculate budget")
		} else {
			quota.Status.BudgetStatus = budgetStatus

			// Check budget limits
			if budgetStatus.PercentUsed >= 100 && quota.Spec.Budget.HardLimit {
				phase = "BudgetExceeded"
				logger.Info("Budget exceeded", "spent", budgetStatus.SpentThisMonth, "budget", quota.Spec.Budget.MonthlyBudget)
			} else if budgetStatus.PercentUsed >= quota.Spec.Budget.AlertThreshold {
				logger.Info("Budget alert threshold reached", "percentUsed", budgetStatus.PercentUsed, "threshold", quota.Spec.Budget.AlertThreshold)
			}
		}
	}

	quota.Status.Phase = phase

	// Enforce quota by updating jobs
	if phase == "QuotaExceeded" || phase == "BudgetExceeded" {
		if err := r.enforceQuota(ctx, quota); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to enforce quota: %w", err)
		}
	}

	return ctrl.Result{}, nil
}

func (r *FabricQuotaReconciler) labelNamespaces(ctx context.Context, quota *kubefabricv1.FabricQuota) error {
	logger := log.FromContext(ctx)

	for _, nsName := range quota.Spec.Namespaces {
		ns := &corev1.Namespace{}
		err := r.Get(ctx, types.NamespacedName{Name: nsName}, ns)
		if err != nil {
			if errors.IsNotFound(err) {
				logger.Info("Namespace not found, skipping", "namespace", nsName)
				continue
			}
			return err
		}

		if ns.Labels == nil {
			ns.Labels = make(map[string]string)
		}

		// Add team label
		if ns.Labels["kubefabric.io/team"] != quota.Spec.Team {
			ns.Labels["kubefabric.io/team"] = quota.Spec.Team
			ns.Labels["kubefabric.io/quota"] = quota.Name

			if err := r.Update(ctx, ns); err != nil {
				return fmt.Errorf("failed to update namespace %s: %w", nsName, err)
			}
			logger.Info("Labeled namespace", "namespace", nsName, "team", quota.Spec.Team)
		}
	}

	return nil
}

func (r *FabricQuotaReconciler) enforceQuota(ctx context.Context, quota *kubefabricv1.FabricQuota) error {
	logger := log.FromContext(ctx)

	// Get all AI jobs in quota namespaces
	jobList := &kubefabricv1.FabricAIJobList{}
	for _, nsName := range quota.Spec.Namespaces {
		jobs := &kubefabricv1.FabricAIJobList{}
		if err := r.List(ctx, jobs, client.InNamespace(nsName)); err != nil {
			logger.Error(err, "Failed to list jobs in namespace", "namespace", nsName)
			continue
		}
		jobList.Items = append(jobList.Items, jobs.Items...)
	}

	// Sort jobs by priority and creation time
	pendingJobs := []kubefabricv1.FabricAIJob{}
	for _, job := range jobList.Items {
		if job.Status.Phase == "Pending" || job.Status.Phase == "Queued" {
			pendingJobs = append(pendingJobs, job)
		}
	}

	// Reject jobs that exceed quota
	for _, job := range pendingJobs {
		rejected := false
		reason := ""

		if quota.Status.Phase == "BudgetExceeded" && quota.Spec.Budget.HardLimit {
			rejected = true
			reason = fmt.Sprintf("Budget exceeded for team %s", quota.Spec.Team)
		}

		if int(job.Spec.GPUs) > quota.Spec.GPUQuota.MaxGPUsPerJob {
			rejected = true
			reason = fmt.Sprintf("Job requests %d GPUs, exceeds max %d per job", job.Spec.GPUs, quota.Spec.GPUQuota.MaxGPUsPerJob)
		}

		if rejected {
			job.Status.Phase = "Rejected"
			meta.SetStatusCondition(&job.Status.Conditions, metav1.Condition{
				Type:               "Rejected",
				Status:             metav1.ConditionTrue,
				Reason:             "QuotaExceeded",
				Message:            reason,
				LastTransitionTime: metav1.Now(),
			})
			if err := r.Status().Update(ctx, &job); err != nil {
				logger.Error(err, "Failed to update job status", "job", job.Name)
			}
			logger.Info("Rejected job", "job", job.Name, "reason", reason)
		}
	}

	return nil
}

func (r *FabricQuotaReconciler) updateStatus(ctx context.Context, quota *kubefabricv1.FabricQuota, phase, message string) {
	if quota.Status.Phase == "" {
		quota.Status.Phase = phase
	}
	quota.Status.LastUpdated = metav1.Now()

	condition := metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             phase,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}

	if phase == "Failed" || phase == "QuotaExceeded" || phase == "BudgetExceeded" {
		condition.Status = metav1.ConditionFalse
	}

	meta.SetStatusCondition(&quota.Status.Conditions, condition)

	if err := r.Status().Update(ctx, quota); err != nil {
		log.FromContext(ctx).Error(err, "Failed to update FabricQuota status")
	}
}

// SetupWithManager sets up the controller with the Manager
func (r *FabricQuotaReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubefabricv1.FabricQuota{}).
		Complete(r)
}
