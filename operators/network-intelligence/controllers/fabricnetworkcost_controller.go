package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	gryviav1 "github.com/zyvorai/gryvia/operators/network-intelligence/api/v1"
)

const (
	// defaultCostReportingInterval is the default interval between cost reports
	defaultCostReportingInterval = 1 * time.Hour

	// maxCostReports is the maximum number of reports to retain (30 days at hourly)
	maxCostReports = 720
)

// FabricNetworkCostReconciler reconciles a FabricNetworkCost object
type FabricNetworkCostReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=gryvia.io,resources=fabricnetworkcosts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gryvia.io,resources=fabricnetworkcosts/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gryvia.io,resources=fabricnetworkcosts/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch

func (r *FabricNetworkCostReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the FabricNetworkCost instance
	costTracker := &gryviav1.FabricNetworkCost{}
	if err := r.Get(ctx, req.NamespacedName, costTracker); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("FabricNetworkCost resource not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get FabricNetworkCost")
		return ctrl.Result{}, err
	}

	logger.Info("Reconciling FabricNetworkCost",
		"name", costTracker.Name,
		"namespaces", costTracker.Spec.TargetNamespaces,
	)

	// Parse reporting interval
	reportingInterval := defaultCostReportingInterval
	if costTracker.Spec.ReportingInterval != "" {
		if parsed, err := time.ParseDuration(costTracker.Spec.ReportingInterval); err == nil && parsed > 0 {
			reportingInterval = parsed
		}
	}

	// Query collector for per-namespace byte counters
	byteCounters := r.queryNetworkCosts(ctx, costTracker)

	// Build cost center lookup
	costCenterLookup := make(map[string]gryviav1.CostCenterMapping)
	for _, cc := range costTracker.Spec.CostCenters {
		costCenterLookup[cc.Namespace] = cc
	}

	// Generate cost reports per namespace
	var newReports []gryviav1.NetworkCostReport
	period := time.Now().UTC().Format("2006-01-02T15:04")

	for ns, counters := range byteCounters {
		team := ""
		if cc, ok := costCenterLookup[ns]; ok {
			team = cc.Team
		}

		// Apply cost model
		sameZoneCost := float64(counters.sameZoneBytes) / (1024 * 1024 * 1024) * costTracker.Spec.CostPerGB.SameZone
		crossZoneCost := float64(counters.crossZoneBytes) / (1024 * 1024 * 1024) * costTracker.Spec.CostPerGB.CrossZone
		externalCost := float64(counters.externalBytes) / (1024 * 1024 * 1024) * costTracker.Spec.CostPerGB.InternetEgress
		totalCost := sameZoneCost + crossZoneCost + externalCost

		report := gryviav1.NetworkCostReport{
			Period:         period,
			Namespace:      ns,
			Team:           team,
			SameZoneBytes:  counters.sameZoneBytes,
			CrossZoneBytes: counters.crossZoneBytes,
			ExternalBytes:  counters.externalBytes,
			TotalCostUSD:   totalCost,
		}
		newReports = append(newReports, report)
	}

	// Merge with existing reports, keeping last maxCostReports entries
	allReports := append(costTracker.Status.Reports, newReports...)
	if len(allReports) > maxCostReports {
		allReports = allReports[len(allReports)-maxCostReports:]
	}

	// Update status
	r.updateCostStatus(ctx, req.NamespacedName, "Active", allReports)

	logger.Info("FabricNetworkCost report generated",
		"newReports", len(newReports),
		"totalReports", len(allReports),
	)

	return ctrl.Result{RequeueAfter: reportingInterval}, nil
}

// namespaceByteCounts holds per-namespace traffic byte counters
type namespaceByteCounts struct {
	sameZoneBytes  int64
	crossZoneBytes int64
	externalBytes  int64
}

// collectorCostResponse represents the response from the collector cost API
type collectorCostResponse struct {
	Namespaces map[string]struct {
		SameZoneBytes  int64 `json:"sameZoneBytes"`
		CrossZoneBytes int64 `json:"crossZoneBytes"`
		ExternalBytes  int64 `json:"externalBytes"`
	} `json:"namespaces"`
}

// queryNetworkCosts fetches per-namespace byte counters from the collector API
func (r *FabricNetworkCostReconciler) queryNetworkCosts(ctx context.Context, costTracker *gryviav1.FabricNetworkCost) map[string]namespaceByteCounts {
	logger := log.FromContext(ctx)
	result := make(map[string]namespaceByteCounts)

	httpClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := httpClient.Get(fmt.Sprintf("%s/api/v1/network/costs", collectorBaseURL))
	if err != nil {
		logger.V(1).Info("Failed to query network costs from collector", "error", err)
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.V(1).Info("Collector returned non-OK for network costs", "statusCode", resp.StatusCode)
		return result
	}

	var costResp collectorCostResponse
	if err := json.NewDecoder(resp.Body).Decode(&costResp); err != nil {
		logger.Error(err, "Failed to decode network costs response")
		return result
	}

	// Filter to target namespaces
	targetNS := make(map[string]bool)
	for _, ns := range costTracker.Spec.TargetNamespaces {
		targetNS[ns] = true
	}

	for ns, counts := range costResp.Namespaces {
		if len(targetNS) > 0 && !targetNS[ns] {
			continue
		}
		result[ns] = namespaceByteCounts{
			sameZoneBytes:  counts.SameZoneBytes,
			crossZoneBytes: counts.CrossZoneBytes,
			externalBytes:  counts.ExternalBytes,
		}
	}

	return result
}

// updateCostStatus updates the FabricNetworkCost status subresource
func (r *FabricNetworkCostReconciler) updateCostStatus(ctx context.Context, namespacedName types.NamespacedName, phase string, reports []gryviav1.NetworkCostReport) {
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		costTracker := &gryviav1.FabricNetworkCost{}
		if err := r.Get(ctx, namespacedName, costTracker); err != nil {
			return err
		}
		costTracker.Status.Phase = phase
		costTracker.Status.Reports = reports
		costTracker.Status.LastReport = metav1.Now()
		return r.Status().Update(ctx, costTracker)
	}); err != nil {
		log.FromContext(ctx).Error(err, "Failed to update FabricNetworkCost status")
	}
}

// SetupWithManager sets up the controller with the Manager
func (r *FabricNetworkCostReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gryviav1.FabricNetworkCost{}).
		Complete(r)
}
