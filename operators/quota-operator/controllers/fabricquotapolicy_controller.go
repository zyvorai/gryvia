package controllers

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	tensorreaperv1 "github.com/ssahani/TensorReaper/operators/quota-operator/api/v1"
	"github.com/ssahani/TensorReaper/operators/quota-operator/pkg/budget"
)

// FabricQuotaPolicyReconciler reconciles a FabricQuotaPolicy object
type FabricQuotaPolicyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabricquotapolicies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabricquotapolicies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabricquotapolicies/finalizers,verbs=update
//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabricaijobs,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabricquotas,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *FabricQuotaPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the FabricQuotaPolicy instance
	policy := &tensorreaperv1.FabricQuotaPolicy{}
	err := r.Get(ctx, req.NamespacedName, policy)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("FabricQuotaPolicy resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get FabricQuotaPolicy")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !policy.ObjectMeta.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	logger.Info("Reconciling FabricQuotaPolicy", "scope", policy.Spec.Scope.Name)

	// Reconcile the quota policy
	result, err := r.reconcileQuotaPolicy(ctx, policy)
	if err != nil {
		logger.Error(err, "Failed to reconcile quota policy")
		r.updateCondition(policy, "Ready", metav1.ConditionFalse, "ReconcileFailed", err.Error())
		if statusErr := r.Status().Update(ctx, policy); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after reconcile failure")
		}
		return result, err
	}

	// Update status
	if err := r.Status().Update(ctx, policy); err != nil {
		logger.Error(err, "Failed to update FabricQuotaPolicy status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
}

func (r *FabricQuotaPolicyReconciler) reconcileQuotaPolicy(ctx context.Context, policy *tensorreaperv1.FabricQuotaPolicy) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Calculate current usage
	usage, err := r.calculatePolicyUsage(ctx, policy)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to calculate usage: %w", err)
	}
	policy.Status.Usage = usage

	// Calculate utilization
	policy.Status.Utilization = r.calculatePolicyUtilization(policy)

	// Determine active time-based schedule
	if policy.Spec.TimeBased != nil && policy.Spec.TimeBased.Enabled {
		r.applyTimeBasedPolicy(policy)
	} else {
		policy.Status.EffectiveMultiplier = 1.0
		policy.Status.ActiveSchedule = ""
	}

	// Determine quota state
	state := r.determineState(policy)
	policy.Status.State = state

	// Handle hierarchical children
	if policy.Spec.Hierarchy != nil && policy.Spec.Hierarchy.Enabled && policy.Spec.Hierarchy.AllowChildren {
		if err := r.reconcileChildren(ctx, policy); err != nil {
			logger.Error(err, "Failed to reconcile children")
		}
	}

	// Enforce quota based on the policy
	if policy.Spec.Enforcement != nil && (state == "exceeded" || state == "blocked") {
		if err := r.enforcePolicy(ctx, policy); err != nil {
			logger.Error(err, "Failed to enforce quota policy")
		}
	}

	r.updateCondition(policy, "Ready", metav1.ConditionTrue, "PolicyActive",
		fmt.Sprintf("Quota policy %s: state=%s", policy.Spec.Scope.Name, state))

	return ctrl.Result{}, nil
}

