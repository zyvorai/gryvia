package budget

import (
	"context"
	"sync"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	kubefabricv1 "github.com/yourusername/kubefabric/operators/quota-operator/api/v1"
	"github.com/yourusername/kubefabric/operators/quota-operator/pkg/usage"
)

// gpuPricingMu protects concurrent access to gpuPricing
var gpuPricingMu sync.RWMutex

// gpuPricing contains GPU hourly rates in USD
var gpuPricing = map[string]float64{
	"H100":     8.00, // $8/hour
	"A100-80G": 4.00, // $4/hour
	"A100-40G": 3.00, // $3/hour
	"L40":      2.50, // $2.50/hour
	"A10":      1.50, // $1.50/hour
	"V100":     2.00, // $2/hour
	"T4":       0.75, // $0.75/hour
	"default":  2.00, // Default rate if type unknown
}

// CalculateBudget calculates budget status for a quota
func CalculateBudget(ctx context.Context, k8sClient client.Client, quota *kubefabricv1.FabricQuota, currentUsage *kubefabricv1.QuotaUsage) (*kubefabricv1.BudgetStatus, error) {
	budgetStatus := &kubefabricv1.BudgetStatus{}

	if quota.Spec.Budget == nil {
		return budgetStatus, nil
	}

	// Get total GPU hours this month
	monthlyGPUHours, err := usage.GetMonthlyGPUHours(ctx, k8sClient, quota)
	if err != nil {
		return nil, err
	}

	// Calculate cost based on GPU types used
	avgRate := calculateAverageRate(quota)
	spentThisMonth := monthlyGPUHours * avgRate

	budgetStatus.SpentThisMonth = spentThisMonth
	budgetStatus.RemainingBudget = quota.Spec.Budget.MonthlyBudget - spentThisMonth
	budgetStatus.PercentUsed = (spentThisMonth / quota.Spec.Budget.MonthlyBudget) * 100

	// Project end-of-month spending
	now := time.Now()
	daysInMonth := float64(daysInCurrentMonth())
	dayOfMonth := float64(now.Day())
	dailyBurn := spentThisMonth / dayOfMonth
	budgetStatus.ProjectedSpend = dailyBurn * daysInMonth

	return budgetStatus, nil
}

func calculateAverageRate(quota *kubefabricv1.FabricQuota) float64 {
	gpuPricingMu.RLock()
	defer gpuPricingMu.RUnlock()

	if len(quota.Spec.GPUQuota.AllowedGPUTypes) == 0 {
		return gpuPricing["default"]
	}

	totalRate := 0.0
	count := 0
	for _, gpuType := range quota.Spec.GPUQuota.AllowedGPUTypes {
		if rate, exists := gpuPricing[gpuType]; exists {
			totalRate += rate
			count++
		}
	}

	if count == 0 {
		return gpuPricing["default"]
	}

	return totalRate / float64(count)
}

func daysInCurrentMonth() int {
	now := time.Now()
	year, month, _ := now.Date()
	// Get last day of month by going to first day of next month and subtracting 1 day
	firstOfNextMonth := time.Date(year, month+1, 1, 0, 0, 0, 0, now.Location())
	lastOfThisMonth := firstOfNextMonth.Add(-24 * time.Hour)
	return lastOfThisMonth.Day()
}

// GetGPURate returns the hourly rate for a GPU type
func GetGPURate(gpuType string) float64 {
	gpuPricingMu.RLock()
	defer gpuPricingMu.RUnlock()

	if rate, exists := gpuPricing[gpuType]; exists {
		return rate
	}
	return gpuPricing["default"]
}

// UpdatePricing allows updating GPU pricing (useful for custom on-prem pricing)
func UpdatePricing(gpuType string, hourlyRate float64) {
	gpuPricingMu.Lock()
	defer gpuPricingMu.Unlock()
	gpuPricing[gpuType] = hourlyRate
}
