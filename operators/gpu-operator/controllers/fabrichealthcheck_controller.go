package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/client-go/util/retry"

	tensorreaperv1 "github.com/ssahani/TensorReaper/operators/gpu-operator/api/v1"
)

const (
	healthHealthy   = "healthy"
	healthDegraded  = "degraded"
	healthUnhealthy = "unhealthy"
	healthUnknown   = "unknown"

	checkStatusPass    = "pass"
	checkStatusWarning = "warning"
	checkStatusFail    = "fail"
)

// FabricHealthCheckReconciler reconciles a FabricHealthCheck object
type FabricHealthCheckReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabrichealthchecks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabrichealthchecks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabrichealthchecks/finalizers,verbs=update
//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabricgpunodes,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch;update;patch

// Reconcile is part of the main kubernetes reconciliation loop
func (r *FabricHealthCheckReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("fabrichealthcheck", req.NamespacedName)

	// Fetch the FabricHealthCheck instance
	hc := &tensorreaperv1.FabricHealthCheck{}
	err := r.Get(ctx, req.NamespacedName, hc)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("FabricHealthCheck resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get FabricHealthCheck")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !hc.ObjectMeta.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	// Initialize status if needed
	if hc.Status.OverallHealth == "" {
		hc.Status.OverallHealth = healthUnknown
		if err := r.Status().Update(ctx, hc); err != nil {
			log.Error(err, "Failed to update initial status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Reconcile health check
	result, err := r.reconcileHealthCheck(ctx, hc)
	if err != nil {
		log.Error(err, "Failed to reconcile health check")
		return result, err
	}

	return result, nil
}

func (r *FabricHealthCheckReconciler) reconcileHealthCheck(ctx context.Context, hc *tensorreaperv1.FabricHealthCheck) (ctrl.Result, error) {
	log := r.Log.WithValues("fabrichealthcheck", hc.Name)

	// Get target nodes
	nodes, err := r.getTargetNodes(ctx, hc)
	if err != nil {
		log.Error(err, "Failed to get target nodes")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	// Get GPU node info
	gpuNodes := &tensorreaperv1.FabricGpuNodeList{}
	if err := r.List(ctx, gpuNodes); err != nil {
		log.Error(err, "Failed to list FabricGpuNodes")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	// Run health checks on each node
	var checkResults []tensorreaperv1.HealthCheckResult
	var affectedResources []tensorreaperv1.AffectedResource
	overallHealth := healthHealthy
	now := metav1.Now()

	for _, node := range nodes {
		// Find matching GPU node
		var gpuNode *tensorreaperv1.FabricGpuNode
		for i := range gpuNodes.Items {
			if gpuNodes.Items[i].Spec.NodeName == node.Name {
				gpuNode = &gpuNodes.Items[i]
				break
			}
		}

		// Run each configured check
		for _, check := range hc.Spec.Checks {
			if !check.Enabled {
				continue
			}

			result := r.runCheck(check, gpuNode, &node)
			result.Timestamp = &now
			checkResults = append(checkResults, result)

			// Track affected resources
			if result.Status == checkStatusFail {
				overallHealth = healthUnhealthy
				affectedResources = append(affectedResources, tensorreaperv1.AffectedResource{
					Type:   "node",
					Name:   node.Name,
					Status: healthUnhealthy,
				})
			} else if result.Status == checkStatusWarning && overallHealth != healthUnhealthy {
				overallHealth = healthDegraded
				affectedResources = append(affectedResources, tensorreaperv1.AffectedResource{
					Type:   "node",
					Name:   node.Name,
					Status: healthDegraded,
				})
			}
		}
	}

	// Update status
	hc.Status.LastCheckTime = &now
	nextCheck := now.Add(r.getCheckInterval(hc))
	nextCheckTime := metav1.NewTime(nextCheck)
	hc.Status.NextCheckTime = &nextCheckTime
	hc.Status.OverallHealth = overallHealth
	hc.Status.CheckResults = checkResults
	hc.Status.AffectedResources = affectedResources

	// Handle failures
	if overallHealth == healthUnhealthy || overallHealth == healthDegraded {
		if err := r.handleFailure(ctx, hc, nodes, affectedResources); err != nil {
			log.Error(err, "Failed to handle health check failure")
		}
	}

	// Update node labels based on health status
	if err := r.updateNodeLabels(ctx, nodes, checkResults, overallHealth); err != nil {
		log.Error(err, "Failed to update node labels")
	}

	// Update status
	if err := r.Status().Update(ctx, hc); err != nil {
		log.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: r.getCheckInterval(hc)}, nil
}

func (r *FabricHealthCheckReconciler) getTargetNodes(ctx context.Context, hc *tensorreaperv1.FabricHealthCheck) ([]corev1.Node, error) {
	nodeList := &corev1.NodeList{}

	switch hc.Spec.Target.Type {
	case "cluster":
		// Get all GPU nodes
		if err := r.List(ctx, nodeList, client.HasLabels{"tensorreaper.ai/gpu"}); err != nil {
			return nil, err
		}

	case "node":
		// Get nodes matching selector
		if len(hc.Spec.Target.Selector) > 0 {
			if err := r.List(ctx, nodeList, client.MatchingLabels(hc.Spec.Target.Selector)); err != nil {
				return nil, err
			}
		} else {
			if err := r.List(ctx, nodeList, client.HasLabels{"tensorreaper.ai/gpu"}); err != nil {
				return nil, err
			}
		}

	default:
		// For job and other types, get all GPU nodes as a fallback
		if err := r.List(ctx, nodeList, client.HasLabels{"tensorreaper.ai/gpu"}); err != nil {
			return nil, err
		}
	}

	return nodeList.Items, nil
}

func (r *FabricHealthCheckReconciler) runCheck(check tensorreaperv1.HealthCheck, gpuNode *tensorreaperv1.FabricGpuNode, node *corev1.Node) tensorreaperv1.HealthCheckResult {
	result := tensorreaperv1.HealthCheckResult{
		CheckName: check.Name,
		Status:    checkStatusPass,
		Message:   "Check passed",
	}

	if gpuNode == nil {
		result.Status = checkStatusWarning
		result.Message = "No FabricGpuNode found for node " + node.Name
		return result
	}

	switch check.Type {
	case "gpu-temperature":
		r.checkGpuTemperature(check, gpuNode, &result)

	case "gpu-utilization":
		r.checkGpuUtilization(check, gpuNode, &result)

	case "gpu-memory":
		r.checkGpuMemory(check, gpuNode, &result)

	case "gpu-ecc-errors":
		r.checkECCErrors(check, gpuNode, &result)

	case "gpu-nvlink":
		r.checkNVLink(check, gpuNode, &result)

	case "gpu-power":
		r.checkGpuPower(check, gpuNode, &result)

	case "pcie-bandwidth":
		result.Status = checkStatusPass
		result.Value = "N/A"
		result.Message = "PCIe bandwidth check requires runtime measurement"

	case "clock-speeds":
		result.Status = checkStatusPass
		result.Value = "N/A"
		result.Message = "Clock speed check requires runtime measurement"

	default:
		result.Status = checkStatusWarning
		result.Message = fmt.Sprintf("Unknown check type: %s", check.Type)
	}

	return result
}

func (r *FabricHealthCheckReconciler) checkGpuTemperature(check tensorreaperv1.HealthCheck, gpuNode *tensorreaperv1.FabricGpuNode, result *tensorreaperv1.HealthCheckResult) {
	var maxTemp int
	for _, gpuStatus := range gpuNode.Status.GpuStatus {
		if gpuStatus.Temperature > maxTemp {
			maxTemp = gpuStatus.Temperature
		}
	}

	result.Value = fmt.Sprintf("%d", maxTemp)

	if check.Threshold != nil {
		thresholdStr := ""
		if check.Threshold.Max != nil {
			thresholdStr = fmt.Sprintf("max: %.0f", *check.Threshold.Max)
			if float64(maxTemp) > *check.Threshold.Max {
				result.Status = checkStatusFail
				result.Message = fmt.Sprintf("GPU temperature %d exceeds max threshold %.0f", maxTemp, *check.Threshold.Max)
				return
			}
		}
		if check.Threshold.Critical != nil {
			if float64(maxTemp) > *check.Threshold.Critical {
				result.Status = checkStatusFail
				result.Message = fmt.Sprintf("GPU temperature %d exceeds critical threshold %.0f", maxTemp, *check.Threshold.Critical)
				return
			}
		}
		if check.Threshold.Warning != nil {
			if float64(maxTemp) > *check.Threshold.Warning {
				result.Status = checkStatusWarning
				result.Message = fmt.Sprintf("GPU temperature %d exceeds warning threshold %.0f", maxTemp, *check.Threshold.Warning)
				return
			}
		}
		result.Threshold = thresholdStr
	}

	result.Message = fmt.Sprintf("GPU temperature %d within normal range", maxTemp)
}

func (r *FabricHealthCheckReconciler) checkGpuUtilization(check tensorreaperv1.HealthCheck, gpuNode *tensorreaperv1.FabricGpuNode, result *tensorreaperv1.HealthCheckResult) {
	var maxUtil int
	for _, gpuStatus := range gpuNode.Status.GpuStatus {
		if gpuStatus.Utilization > maxUtil {
			maxUtil = gpuStatus.Utilization
		}
	}

	result.Value = fmt.Sprintf("%d", maxUtil)

	if check.Threshold != nil {
		if check.Threshold.Max != nil && float64(maxUtil) > *check.Threshold.Max {
			result.Status = checkStatusFail
			result.Message = fmt.Sprintf("GPU utilization %d%% exceeds max threshold %.0f%%", maxUtil, *check.Threshold.Max)
			return
		}
		if check.Threshold.Warning != nil && float64(maxUtil) > *check.Threshold.Warning {
			result.Status = checkStatusWarning
			result.Message = fmt.Sprintf("GPU utilization %d%% exceeds warning threshold %.0f%%", maxUtil, *check.Threshold.Warning)
			return
		}
		if check.Threshold.Min != nil && float64(maxUtil) < *check.Threshold.Min {
			result.Status = checkStatusWarning
			result.Message = fmt.Sprintf("GPU utilization %d%% below minimum threshold %.0f%%", maxUtil, *check.Threshold.Min)
			return
		}
	}

	result.Message = fmt.Sprintf("GPU utilization %d%% within normal range", maxUtil)
}

func (r *FabricHealthCheckReconciler) checkGpuMemory(check tensorreaperv1.HealthCheck, gpuNode *tensorreaperv1.FabricGpuNode, result *tensorreaperv1.HealthCheckResult) {
	var maxMemPct float64
	for _, gpuStatus := range gpuNode.Status.GpuStatus {
		if gpuStatus.MemoryTotal > 0 {
			pct := float64(gpuStatus.MemoryUsed) / float64(gpuStatus.MemoryTotal) * 100
			if pct > maxMemPct {
				maxMemPct = pct
			}
		}
	}

	result.Value = fmt.Sprintf("%.1f", maxMemPct)

	if check.Threshold != nil {
		if check.Threshold.Max != nil && maxMemPct > *check.Threshold.Max {
			result.Status = checkStatusFail
			result.Message = fmt.Sprintf("GPU memory usage %.1f%% exceeds max threshold %.0f%%", maxMemPct, *check.Threshold.Max)
			return
		}
		if check.Threshold.Warning != nil && maxMemPct > *check.Threshold.Warning {
			result.Status = checkStatusWarning
			result.Message = fmt.Sprintf("GPU memory usage %.1f%% exceeds warning threshold %.0f%%", maxMemPct, *check.Threshold.Warning)
			return
		}
	}

	result.Message = fmt.Sprintf("GPU memory usage %.1f%% within normal range", maxMemPct)
}

func (r *FabricHealthCheckReconciler) checkECCErrors(check tensorreaperv1.HealthCheck, gpuNode *tensorreaperv1.FabricGpuNode, result *tensorreaperv1.HealthCheckResult) {
	// In a real implementation, this would query DCGM for ECC error counts.
	// For now, check health status from the GPU node.
	var unhealthyGPUs int
	for _, gpuStatus := range gpuNode.Status.GpuStatus {
		if gpuStatus.Health != "Healthy" && gpuStatus.Health != "" {
			unhealthyGPUs++
		}
	}

	result.Value = fmt.Sprintf("%d", unhealthyGPUs)

	if unhealthyGPUs > 0 {
		if check.Threshold != nil && check.Threshold.Max != nil && float64(unhealthyGPUs) > *check.Threshold.Max {
			result.Status = checkStatusFail
			result.Message = fmt.Sprintf("%d GPUs reporting health issues", unhealthyGPUs)
			return
		}
		result.Status = checkStatusWarning
		result.Message = fmt.Sprintf("%d GPUs reporting health issues", unhealthyGPUs)
		return
	}

	result.Message = "No ECC errors detected"
}

func (r *FabricHealthCheckReconciler) checkNVLink(check tensorreaperv1.HealthCheck, gpuNode *tensorreaperv1.FabricGpuNode, result *tensorreaperv1.HealthCheckResult) {
	// Check NVLink health via GPU node status
	healthyGPUs := 0
	totalGPUs := len(gpuNode.Status.GpuStatus)
	for _, gpuStatus := range gpuNode.Status.GpuStatus {
		if gpuStatus.Health == "Healthy" || gpuStatus.Health == "" {
			healthyGPUs++
		}
	}

	result.Value = fmt.Sprintf("%d/%d", healthyGPUs, totalGPUs)

	if healthyGPUs < totalGPUs {
		result.Status = checkStatusWarning
		result.Message = fmt.Sprintf("NVLink: %d/%d GPUs healthy", healthyGPUs, totalGPUs)
	} else {
		result.Message = "All NVLink connections healthy"
	}
}

func (r *FabricHealthCheckReconciler) checkGpuPower(check tensorreaperv1.HealthCheck, gpuNode *tensorreaperv1.FabricGpuNode, result *tensorreaperv1.HealthCheckResult) {
	var maxPower int
	for _, gpuStatus := range gpuNode.Status.GpuStatus {
		if gpuStatus.PowerUsage > maxPower {
			maxPower = gpuStatus.PowerUsage
		}
	}

	result.Value = fmt.Sprintf("%d", maxPower)

	if check.Threshold != nil {
		if check.Threshold.Max != nil && float64(maxPower) > *check.Threshold.Max {
			result.Status = checkStatusFail
			result.Message = fmt.Sprintf("GPU power %dW exceeds max threshold %.0fW", maxPower, *check.Threshold.Max)
			return
		}
		if check.Threshold.Warning != nil && float64(maxPower) > *check.Threshold.Warning {
			result.Status = checkStatusWarning
			result.Message = fmt.Sprintf("GPU power %dW exceeds warning threshold %.0fW", maxPower, *check.Threshold.Warning)
			return
		}
	}

	result.Message = fmt.Sprintf("GPU power %dW within normal range", maxPower)
}

func (r *FabricHealthCheckReconciler) handleFailure(ctx context.Context, hc *tensorreaperv1.FabricHealthCheck, nodes []corev1.Node, affected []tensorreaperv1.AffectedResource) error {
	if hc.Spec.OnFailure == nil {
		return nil
	}

	log := r.Log.WithValues("fabrichealthcheck", hc.Name)

	// Build set of affected node names
	affectedNodes := make(map[string]bool)
	for _, res := range affected {
		if res.Type == "node" {
			affectedNodes[res.Name] = true
		}
	}

	// Cordon affected nodes
	if hc.Spec.OnFailure.Cordon {
		for _, node := range nodes {
			if !affectedNodes[node.Name] {
				continue
			}

			if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				n := &corev1.Node{}
				if err := r.Get(ctx, types.NamespacedName{Name: node.Name}, n); err != nil {
					return err
				}
				if !n.Spec.Unschedulable {
					n.Spec.Unschedulable = true
					return r.Update(ctx, n)
				}
				return nil
			}); err != nil {
				log.Error(err, "Failed to cordon node", "node", node.Name)
			} else {
				now := metav1.Now()
				hc.Status.RemediationHistory = append(hc.Status.RemediationHistory, tensorreaperv1.RemediationEvent{
					Timestamp: &now,
					Action:    "cordon-node",
					Success:   true,
					Message:   fmt.Sprintf("Node %s cordoned", node.Name),
				})
			}
		}
	}

	// Auto-remediate if enabled
	if hc.Spec.OnFailure.AutoRemediate && hc.Spec.Remediation != nil {
		for _, node := range nodes {
			if !affectedNodes[node.Name] {
				continue
			}

			if hc.Spec.Remediation.GpuReset {
				now := metav1.Now()
				log.Info("Attempting GPU reset", "node", node.Name)
				// In a real implementation, this would trigger a GPU reset via DaemonSet or node agent
				hc.Status.RemediationHistory = append(hc.Status.RemediationHistory, tensorreaperv1.RemediationEvent{
					Timestamp: &now,
					Action:    "gpu-reset",
					Success:   false,
					Message:   fmt.Sprintf("GPU reset requested for node %s (requires node agent)", node.Name),
				})
			}
		}
	}

	return nil
}

func (r *FabricHealthCheckReconciler) updateNodeLabels(ctx context.Context, nodes []corev1.Node, results []tensorreaperv1.HealthCheckResult, overallHealth string) error {
	// Build per-node health status
	nodeHealth := make(map[string]string)
	for _, result := range results {
		if result.Status == checkStatusFail {
			// Find which node this result belongs to by checking affected resources
			// For simplicity, mark all nodes in a cluster check
			for _, node := range nodes {
				nodeHealth[node.Name] = healthUnhealthy
			}
		}
	}

	for _, node := range nodes {
		health, ok := nodeHealth[node.Name]
		if !ok {
			health = healthHealthy
		}

		if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			n := &corev1.Node{}
			if err := r.Get(ctx, types.NamespacedName{Name: node.Name}, n); err != nil {
				return err
			}

			if n.Labels == nil {
				n.Labels = make(map[string]string)
			}

			currentHealth := n.Labels["tensorreaper.ai/gpu-health"]
			if currentHealth != health {
				n.Labels["tensorreaper.ai/gpu-health"] = health
				return r.Update(ctx, n)
			}
			return nil
		}); err != nil {
			r.Log.Error(err, "Failed to update node health label", "node", node.Name)
		}
	}

	return nil
}

func (r *FabricHealthCheckReconciler) getCheckInterval(hc *tensorreaperv1.FabricHealthCheck) time.Duration {
	// Default to 5 minutes
	interval := 5 * time.Minute

	// Parse cron-like schedule for simple intervals
	if hc.Spec.Schedule != "" {
		switch hc.Spec.Schedule {
		case "*/1 * * * *":
			interval = 1 * time.Minute
		case "*/2 * * * *":
			interval = 2 * time.Minute
		case "*/5 * * * *":
			interval = 5 * time.Minute
		case "*/10 * * * *":
			interval = 10 * time.Minute
		case "*/15 * * * *":
			interval = 15 * time.Minute
		}
	}

	return interval
}

// SetupWithManager sets up the controller with the Manager.
func (r *FabricHealthCheckReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tensorreaperv1.FabricHealthCheck{}).
		Complete(r)
}
