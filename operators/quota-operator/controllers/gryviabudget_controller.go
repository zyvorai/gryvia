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
	"github.com/zyvorai/gryvia/operators/quota-operator/pkg/budget"
)

// GryviaBudgetReconciler reconciles a GryviaBudget object
type GryviaBudgetReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=gryvia.io,resources=gryviabudgets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gryvia.io,resources=gryviabudgets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gryvia.io,resources=gryviabudgets/finalizers,verbs=update
//+kubebuilder:rbac:groups=gryvia.io,resources=gryviaaijobs,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=gryvia.io,resources=gryviaquotas,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch

func (r *GryviaBudgetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the GryviaBudget instance
	fb := &gryviav1.GryviaBudget{}
	err := r.Get(ctx, req.NamespacedName, fb)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("GryviaBudget resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get GryviaBudget")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !fb.ObjectMeta.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	logger.Info("Reconciling GryviaBudget", "scope", fb.Spec.Scope.Name, "period", fb.Spec.Period.Type)

	// Reconcile the budget
	result, err := r.reconcileBudget(ctx, fb)
	if err != nil {
		logger.Error(err, "Failed to reconcile budget")
		r.updateCondition(fb, "Ready", metav1.ConditionFalse, "ReconcileFailed", err.Error())
		if statusErr := r.Status().Update(ctx, fb); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after reconcile failure")
		}
		return result, err
	}

	// Update status
	if err := r.Status().Update(ctx, fb); err != nil {
		logger.Error(err, "Failed to update GryviaBudget status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
}

func (r *GryviaBudgetReconciler) reconcileBudget(ctx context.Context, fb *gryviav1.GryviaBudget) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Calculate current period
	r.calculateCurrentPeriod(fb)

	// Calculate usage by listing jobs matching the budget scope
	usage, err := r.calculateUsage(ctx, fb)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to calculate usage: %w", err)
	}
	fb.Status.Usage = usage

	// Calculate utilization percentages
	fb.Status.Utilization = r.calculateUtilization(fb)

	// Calculate burn rate and forecast
	fb.Status.Forecast = r.calculateForecast(fb)

	// Determine budget state
	state := "active"
	costPercent := 0.0
	if fb.Status.Utilization != nil {
		costPercent = fb.Status.Utilization.CostPercent
	}

	// Check alert thresholds and emit events
	for _, alert := range fb.Spec.Alerts {
		if costPercent >= alert.Threshold {
			r.checkAndEmitAlert(ctx, fb, alert, costPercent)

			// Check if this threshold has a block action
			for _, action := range alert.Actions {
				if action == "block" {
					state = "blocked"
				}
			}

			if state != "blocked" && costPercent >= 100 {
				state = "exceeded"
			} else if state != "blocked" && state != "exceeded" {
				state = "warning"
			}
		}
	}

	if costPercent < 50 {
		state = "active"
	}

	fb.Status.State = state

	// Enforce budget if enabled
	if fb.Spec.Enforcement != nil && fb.Spec.Enforcement.Enabled && state == "blocked" {
		if err := r.enforceBudget(ctx, fb); err != nil {
			logger.Error(err, "Failed to enforce budget")
		}
	}

	r.updateCondition(fb, "Ready", metav1.ConditionTrue, "BudgetReconciled",
		fmt.Sprintf("Budget %s: %.1f%% consumed", fb.Status.State, costPercent))

	return ctrl.Result{}, nil
}

func (r *GryviaBudgetReconciler) calculateCurrentPeriod(fb *gryviav1.GryviaBudget) {
	now := time.Now()
	period := &gryviav1.BudgetCurrentPeriod{}

	switch fb.Spec.Period.Type {
	case "daily":
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		end := start.Add(24 * time.Hour)
		period.StartDate = metav1.NewTime(start)
		period.EndDate = metav1.NewTime(end)
		period.DaysRemaining = 0
	case "weekly":
		weekday := int(now.Weekday())
		start := time.Date(now.Year(), now.Month(), now.Day()-weekday, 0, 0, 0, 0, now.Location())
		end := start.AddDate(0, 0, 7)
		period.StartDate = metav1.NewTime(start)
		period.EndDate = metav1.NewTime(end)
		period.DaysRemaining = int(end.Sub(now).Hours() / 24)
	case "monthly":
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		end := start.AddDate(0, 1, 0)
		period.StartDate = metav1.NewTime(start)
		period.EndDate = metav1.NewTime(end)
		period.DaysRemaining = int(end.Sub(now).Hours() / 24)
	case "quarterly":
		quarter := (int(now.Month()) - 1) / 3
		startMonth := time.Month(quarter*3 + 1)
		start := time.Date(now.Year(), startMonth, 1, 0, 0, 0, 0, now.Location())
		end := start.AddDate(0, 3, 0)
		period.StartDate = metav1.NewTime(start)
		period.EndDate = metav1.NewTime(end)
		period.DaysRemaining = int(end.Sub(now).Hours() / 24)
	default:
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		end := start.AddDate(0, 1, 0)
		period.StartDate = metav1.NewTime(start)
		period.EndDate = metav1.NewTime(end)
		period.DaysRemaining = int(end.Sub(now).Hours() / 24)
	}

	fb.Status.CurrentPeriod = period
}