func (r *FabricQuotaPolicyReconciler) calculatePolicyUsage(ctx context.Context, policy *tensorreaperv1.FabricQuotaPolicy) (*tensorreaperv1.QuotaPolicyUsage, error) {
	policyUsage := &tensorreaperv1.QuotaPolicyUsage{
		GPU:  &tensorreaperv1.QuotaPolicyGPUUsage{ByType: make(map[string]float64)},
		Cost: &tensorreaperv1.QuotaPolicyCostUsage{},
		Jobs: &tensorreaperv1.QuotaPolicyJobUsage{},
	}

	// Find namespaces matching the scope via FabricQuota
	namespaces := r.findPolicyNamespaces(ctx, policy)

	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	for _, ns := range namespaces {
		jobList := &tensorreaperv1.FabricAIJobList{}
		if err := r.List(ctx, jobList, client.InNamespace(ns)); err != nil {
			continue
		}

		for _, job := range jobList.Items {
			switch job.Status.Phase {
			case "Running":
				policyUsage.Jobs.Concurrent++
				policyUsage.GPU.Concurrent += int(job.Spec.GPUs)

				if job.Status.StartTime != nil {
					effectiveStart := job.Status.StartTime.Time
					if effectiveStart.Before(startOfMonth) {
						effectiveStart = startOfMonth
					}
					hours := time.Since(effectiveStart).Hours()
					gpuHours := hours * float64(job.Spec.GPUs)
					policyUsage.GPU.HoursThisMonth += gpuHours
					policyUsage.GPU.ByType[job.Spec.GpuType] += gpuHours
					policyUsage.Cost.MonthToDate += gpuHours * budget.GetGPURate(job.Spec.GpuType)
				}
				policyUsage.Jobs.ThisMonth++
				policyUsage.Jobs.Today++

			case "Succeeded", "Failed":
				if job.Status.StartTime != nil && job.Status.CompletionTime != nil {
					effectiveStart := job.Status.StartTime.Time
					if effectiveStart.Before(startOfMonth) {
						effectiveStart = startOfMonth
					}
					if job.Status.CompletionTime.Time.After(startOfMonth) {
						hours := job.Status.CompletionTime.Time.Sub(effectiveStart).Hours()
						if hours > 0 {
							gpuHours := hours * float64(job.Spec.GPUs)
							policyUsage.GPU.HoursThisMonth += gpuHours
							policyUsage.GPU.ByType[job.Spec.GpuType] += gpuHours
							policyUsage.Cost.MonthToDate += gpuHours * budget.GetGPURate(job.Spec.GpuType)
						}
					}
				}
				policyUsage.Jobs.ThisMonth++

			case "Pending", "Queued":
				policyUsage.Jobs.Concurrent++
			}
		}
	}

	return policyUsage, nil
}

func (r *FabricQuotaPolicyReconciler) findPolicyNamespaces(ctx context.Context, policy *tensorreaperv1.FabricQuotaPolicy) []string {
	var namespaces []string

	switch policy.Spec.Scope.Type {
	case "namespace":
		namespaces = append(namespaces, policy.Spec.Scope.Name)
	case "team", "division", "organization":
		// Find namespaces via FabricQuota team mappings
		quotaList := &tensorreaperv1.FabricQuotaList{}
		if err := r.List(ctx, quotaList); err != nil {
			return namespaces
		}
		for _, quota := range quotaList.Items {
			if quota.Spec.Team == policy.Spec.Scope.Name {
				namespaces = append(namespaces, quota.Spec.Namespaces...)
			}
		}
	default:
		// List all namespaces
		quotaList := &tensorreaperv1.FabricQuotaList{}
		if err := r.List(ctx, quotaList); err != nil {
			return namespaces
		}
		for _, quota := range quotaList.Items {
			namespaces = append(namespaces, quota.Spec.Namespaces...)
		}
	}

	return namespaces
}

func (r *FabricQuotaPolicyReconciler) calculatePolicyUtilization(policy *tensorreaperv1.FabricQuotaPolicy) *tensorreaperv1.QuotaPolicyUtilization {
	util := &tensorreaperv1.QuotaPolicyUtilization{}

	if policy.Status.Usage == nil {
		return util
	}

	// Apply time-based multiplier
	multiplier := policy.Status.EffectiveMultiplier
	if multiplier <= 0 {
		multiplier = 1.0
	}

	// GPU utilization
	if policy.Spec.Limits.GPU != nil && policy.Spec.Limits.GPU.Concurrent > 0 {
		effectiveLimit := float64(policy.Spec.Limits.GPU.Concurrent) * multiplier
		util.GPU = (float64(policy.Status.Usage.GPU.Concurrent) / effectiveLimit) * 100
	}

	// Cost utilization
	if policy.Spec.Limits.Cost != nil && policy.Spec.Limits.Cost.MonthlyBudget > 0 {
		util.Cost = (policy.Status.Usage.Cost.MonthToDate / policy.Spec.Limits.Cost.MonthlyBudget) * 100
	}

	return util
}

func (r *FabricQuotaPolicyReconciler) applyTimeBasedPolicy(policy *tensorreaperv1.FabricQuotaPolicy) {
	if policy.Spec.TimeBased == nil || !policy.Spec.TimeBased.Enabled {
		policy.Status.EffectiveMultiplier = 1.0
		return
	}

	now := time.Now()
	dayOfWeek := now.Weekday().String()[:3] // Mon, Tue, etc.
	currentTime := now.Format("15:04")

	for _, schedule := range policy.Spec.TimeBased.Schedules {
		if schedule.Window == nil {
			continue
		}

		// Check if current day matches
		dayMatch := false
		for _, day := range schedule.Window.DaysOfWeek {
			if day == dayOfWeek {
				dayMatch = true
				break
			}
		}
		if !dayMatch {
			continue
		}

		// Check if current time is within the window
		if currentTime >= schedule.Window.StartTime && currentTime <= schedule.Window.EndTime {
			policy.Status.ActiveSchedule = schedule.Name
			policy.Status.EffectiveMultiplier = schedule.Multiplier
			return
		}
	}

	// No matching schedule, use default
	policy.Status.ActiveSchedule = "default"
	policy.Status.EffectiveMultiplier = 1.0
}

