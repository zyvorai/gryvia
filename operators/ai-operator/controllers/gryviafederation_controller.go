package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gryviav1 "github.com/zyvorai/gryvia/operators/ai-operator/api/v1"
)

// GryviaFederationReconciler reconciles a GryviaFederation object
type GryviaFederationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

//+kubebuilder:rbac:groups=gryvia.io,resources=gryviafederations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gryvia.io,resources=gryviafederations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gryvia.io,resources=gryviafederations/finalizers,verbs=update
//+kubebuilder:rbac:groups=gryvia.io,resources=gryviaaijobs,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *GryviaFederationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("gryviafederation", req.NamespacedName)

	// Fetch the GryviaFederation instance
	federation := &gryviav1.GryviaFederation{}
	err := r.Get(ctx, req.NamespacedName, federation)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("GryviaFederation resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get GryviaFederation")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !federation.ObjectMeta.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	log.Info("Reconciling GryviaFederation", "clusters", len(federation.Spec.Clusters))

	// Reconcile the federation
	result, err := r.reconcileFederation(ctx, federation)
	if err != nil {
		log.Error(err, "Failed to reconcile federation")
		return result, err
	}

	return result, nil
}

func (r *GryviaFederationReconciler) reconcileFederation(ctx context.Context, federation *gryviav1.GryviaFederation) (ctrl.Result, error) {
	log := r.Log.WithValues("federation", federation.Name)

	// Health check all member clusters
	clusterStatuses := r.healthCheckClusters(ctx, federation)
	federation.Status.ClusterStatus = clusterStatuses

	// Calculate aggregate stats
	federation.Status.AggregateStats = r.calculateAggregateStats(federation)

	// Calculate job distribution
	federation.Status.JobDistribution = r.calculateJobDistribution(ctx, federation)

	// Determine overall federation state
	federation.Status.State = r.determineFederationState(clusterStatuses)

	// Update condition
	r.updateFederationCondition(federation, "Ready", metav1.ConditionTrue, "FederationHealthy",
		fmt.Sprintf("Federation %s: %d clusters, %d total GPUs",
			federation.Status.State,
			len(clusterStatuses),
			federation.Status.AggregateStats.TotalGPUs))

	// Update status
	if err := r.Status().Update(ctx, federation); err != nil {
		log.Error(err, "Failed to update federation status")
		return ctrl.Result{}, err
	}

	// Requeue based on health check interval
	requeueAfter := 30 * time.Second
	if federation.Spec.Failover != nil && federation.Spec.Failover.HealthCheck != nil {
		if interval, err := time.ParseDuration(federation.Spec.Failover.HealthCheck.Interval); err == nil {
			requeueAfter = interval
		}
	}

	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

func (r *GryviaFederationReconciler) healthCheckClusters(ctx context.Context, federation *gryviav1.GryviaFederation) []gryviav1.FederationClusterStatus {
	var statuses []gryviav1.FederationClusterStatus

	for _, cluster := range federation.Spec.Clusters {
		if !cluster.Enabled {
			continue
		}

		status := gryviav1.FederationClusterStatus{
			Name: cluster.Name,
		}

		// Perform health check
		// In production, this would use the cluster's API server endpoint
		// and credentials to verify connectivity
		healthy := r.checkClusterHealth(ctx, cluster)

		now := metav1.Now()
		status.LastHealthCheck = &now

		if healthy {
			status.State = "healthy"
		} else {
			status.State = "unhealthy"
		}

		// Calculate utilization from capacity
		status.Utilization = r.calculateClusterUtilization(cluster)

		statuses = append(statuses, status)
	}

	return statuses
}

func (r *GryviaFederationReconciler) checkClusterHealth(ctx context.Context, cluster gryviav1.FederationCluster) bool {
	// In a production implementation, this would:
	// 1. Load kubeconfig from the referenced secret
	// 2. Create a client for the remote cluster
	// 3. Perform a health check (e.g., GET /healthz)
	// 4. Return true if the cluster is reachable and healthy

	// For now, check that the API server URL is configured
	if cluster.APIServer == "" {
		return false
	}

	// Assume healthy if configured
	return true
}

func (r *GryviaFederationReconciler) calculateClusterUtilization(cluster gryviav1.FederationCluster) *gryviav1.FederationUtilization {
	if cluster.Capacity == nil {
		return nil
	}

	totalGPUs := 0
	availableGPUs := 0
	for _, gpuType := range cluster.Capacity.GPUTypes {
		totalGPUs += gpuType.Total
		availableGPUs += gpuType.Available
	}

	usedGPUs := totalGPUs - availableGPUs
	percentage := 0.0
	if totalGPUs > 0 {
		percentage = float64(usedGPUs) / float64(totalGPUs) * 100
	}

	return &gryviav1.FederationUtilization{
		GPUs:       usedGPUs,
		Percentage: percentage,
	}
}

func (r *GryviaFederationReconciler) calculateAggregateStats(federation *gryviav1.GryviaFederation) *gryviav1.FederationAggregateStats {
	stats := &gryviav1.FederationAggregateStats{}

	for _, cluster := range federation.Spec.Clusters {
		if !cluster.Enabled || cluster.Capacity == nil {
			continue
		}

		for _, gpuType := range cluster.Capacity.GPUTypes {
			stats.TotalGPUs += gpuType.Total
			stats.AvailableGPUs += gpuType.Available
		}

		// Calculate cost per hour based on used GPUs and pricing
		if cluster.Pricing != nil {
			for _, gpuType := range cluster.Capacity.GPUTypes {
				usedGPUs := gpuType.Total - gpuType.Available
				if rate, ok := cluster.Pricing.GPUHourlyRates[gpuType.Type]; ok {
					stats.TotalCostPerHour += float64(usedGPUs) * rate
				}
			}
		}
	}

	return stats
}

func (r *GryviaFederationReconciler) calculateJobDistribution(ctx context.Context, federation *gryviav1.GryviaFederation) map[string]int {
	distribution := make(map[string]int)

	// List all jobs in the local cluster
	jobList := &gryviav1.GryviaAIJobList{}
	if err := r.List(ctx, jobList); err != nil {
		return distribution
	}

	// Count jobs per cluster (using labels or annotations)
	localJobs := 0
	for _, job := range jobList.Items {
		if job.Status.Phase == "Running" {
			localJobs++
		}
	}

	// For the local cluster, use the first cluster name
	if len(federation.Spec.Clusters) > 0 {
		distribution[federation.Spec.Clusters[0].Name] = localJobs
	}

	// In production, remote cluster job counts would be fetched via cross-cluster API calls

	return distribution
}

func (r *GryviaFederationReconciler) determineFederationState(statuses []gryviav1.FederationClusterStatus) string {
	if len(statuses) == 0 {
		return "unavailable"
	}

	healthyCount := 0
	for _, status := range statuses {
		if status.State == "healthy" {
			healthyCount++
		}
	}

	if healthyCount == len(statuses) {
		return "healthy"
	}
	if healthyCount == 0 {
		return "unavailable"
	}
	return "degraded"
}

func (r *GryviaFederationReconciler) updateFederationCondition(federation *gryviav1.GryviaFederation, condType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: federation.Generation,
		LastTransitionTime: metav1.Now(),
	}

	found := false
	for i, cond := range federation.Status.Conditions {
		if cond.Type == condType {
			federation.Status.Conditions[i] = condition
			found = true
			break
		}
	}
	if !found {
		federation.Status.Conditions = append(federation.Status.Conditions, condition)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *GryviaFederationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gryviav1.GryviaFederation{}).
		Complete(r)
}