func (r *GryviaBudgetReconciler) calculateUsage(ctx context.Context, fb *gryviav1.GryviaBudget) (*gryviav1.BudgetUsage, error) {
	budgetUsage := &gryviav1.BudgetUsage{
		GPUTypeHours: make(map[string]float64),
	}

	// Find matching jobs based on scope
	jobs, err := r.findScopeJobs(ctx, fb)
	if err != nil {
		return nil, err
	}

	periodStart := time.Time{}
	if fb.Status.CurrentPeriod != nil {
		periodStart = fb.Status.CurrentPeriod.StartDate.Time
	}

	for _, job := range jobs {
		switch job.Status.Phase {
		case "Running":
			budgetUsage.CurrentConcurrentJobs++
			budgetUsage.CurrentConcurrentGPUs += int(job.Spec.GPUs)

			if job.Status.StartTime != nil && !job.Status.StartTime.IsZero() {
				effectiveStart := job.Status.StartTime.Time
				if effectiveStart.Before(periodStart) {
					effectiveStart = periodStart
				}
				hours := time.Since(effectiveStart).Hours()
				gpuHours := hours * float64(job.Spec.GPUs)
				budgetUsage.GPUHours += gpuHours
				budgetUsage.CostUSD += gpuHours * budget.GetGPURate(job.Spec.GpuType)

				// Per GPU type tracking
				budgetUsage.GPUTypeHours[job.Spec.GpuType] += gpuHours
			}
			budgetUsage.JobCount++

		case "Succeeded", "Failed":
			if job.Status.StartTime != nil && job.Status.CompletionTime != nil {
				effectiveStart := job.Status.StartTime.Time
				if effectiveStart.Before(periodStart) {
					effectiveStart = periodStart
				}
				endTime := job.Status.CompletionTime.Time
				hours := endTime.Sub(effectiveStart).Hours()
				if hours > 0 {
					gpuHours := hours * float64(job.Spec.GPUs)
					budgetUsage.GPUHours += gpuHours
					budgetUsage.CostUSD += gpuHours * budget.GetGPURate(job.Spec.GpuType)
					budgetUsage.GPUTypeHours[job.Spec.GpuType] += gpuHours
				}
			}
			budgetUsage.JobCount++
		}
	}

	return budgetUsage, nil
}

func (r *GryviaBudgetReconciler) findScopeJobs(ctx context.Context, fb *gryviav1.GryviaBudget) ([]gryviav1.GryviaAIJob, error) {
	var allJobs []gryviav1.GryviaAIJob

	switch fb.Spec.Scope.Type {
	case "namespace":
		jobList := &gryviav1.GryviaAIJobList{}
		if err := r.List(ctx, jobList, client.InNamespace(fb.Spec.Scope.Name)); err != nil {
			return nil, fmt.Errorf("failed to list jobs in namespace %s: %w", fb.Spec.Scope.Name, err)
		}
		allJobs = jobList.Items

	case "team":
		// Find namespaces belonging to the team via GryviaQuota
		quotaList := &gryviav1.GryviaQuotaList{}
		if err := r.List(ctx, quotaList); err != nil {
			return nil, fmt.Errorf("failed to list quotas: %w", err)
		}
		for _, quota := range quotaList.Items {
			if quota.Spec.Team == fb.Spec.Scope.Name {
				for _, ns := range quota.Spec.Namespaces {
					jobList := &gryviav1.GryviaAIJobList{}
					if err := r.List(ctx, jobList, client.InNamespace(ns)); err != nil {
						continue
					}
					allJobs = append(allJobs, jobList.Items...)
				}
			}
		}

	default:
		// List all jobs
		jobList := &gryviav1.GryviaAIJobList{}
		if err := r.List(ctx, jobList); err != nil {
			return nil, fmt.Errorf("failed to list all jobs: %w", err)
		}
		allJobs = jobList.Items
	}

	return allJobs, nil
}

func (r *GryviaBudgetReconciler) calculateUtilization(fb *gryviav1.GryviaBudget) *gryviav1.BudgetUtilization {
	util := &gryviav1.BudgetUtilization{}

	if fb.Status.Usage == nil {
		return util
	}

	if fb.Spec.Limits.CostUSD > 0 {
		util.CostPercent = (fb.Status.Usage.CostUSD / fb.Spec.Limits.CostUSD) * 100
	}

	if fb.Spec.Limits.GPUHours > 0 {
		util.GPUHoursPercent = (fb.Status.Usage.GPUHours / fb.Spec.Limits.GPUHours) * 100
	}

	if fb.Spec.Limits.MaxConcurrentJobs > 0 {
		util.JobsPercent = (float64(fb.Status.Usage.CurrentConcurrentJobs) / float64(fb.Spec.Limits.MaxConcurrentJobs)) * 100
	}

	return util
}

