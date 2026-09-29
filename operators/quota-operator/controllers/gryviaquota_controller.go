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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gryviav1 "github.com/zyvorai/gryvia/operators/quota-operator/api/v1"
	"github.com/zyvorai/gryvia/operators/quota-operator/pkg/budget"
	"github.com/zyvorai/gryvia/operators/quota-operator/pkg/usage"
)

const (
	gryviaQuotaFinalizer = "gryvia.io/quota-finalizer"
)

// GryviaQuotaReconciler reconciles a GryviaQuota object
type GryviaQuotaReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=gryvia.io,resources=gryviaquotas,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gryvia.io,resources=gryviaquotas/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gryvia.io,resources=gryviaquotas/finalizers,verbs=update
//+kubebuilder:rbac:groups=gryvia.io,resources=gryviaaijobs,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups="",resources=resourcequotas,verbs=get;list;watch;create;update;patch;delete

func (r *GryviaQuotaReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the GryviaQuota instance
	quota := &gryviav1.GryviaQuota{}
	err := r.Get(ctx, req.NamespacedName, quota)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("GryviaQuota resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get GryviaQuota")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !quota.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, quota)
	}

	// Add finalizer if it doesn't exist
	if !controllerutil.ContainsFinalizer(quota, gryviaQuotaFinalizer) {
		controllerutil.AddFinalizer(quota, gryviaQuotaFinalizer)
		if err := r.Update(ctx, quota); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	logger.Info("Reconciling GryviaQuota", "team", quota.Spec.Team, "maxGPUs", quota.Spec.GPUQuota.MaxGPUs)

	// Reconcile the quota
	result, err := r.reconcileQuota(ctx, quota)
	if err != nil {
		logger.Error(err, "Failed to reconcile quota")
		if statusErr := r.updateStatus(ctx, quota, "Failed", err.Error()); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after reconcile failure")
		}
		return result, err
	}

	// Update status
	if err := r.updateStatus(ctx, quota, quota.Status.Phase, "Quota reconciled successfully"); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
}

func (r *GryviaQuotaReconciler) reconcileQuota(ctx context.Context, quota *gryviav1.GryviaQuota) (ctrl.Result, error) {
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

func (r *GryviaQuotaReconciler) labelNamespaces(ctx context.Context, quota *gryviav1.GryviaQuota) error {
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
		if ns.Labels["gryvia.io/team"] != quota.Spec.Team {
			ns.Labels["gryvia.io/team"] = quota.Spec.Team
			ns.Labels["gryvia.io/quota"] = quota.Name

			if err := r.Update(ctx, ns); err != nil {
				return fmt.Errorf("failed to update namespace %s: %w", nsName, err)
			}
			logger.Info("Labeled namespace", "namespace", nsName, "team", quota.Spec.Team)
		}
	}

	return nil
}

func (r *GryviaQuotaReconciler) enforceQuota(ctx context.Context, quota *gryviav1.GryviaQuota) error {
	logger := log.FromContext(ctx)

	// Get all AI jobs in quota namespaces
	jobList := &gryviav1.GryviaAIJobList{}
	for _, nsName := range quota.Spec.Namespaces {
		jobs := &gryviav1.GryviaAIJobList{}
		if err := r.List(ctx, jobs, client.InNamespace(nsName)); err != nil {
			return fmt.Errorf("failed to list jobs in namespace %s: %w", nsName, err)
		}
		jobList.Items = append(jobList.Items, jobs.Items...)
	}

	// Sort jobs by priority and creation time
	pendingJobs := []gryviav1.GryviaAIJob{}
	for _, job := range jobList.Items {
		if job.Status.Phase == "Pending" || job.Status.Phase == "Queued" {
			pendingJobs = append(pendingJobs, job)
		}
	}

	// Reject jobs that exceed quota
	for _, job := range pendingJobs {
		if job.Spec.GPUs <= 0 {
			continue // skip invalid job specs
		}

		rejected := false
		reason := ""

		if quota.Status.Phase == "BudgetExceeded" && quota.Spec.Budget.HardLimit {
			rejected = true
			reason = fmt.Sprintf("Budget exceeded for team %s", quota.Spec.Team)
		}

		if quota.Spec.GPUQuota.MaxGPUsPerJob > 0 && int(job.Spec.GPUs) > quota.Spec.GPUQuota.MaxGPUsPerJob {
			rejected = true
			reason = fmt.Sprintf("Job requests %d GPUs, exceeds max %d per job", job.Spec.GPUs, quota.Spec.GPUQuota.MaxGPUsPerJob)
		}

		// Check GPU type is allowed
		if len(quota.Spec.GPUQuota.AllowedGPUTypes) > 0 {
			gpuType := job.Spec.GpuType
			allowed := false
			for _, t := range quota.Spec.GPUQuota.AllowedGPUTypes {
				if t == gpuType {
					allowed = true
					break
				}
			}
			if !allowed {
				// Queue the job - GPU type not allowed
				logger.Info("Job uses disallowed GPU type", "job", job.Name, "gpuType", gpuType)
				continue
			}
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
				return fmt.Errorf("failed to reject job %s: %w", job.Name, err)
			}
			logger.Info("Rejected job", "job", job.Name, "reason", reason)
		}
	}

	return nil
}

func (r *GryviaQuotaReconciler) updateStatus(ctx context.Context, quota *gryviav1.GryviaQuota, phase, message string) error {
	quota.Status.Phase = phase
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
		log.FromContext(ctx).Error(err, "Failed to update GryviaQuota status")
		return err
	}
	return nil
}

func (r *GryviaQuotaReconciler) handleDeletion(ctx context.Context, quota *gryviav1.GryviaQuota) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if controllerutil.ContainsFinalizer(quota, gryviaQuotaFinalizer) {
		logger.Info("Running cleanup for GryviaQuota", "team", quota.Spec.Team)

		// Remove gryvia labels from namespaces
		for _, nsName := range quota.Spec.Namespaces {
			ns := &corev1.Namespace{}
			err := r.Get(ctx, types.NamespacedName{Name: nsName}, ns)
			if err != nil {
				if !errors.IsNotFound(err) {
					logger.Error(err, "Failed to get namespace during cleanup", "namespace", nsName)
				}
				continue
			}

			if ns.Labels != nil {
				delete(ns.Labels, "gryvia.io/team")
				delete(ns.Labels, "gryvia.io/quota")
				if err := r.Update(ctx, ns); err != nil {
					logger.Error(err, "Failed to remove labels from namespace", "namespace", nsName)
				}
			}
		}

		// Remove finalizer
		controllerutil.RemoveFinalizer(quota, gryviaQuotaFinalizer)
		if err := r.Update(ctx, quota); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager
func (r *GryviaQuotaReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gryviav1.GryviaQuota{}).
		Watches(&gryviav1.GryviaAIJob{}, handler.EnqueueRequestsFromMapFunc(
			func(ctx context.Context, obj client.Object) []reconcile.Request {
				// When a GryviaAIJob changes, enqueue all GryviaQuota objects
				// in the same namespace so quota usage is recalculated.
				quotaList := &gryviav1.GryviaQuotaList{}
				if err := mgr.GetClient().List(ctx, quotaList); err != nil {
					return nil
				}
				var requests []reconcile.Request
				for _, quota := range quotaList.Items {
					for _, ns := range quota.Spec.Namespaces {
						if ns == obj.GetNamespace() {
							requests = append(requests, reconcile.Request{
								NamespacedName: types.NamespacedName{
									Name:      quota.Name,
									Namespace: quota.Namespace,
								},
							})
							break
						}
					}
				}
				return requests
			},
		)).
		Complete(r)
}
