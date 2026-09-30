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

	gryviav1 "github.com/zyvorai/gryvia/operators/quota-operator/api/v1"
)

// GryviaChargebackReconciler reconciles a GryviaChargeback object
type GryviaChargebackReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=gryvia.io,resources=gryviachargebacks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gryvia.io,resources=gryviachargebacks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gryvia.io,resources=gryviachargebacks/finalizers,verbs=update
//+kubebuilder:rbac:groups=gryvia.io,resources=gryviaaijobs,verbs=get;list;watch
//+kubebuilder:rbac:groups=gryvia.io,resources=gryviaquotas,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *GryviaChargebackReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the GryviaChargeback instance
	chargeback := &gryviav1.GryviaChargeback{}
	err := r.Get(ctx, req.NamespacedName, chargeback)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("GryviaChargeback resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get GryviaChargeback")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !chargeback.ObjectMeta.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	logger.Info("Reconciling GryviaChargeback", "mode", chargeback.Spec.Mode, "period", chargeback.Spec.Period.Type)

	// Reconcile the chargeback
	result, err := r.reconcileChargeback(ctx, chargeback)
	if err != nil {
		logger.Error(err, "Failed to reconcile chargeback")
		r.updateCondition(chargeback, "Ready", metav1.ConditionFalse, "ReconcileFailed", err.Error())
		if statusErr := r.Status().Update(ctx, chargeback); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after reconcile failure")
		}
		return result, err
	}

	// Update status
	if err := r.Status().Update(ctx, chargeback); err != nil {
		logger.Error(err, "Failed to update GryviaChargeback status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

func (r *GryviaChargebackReconciler) reconcileChargeback(ctx context.Context, cb *gryviav1.GryviaChargeback) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Initialize current period
	currentPeriod := r.initializeCurrentPeriod(cb)

	// Calculate costs per cost center
	if err := r.calculateCostCenterCosts(ctx, cb, currentPeriod); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to calculate cost center costs: %w", err)
	}

	// Calculate total cost
	totalCost := 0.0
	for _, cc := range currentPeriod.CostCenters {
		totalCost += cc.Cost
	}

	// Apply platform overhead
	if cb.Spec.AllocationModel.IncludePlatformCosts && cb.Spec.AllocationModel.PlatformOverhead > 0 {
		platformCost := totalCost * (cb.Spec.AllocationModel.PlatformOverhead / 100.0)
		if currentPeriod.ByResourceType == nil {
			currentPeriod.ByResourceType = &gryviav1.ResourceTypeCosts{}
		}
		currentPeriod.ByResourceType.Platform = platformCost
		totalCost += platformCost
	}
	currentPeriod.TotalCost = totalCost

	cb.Status.CurrentPeriod = currentPeriod

	// Check if periodic report is due
	if cb.Spec.Reports != nil && cb.Spec.Reports.Enabled {
		r.checkReportGeneration(cb)
	}

	logger.Info("Chargeback reconciled", "totalCost", totalCost, "costCenters", len(currentPeriod.CostCenters))

	r.updateCondition(cb, "Ready", metav1.ConditionTrue, "ChargebackActive",
		fmt.Sprintf("Total cost: $%.2f for %d cost centers", totalCost, len(currentPeriod.CostCenters)))

	return ctrl.Result{}, nil
}

func (r *GryviaChargebackReconciler) initializeCurrentPeriod(cb *gryviav1.GryviaChargeback) *gryviav1.ChargebackCurrentPeriod {
	now := time.Now()
	period := &gryviav1.ChargebackCurrentPeriod{}

	switch cb.Spec.Period.Type {
	case "monthly":
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		end := start.AddDate(0, 1, 0).Add(-time.Second)
		period.StartDate = start.Format("2006-01-02")
		period.EndDate = end.Format("2006-01-02")
	case "quarterly":
		quarter := (int(now.Month()) - 1) / 3
		startMonth := time.Month(quarter*3 + 1)
		start := time.Date(now.Year(), startMonth, 1, 0, 0, 0, 0, now.Location())
		end := start.AddDate(0, 3, 0).Add(-time.Second)
		period.StartDate = start.Format("2006-01-02")
		period.EndDate = end.Format("2006-01-02")
	case "annual":
		start := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
		end := time.Date(now.Year(), 12, 31, 23, 59, 59, 0, now.Location())
		period.StartDate = start.Format("2006-01-02")
		period.EndDate = end.Format("2006-01-02")
	default:
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		end := start.AddDate(0, 1, 0).Add(-time.Second)
		period.StartDate = start.Format("2006-01-02")
		period.EndDate = end.Format("2006-01-02")
	}

	period.ByResourceType = &gryviav1.ResourceTypeCosts{}

	return period
}