func (r *GryviaBudgetReconciler) calculateForecast(fb *gryviav1.GryviaBudget) *gryviav1.BudgetForecast {
	forecast := &gryviav1.BudgetForecast{}

	if fb.Status.Usage == nil || fb.Status.CurrentPeriod == nil {
		return forecast
	}

	now := time.Now()
	periodStart := fb.Status.CurrentPeriod.StartDate.Time
	periodEnd := fb.Status.CurrentPeriod.EndDate.Time

	elapsed := now.Sub(periodStart)
	totalDuration := periodEnd.Sub(periodStart)

	if elapsed <= 0 || totalDuration <= 0 {
		return forecast
	}

	// Calculate daily burn rate
	elapsedDays := elapsed.Hours() / 24
	if elapsedDays <= 0 {
		elapsedDays = 1
	}

	dailyBurnCost := fb.Status.Usage.CostUSD / elapsedDays
	dailyBurnGPUHours := fb.Status.Usage.GPUHours / elapsedDays

	totalDays := totalDuration.Hours() / 24
	forecast.ProjectedCostUSD = dailyBurnCost * totalDays
	forecast.ProjectedGPUHours = dailyBurnGPUHours * totalDays

	// Check if budget is sufficient
	forecast.BudgetSufficient = forecast.ProjectedCostUSD <= fb.Spec.Limits.CostUSD

	// Estimate exhaustion date
	if dailyBurnCost > 0 && fb.Spec.Limits.CostUSD > 0 {
		remainingBudget := fb.Spec.Limits.CostUSD - fb.Status.Usage.CostUSD
		daysUntilExhaustion := remainingBudget / dailyBurnCost
		exhaustionDate := now.Add(time.Duration(daysUntilExhaustion*24) * time.Hour)
		exhaustionTime := metav1.NewTime(exhaustionDate)
		forecast.EstimatedExhaustionDate = &exhaustionTime
	}

	return forecast
}

func (r *GryviaBudgetReconciler) checkAndEmitAlert(ctx context.Context, fb *gryviav1.GryviaBudget, alert gryviav1.BudgetAlert, currentPercent float64) {
	logger := log.FromContext(ctx)

	// Check if this threshold was already alerted
	for _, existing := range fb.Status.Alerts {
		if existing.Threshold == alert.Threshold {
			return // Already alerted
		}
	}

	// Record the alert
	alertEvent := gryviav1.BudgetAlertEvent{
		Timestamp: metav1.Now(),
		Threshold: alert.Threshold,
		Message:   fmt.Sprintf("Budget %.0f%% consumed (threshold: %.0f%%)", currentPercent, alert.Threshold),
	}
	fb.Status.Alerts = append(fb.Status.Alerts, alertEvent)

	// Keep max 20 alerts
	if len(fb.Status.Alerts) > 20 {
		fb.Status.Alerts = fb.Status.Alerts[len(fb.Status.Alerts)-20:]
	}

	logger.Info("Budget threshold crossed",
		"scope", fb.Spec.Scope.Name,
		"threshold", alert.Threshold,
		"currentPercent", currentPercent,
	)
}

func (r *GryviaBudgetReconciler) enforceBudget(ctx context.Context, fb *gryviav1.GryviaBudget) error {
	logger := log.FromContext(ctx)

	if fb.Spec.Enforcement == nil || fb.Spec.Enforcement.Action != "block" {
		return nil
	}

	// Find pending jobs and reject them
	jobs, err := r.findScopeJobs(ctx, fb)
	if err != nil {
		return err
	}

	for i := range jobs {
		job := &jobs[i]
		if job.Status.Phase == "Pending" || job.Status.Phase == "Queued" {
			msg := fmt.Sprintf("Budget exceeded for %s %s", fb.Spec.Scope.Type, fb.Spec.Scope.Name)
			condition := metav1.Condition{
				Type:               "Rejected",
				Status:             metav1.ConditionTrue,
				Reason:             "BudgetExceeded",
				Message:            msg,
				LastTransitionTime: metav1.Now(),
			}
			if err := patchJobPhase(ctx, r.Client, job, "Rejected", msg, &condition); err != nil {
				logger.Error(err, "Failed to reject job", "job", job.Name)
				continue
			}
			logger.Info("Rejected job due to budget exceeded", "job", job.Name)
		}
	}

	return nil
}

func (r *GryviaBudgetReconciler) updateCondition(fb *gryviav1.GryviaBudget, condType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}
	meta.SetStatusCondition(&fb.Status.Conditions, condition)
}

// SetupWithManager sets up the controller with the Manager.
func (r *GryviaBudgetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gryviav1.GryviaBudget{}).
		Watches(&gryviav1.GryviaAIJob{}, handler.EnqueueRequestsFromMapFunc(
			func(ctx context.Context, obj client.Object) []reconcile.Request {
				budgetList := &gryviav1.GryviaBudgetList{}
				if err := mgr.GetClient().List(ctx, budgetList); err != nil {
					return nil
				}
				var requests []reconcile.Request
				for _, b := range budgetList.Items {
					requests = append(requests, reconcile.Request{
						NamespacedName: types.NamespacedName{
							Name: b.Name,
						},
					})
				}
				return requests
			},
		)).
		Complete(r)
}