func (r *FabricQuotaPolicyReconciler) determineState(policy *tensorreaperv1.FabricQuotaPolicy) string {
	if policy.Status.Utilization == nil {
		return "under-quota"
	}

	maxUtil := policy.Status.Utilization.GPU
	if policy.Status.Utilization.Cost > maxUtil {
		maxUtil = policy.Status.Utilization.Cost
	}

	// Check for burst allowance
	burstLimit := 0.0
	if policy.Spec.Enforcement != nil && policy.Spec.Enforcement.AllowBurst {
		burstLimit = policy.Spec.Enforcement.BurstLimit
	}

	if maxUtil >= 100+burstLimit {
		if policy.Spec.Enforcement != nil && policy.Spec.Enforcement.OnExceeded == "block" {
			return "blocked"
		}
		return "exceeded"
	}

	if maxUtil >= 85 {
		return "near-limit"
	}

	return "under-quota"
}

func (r *FabricQuotaPolicyReconciler) reconcileChildren(ctx context.Context, policy *tensorreaperv1.FabricQuotaPolicy) error {
	// Find child policies
	policyList := &tensorreaperv1.FabricQuotaPolicyList{}
	if err := r.List(ctx, policyList); err != nil {
		return err
	}

	var children []tensorreaperv1.QuotaPolicyChildStatus
	for _, child := range policyList.Items {
		if child.Spec.Hierarchy != nil && child.Spec.Hierarchy.ParentRef == policy.Name {
			allocated := 0.0
			used := 0.0
			if child.Spec.Limits.GPU != nil {
				allocated = float64(child.Spec.Limits.GPU.Concurrent)
			}
			if child.Status.Usage != nil && child.Status.Usage.GPU != nil {
				used = float64(child.Status.Usage.GPU.Concurrent)
			}

			children = append(children, tensorreaperv1.QuotaPolicyChildStatus{
				Name:      child.Name,
				Allocated: allocated,
				Used:      used,
			})
		}
	}

	policy.Status.Children = children
	return nil
}

func (r *FabricQuotaPolicyReconciler) enforcePolicy(ctx context.Context, policy *tensorreaperv1.FabricQuotaPolicy) error {
	logger := log.FromContext(ctx)

	if policy.Spec.Enforcement == nil {
		return nil
	}

	namespaces := r.findPolicyNamespaces(ctx, policy)

	for _, ns := range namespaces {
		jobList := &tensorreaperv1.FabricAIJobList{}
		if err := r.List(ctx, jobList, client.InNamespace(ns)); err != nil {
			continue
		}

		for i := range jobList.Items {
			job := &jobList.Items[i]
			if job.Status.Phase != "Pending" && job.Status.Phase != "Queued" {
				continue
			}

			switch policy.Spec.Enforcement.OnExceeded {
			case "block":
				job.Status.Phase = "Rejected"
				condition := metav1.Condition{
					Type:               "Rejected",
					Status:             metav1.ConditionTrue,
					Reason:             "QuotaPolicyExceeded",
					Message:            fmt.Sprintf("Quota policy %s exceeded for %s", policy.Name, policy.Spec.Scope.Name),
					LastTransitionTime: metav1.Now(),
				}
				meta.SetStatusCondition(&job.Status.Conditions, condition)
				if err := r.Status().Update(ctx, job); err != nil {
					logger.Error(err, "Failed to reject job", "job", job.Name)
				}

			case "queue":
				// Keep job in Queued state
				if job.Status.Phase != "Queued" {
					job.Status.Phase = "Queued"
					if err := r.Status().Update(ctx, job); err != nil {
						logger.Error(err, "Failed to queue job", "job", job.Name)
					}
				}
			}
		}
	}

	return nil
}

func (r *FabricQuotaPolicyReconciler) updateCondition(policy *tensorreaperv1.FabricQuotaPolicy, condType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}
	meta.SetStatusCondition(&policy.Status.Conditions, condition)
}

// SetupWithManager sets up the controller with the Manager.
func (r *FabricQuotaPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tensorreaperv1.FabricQuotaPolicy{}).
		Complete(r)
}