func (r *GryviaChargebackReconciler) calculateCostCenterCosts(ctx context.Context, cb *gryviav1.GryviaChargeback, period *gryviav1.ChargebackCurrentPeriod) error {
	// Get all jobs to calculate costs
	jobList := &gryviav1.GryviaAIJobList{}
	if err := r.List(ctx, jobList); err != nil {
		return fmt.Errorf("failed to list GryviaAIJobs: %w", err)
	}

	// Find team-to-cost-center mapping
	teamToCostCenter := make(map[string]*gryviav1.CostCenter)
	for i := range cb.Spec.CostCenters {
		cc := &cb.Spec.CostCenters[i]
		for _, team := range cc.Teams {
			teamToCostCenter[team] = cc
		}
	}

	// Find team-to-namespace mapping via GryviaQuota
	quotaList := &gryviav1.GryviaQuotaList{}
	if err := r.List(ctx, quotaList); err != nil {
		return fmt.Errorf("failed to list GryviaQuotas: %w", err)
	}
	nsToTeam := make(map[string]string)
	for _, quota := range quotaList.Items {
		for _, ns := range quota.Spec.Namespaces {
			nsToTeam[ns] = quota.Spec.Team
		}
	}

	// Calculate per-cost-center costs
	costCenterCosts := make(map[string]float64)
	costCenterComputeCosts := make(map[string]float64)

	periodStart := time.Time{}
	if period.StartDate != "" {
		if t, err := time.Parse("2006-01-02", period.StartDate); err == nil {
			periodStart = t
		}
	}

	totalComputeCost := 0.0

	for _, job := range jobList.Items {
		if job.Status.StartTime == nil || job.Status.StartTime.IsZero() {
			continue
		}

		// Calculate GPU hours for this job in the current period
		effectiveStart := job.Status.StartTime.Time
		if effectiveStart.Before(periodStart) {
			effectiveStart = periodStart
		}

		var duration time.Duration
		if job.Status.CompletionTime == nil || job.Status.CompletionTime.IsZero() {
			duration = time.Since(effectiveStart)
		} else {
			duration = job.Status.CompletionTime.Time.Sub(effectiveStart)
		}

		if duration <= 0 {
			continue
		}

		hours := duration.Hours()
		gpuHours := hours * float64(job.Spec.TotalGPUs())

		// Get GPU rate from pricing config or default
		rate := r.getGPURate(cb, job.Spec.GpuType)
		computeCost := gpuHours * rate
		totalComputeCost += computeCost

		// Map job to cost center via namespace -> team -> cost center
		team, teamFound := nsToTeam[job.Namespace]
		if !teamFound {
			team = "unassigned"
		}

		cc, ccFound := teamToCostCenter[team]
		ccID := "unassigned"
		if ccFound {
			ccID = cc.ID
		}

		costCenterCosts[ccID] += computeCost
		costCenterComputeCosts[ccID] += computeCost
	}

	// Build cost center status entries
	for _, cc := range cb.Spec.CostCenters {
		cost := costCenterCosts[cc.ID]
		budgetAmount := 0.0
		if cc.Budget != nil {
			switch cb.Spec.Period.Type {
			case "monthly":
				budgetAmount = cc.Budget.Monthly
			case "quarterly":
				budgetAmount = cc.Budget.Quarterly
			case "annual":
				budgetAmount = cc.Budget.Annual
			default:
				budgetAmount = cc.Budget.Monthly
			}
		}

		percentUsed := 0.0
		if budgetAmount > 0 {
			percentUsed = (cost / budgetAmount) * 100
		}

		ccStatus := gryviav1.CostCenterStatus{
			ID:          cc.ID,
			Name:        cc.Name,
			Cost:        cost,
			Budget:      budgetAmount,
			Variance:    cost - budgetAmount,
			PercentUsed: percentUsed,
		}
		period.CostCenters = append(period.CostCenters, ccStatus)
	}

	period.ByResourceType.Compute = totalComputeCost

	return nil
}

func (r *GryviaChargebackReconciler) getGPURate(cb *gryviav1.GryviaChargeback, gpuType string) float64 {
	if cb.Spec.Pricing.GPURates != nil {
		if rate, ok := cb.Spec.Pricing.GPURates[gpuType]; ok {
			return rate.HourlyRate
		}
	}
	// Default rates
	defaults := map[string]float64{
		"H100":     32.00,
		"A100-80G": 24.00,
		"A100-40G": 18.00,
		"V100":     16.00,
		"T4":       8.00,
	}
	if rate, ok := defaults[gpuType]; ok {
		return rate
	}
	return 10.00
}

func (r *GryviaChargebackReconciler) checkReportGeneration(cb *gryviav1.GryviaChargeback) {
	if cb.Spec.Reports == nil || !cb.Spec.Reports.Enabled {
		return
	}

	// Check if a report needs to be generated based on frequency
	now := time.Now()
	generateReport := false

	if cb.Status.LastReport == nil || cb.Status.LastReport.Timestamp == nil {
		generateReport = true
	} else {
		lastReport := cb.Status.LastReport.Timestamp.Time
		switch cb.Spec.Reports.Frequency {
		case "daily":
			generateReport = now.Sub(lastReport) > 24*time.Hour
		case "weekly":
			generateReport = now.Sub(lastReport) > 7*24*time.Hour
		case "monthly":
			generateReport = now.Sub(lastReport) > 30*24*time.Hour
		case "quarterly":
			generateReport = now.Sub(lastReport) > 90*24*time.Hour
		}
	}

	if generateReport {
		reportTime := metav1.Now()
		cb.Status.LastReport = &gryviav1.ChargebackReportRef{
			Timestamp: &reportTime,
			Period:    fmt.Sprintf("%s-%s", cb.Status.CurrentPeriod.StartDate, cb.Status.CurrentPeriod.EndDate),
			Path:      fmt.Sprintf("/reports/chargeback/%s-%s.json", cb.Name, now.Format("2006-01")),
		}
	}
}

func (r *GryviaChargebackReconciler) updateCondition(cb *gryviav1.GryviaChargeback, condType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}
	meta.SetStatusCondition(&cb.Status.Conditions, condition)
}

// SetupWithManager sets up the controller with the Manager.
func (r *GryviaChargebackReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gryviav1.GryviaChargeback{}).
		Complete(r)
}
