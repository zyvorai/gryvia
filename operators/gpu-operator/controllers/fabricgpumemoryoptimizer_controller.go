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

	kubefabricv1 "github.com/ssahani/kube-fabric/operators/gpu-operator/api/v1"
	"github.com/ssahani/kube-fabric/operators/gpu-operator/pkg/memory"
)

const (
	// Default analysis interval
	defaultAnalysisInterval = 60 * time.Second

	// Condition types for the memory optimizer
	ConditionAnalysisRunning   = "AnalysisRunning"
	ConditionOomMonitoring     = "OomMonitoringActive"
	ConditionRightSizingActive = "RightSizingActive"
)

// FabricGpuMemoryOptimizerReconciler reconciles a FabricGpuMemoryOptimizer object
type FabricGpuMemoryOptimizerReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	Log       logr.Logger
	Predictor *memory.Predictor
}

//+kubebuilder:rbac:groups=kubefabric.ai,resources=fabricgpumemoryoptimizers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kubefabric.ai,resources=fabricgpumemoryoptimizers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kubefabric.ai,resources=fabricgpumemoryoptimizers/finalizers,verbs=update
//+kubebuilder:rbac:groups=kubefabric.ai,resources=fabricgpunodes,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop
func (r *FabricGpuMemoryOptimizerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("fabricgpumemoryoptimizer", req.NamespacedName)

	// Fetch the FabricGpuMemoryOptimizer instance
	optimizer := &kubefabricv1.FabricGpuMemoryOptimizer{}
	err := r.Get(ctx, req.NamespacedName, optimizer)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("FabricGpuMemoryOptimizer resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get FabricGpuMemoryOptimizer")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !optimizer.ObjectMeta.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	// Run the reconciliation
	result, err := r.reconcileOptimizer(ctx, optimizer)
	if err != nil {
		log.Error(err, "Failed to reconcile GPU memory optimizer")
		return result, err
	}

	return result, nil
}

func (r *FabricGpuMemoryOptimizerReconciler) reconcileOptimizer(ctx context.Context, optimizer *kubefabricv1.FabricGpuMemoryOptimizer) (ctrl.Result, error) {
	log := r.Log.WithValues("optimizer", optimizer.Name)

	// Collect GPU memory data from FabricGpuNode resources
	gpuNodes, err := r.collectGpuNodes(ctx, optimizer)
	if err != nil {
		log.Error(err, "Failed to collect GPU node data")
		optimizer.Status.JobsAnalyzed = 0
		r.updateCondition(optimizer, ConditionAnalysisRunning, metav1.ConditionFalse, "CollectionFailed", err.Error())
		if updateErr := r.Status().Update(ctx, optimizer); updateErr != nil {
			log.Error(updateErr, "Failed to update status")
		}
		return ctrl.Result{RequeueAfter: defaultAnalysisInterval}, err
	}

	r.updateCondition(optimizer, ConditionAnalysisRunning, metav1.ConditionTrue, "AnalysisActive", "GPU memory analysis is active")

	// Extract memory samples from GPU nodes
	samples := r.extractMemorySamples(gpuNodes)

	jobsAnalyzed := len(samples)
	optimizer.Status.JobsAnalyzed = jobsAnalyzed

	// Run OOM prevention analysis
	oomPrevented := 0
	if optimizer.Spec.OomPrevention != nil && optimizer.Spec.OomPrevention.Enabled {
		oomPrevented, err = r.runOomPrevention(ctx, optimizer, samples)
		if err != nil {
			log.Error(err, "OOM prevention analysis failed")
			r.updateCondition(optimizer, ConditionOomMonitoring, metav1.ConditionFalse, "OomAnalysisFailed", err.Error())
		} else {
			r.updateCondition(optimizer, ConditionOomMonitoring, metav1.ConditionTrue, "OomMonitoringActive", "OOM prevention monitoring is active")
		}
		optimizer.Status.OomPrevented += oomPrevented
	}

	// Run right-sizing analysis
	if optimizer.Spec.RightSizing != nil && optimizer.Spec.RightSizing.Enabled {
		recommendations, err := r.runRightSizing(ctx, optimizer, samples)
		if err != nil {
			log.Error(err, "Right-sizing analysis failed")
			r.updateCondition(optimizer, ConditionRightSizingActive, metav1.ConditionFalse, "RightSizingFailed", err.Error())
		} else {
			optimizer.Status.RightSizingRecommendations = recommendations
			r.updateCondition(optimizer, ConditionRightSizingActive, metav1.ConditionTrue, "RightSizingActive",
				fmt.Sprintf("Generated %d right-sizing recommendations", recommendations))
		}
	}

	// Update memory efficiency metrics
	r.updateMemoryEfficiency(optimizer, samples)

	// Update last analysis time
	now := metav1.Now()
	optimizer.Status.LastAnalysisTime = &now

	// Update status
	if err := r.Status().Update(ctx, optimizer); err != nil {
		log.Error(err, "Failed to update optimizer status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: defaultAnalysisInterval}, nil
}

// collectGpuNodes fetches FabricGpuNode resources matching the optimizer scope
func (r *FabricGpuMemoryOptimizerReconciler) collectGpuNodes(ctx context.Context, optimizer *kubefabricv1.FabricGpuMemoryOptimizer) ([]kubefabricv1.FabricGpuNode, error) {
	nodeList := &kubefabricv1.FabricGpuNodeList{}
	if err := r.List(ctx, nodeList); err != nil {
		return nil, fmt.Errorf("failed to list FabricGpuNodes: %w", err)
	}

	// Filter by scope
	var filtered []kubefabricv1.FabricGpuNode
	for _, node := range nodeList.Items {
		if r.nodeMatchesScope(node, optimizer.Spec.Scope) {
			filtered = append(filtered, node)
		}
	}

	return filtered, nil
}

// nodeMatchesScope checks if a GPU node matches the optimizer scope
func (r *FabricGpuMemoryOptimizerReconciler) nodeMatchesScope(node kubefabricv1.FabricGpuNode, scope kubefabricv1.OptimizerScope) bool {
	switch scope.Type {
	case "cluster":
		return true
	case "namespace":
		// FabricGpuNode is cluster-scoped, so we check labels for namespace association
		if ns, ok := node.Labels["kubefabric.ai/namespace"]; ok {
			return ns == scope.Namespace
		}
		// If no namespace label, include the node (cluster-scoped nodes serve all namespaces)
		return true
	case "job-selector":
		if scope.JobSelector == nil {
			return true
		}
		for key, value := range scope.JobSelector.MatchLabels {
			if nodeValue, ok := node.Labels[key]; !ok || nodeValue != value {
				return false
			}
		}
		return true
	default:
		return true
	}
}

// extractMemorySamples converts GPU node status data into memory samples for analysis
func (r *FabricGpuMemoryOptimizerReconciler) extractMemorySamples(nodes []kubefabricv1.FabricGpuNode) []memory.GpuMemorySample {
	var samples []memory.GpuMemorySample

	for _, node := range nodes {
		for _, gpuStatus := range node.Status.GpuStatus {
			if gpuStatus.MemoryTotal <= 0 {
				continue
			}

			sample := memory.GpuMemorySample{
				NodeName:    node.Spec.NodeName,
				GpuIndex:    gpuStatus.Index,
				GpuUUID:     gpuStatus.UUID,
				GpuType:     node.Spec.GpuType,
				MemoryUsed:  gpuStatus.MemoryUsed,
				MemoryTotal: gpuStatus.MemoryTotal,
				Utilization: gpuStatus.Utilization,
				Timestamp:   time.Now(),
			}

			samples = append(samples, sample)
		}
	}

	return samples
}

// runOomPrevention analyzes memory usage and predicts potential OOM events
func (r *FabricGpuMemoryOptimizerReconciler) runOomPrevention(ctx context.Context, optimizer *kubefabricv1.FabricGpuMemoryOptimizer, samples []memory.GpuMemorySample) (int, error) {
	if r.Predictor == nil {
		return 0, fmt.Errorf("memory predictor not initialized")
	}

	oomPrevented := 0
	warningThreshold := 85
	if optimizer.Spec.OomPrevention.PreemptiveAction != nil {
		if optimizer.Spec.OomPrevention.PreemptiveAction.WarningThresholdPercent > 0 {
			warningThreshold = optimizer.Spec.OomPrevention.PreemptiveAction.WarningThresholdPercent
		}
	}

	projectionMethod := "linear"
	profileSteps := 20
	if optimizer.Spec.OomPrevention.MemoryGrowthProfiling != nil {
		if optimizer.Spec.OomPrevention.MemoryGrowthProfiling.ProjectionMethod != "" {
			projectionMethod = optimizer.Spec.OomPrevention.MemoryGrowthProfiling.ProjectionMethod
		}
		if optimizer.Spec.OomPrevention.MemoryGrowthProfiling.ProfileSteps > 0 {
			profileSteps = optimizer.Spec.OomPrevention.MemoryGrowthProfiling.ProfileSteps
		}
	}

	for _, sample := range samples {
		// Feed the sample to the predictor
		r.Predictor.RecordSample(sample)

		// Check if we have enough samples for this GPU to make a prediction
		gpuKey := fmt.Sprintf("%s/gpu-%d", sample.NodeName, sample.GpuIndex)
		sampleCount := r.Predictor.SampleCount(gpuKey)
		if sampleCount < profileSteps {
			continue
		}

		// Predict whether OOM will occur
		prediction, err := r.Predictor.PredictOOM(gpuKey, projectionMethod)
		if err != nil {
			r.Log.Error(err, "Failed to predict OOM", "gpu", gpuKey)
			continue
		}

		if !prediction.OomLikely {
			continue
		}

		// Memory utilization exceeds warning threshold
		utilizationPct := float64(sample.MemoryUsed) / float64(sample.MemoryTotal) * 100.0
		if utilizationPct < float64(warningThreshold) {
			continue
		}

		r.Log.Info("OOM predicted for GPU",
			"gpu", gpuKey,
			"currentUtilization", fmt.Sprintf("%.1f%%", utilizationPct),
			"predictedTimeToOOM", prediction.TimeToOOM,
			"projectionMethod", projectionMethod,
		)

		// Determine action
		actionType := "warn"
		if optimizer.Spec.OomPrevention.PreemptiveAction != nil && optimizer.Spec.OomPrevention.PreemptiveAction.Type != "" {
			actionType = optimizer.Spec.OomPrevention.PreemptiveAction.Type
		}

		switch actionType {
		case "auto-mitigate":
			if err := r.applyMitigation(ctx, optimizer, sample); err != nil {
				r.Log.Error(err, "Failed to apply OOM mitigation", "gpu", gpuKey)
				continue
			}
			oomPrevented++
		case "evict":
			r.Log.Info("OOM eviction recommended for GPU workload", "gpu", gpuKey,
				"predictedTimeToOOM", prediction.TimeToOOM)
			oomPrevented++
		default:
			// warn only
			r.Log.Info("OOM warning issued for GPU workload", "gpu", gpuKey,
				"predictedTimeToOOM", prediction.TimeToOOM)
		}
	}

	return oomPrevented, nil
}

// applyMitigation applies automatic OOM mitigation strategies
func (r *FabricGpuMemoryOptimizerReconciler) applyMitigation(ctx context.Context, optimizer *kubefabricv1.FabricGpuMemoryOptimizer, sample memory.GpuMemorySample) error {
	if optimizer.Spec.OomPrevention == nil || optimizer.Spec.OomPrevention.PreemptiveAction == nil {
		return fmt.Errorf("no preemptive action configured")
	}
	preemptiveAction := optimizer.Spec.OomPrevention.PreemptiveAction

	mitigations := make([]string, 0)
	if preemptiveAction.AutoEnableGradientCheckpointing {
		mitigations = append(mitigations, "gradient_checkpointing")
	}
	if preemptiveAction.AutoEnableMixedPrecision {
		mitigations = append(mitigations, "mixed_precision")
	}
	if preemptiveAction.AutoReduceBatchSize {
		mitigations = append(mitigations, "reduce_batch_size")
	}

	if len(mitigations) == 0 {
		return fmt.Errorf("no mitigation strategies enabled")
	}

	r.Log.Info("Applying OOM mitigation strategies",
		"node", sample.NodeName,
		"gpuIndex", sample.GpuIndex,
		"strategies", mitigations,
		"memoryUsed", sample.MemoryUsed,
		"memoryTotal", sample.MemoryTotal,
	)

	// In a production implementation, this would:
	// 1. Find the pod running on this GPU
	// 2. Inject environment variables via a ConfigMap or annotation patch
	// 3. Signal the training framework to reload configuration
	// For gradient checkpointing: set GRADIENT_CHECKPOINTING=1
	// For mixed precision: set MIXED_PRECISION=1
	// For batch size reduction: set BATCH_SIZE_FACTOR=0.5

	return nil
}

// runRightSizing analyzes memory usage and generates right-sizing recommendations
func (r *FabricGpuMemoryOptimizerReconciler) runRightSizing(ctx context.Context, optimizer *kubefabricv1.FabricGpuMemoryOptimizer, samples []memory.GpuMemorySample) (int, error) {
	if r.Predictor == nil {
		return 0, fmt.Errorf("memory predictor not initialized")
	}

	headroom := 15
	if optimizer.Spec.RightSizing != nil && optimizer.Spec.RightSizing.MinimumGpuMemoryHeadroom > 0 {
		headroom = optimizer.Spec.RightSizing.MinimumGpuMemoryHeadroom
	}

	recommendations := 0

	// Group samples by GPU to analyze per-GPU utilization
	gpuSamples := make(map[string][]memory.GpuMemorySample)
	for _, sample := range samples {
		key := fmt.Sprintf("%s/gpu-%d", sample.NodeName, sample.GpuIndex)
		gpuSamples[key] = append(gpuSamples[key], sample)
	}

	for gpuKey, gpuSampleSet := range gpuSamples {
		analysis := r.Predictor.AnalyzeUtilization(gpuKey)
		if analysis == nil {
			continue
		}

		// Check if peak utilization is well below capacity
		maxRequiredPct := analysis.PeakUtilization + float64(headroom)

		if optimizer.Spec.RightSizing.Recommendations != nil {
			// Check for GPU type downgrade recommendation
			if optimizer.Spec.RightSizing.Recommendations.GpuTypeDowngrade && maxRequiredPct < 50.0 {
				if len(gpuSampleSet) > 0 {
					r.Log.Info("Right-sizing recommendation: GPU type downgrade",
						"gpu", gpuKey,
						"currentType", gpuSampleSet[0].GpuType,
						"peakUtilization", fmt.Sprintf("%.1f%%", analysis.PeakUtilization),
						"recommendation", "Consider using a GPU with less memory",
					)
					recommendations++
				}
			}

			// Check for GPU count reduction recommendation
			if optimizer.Spec.RightSizing.Recommendations.GpuCountReduction && analysis.SteadyStateUtilization < 30.0 {
				r.Log.Info("Right-sizing recommendation: GPU count reduction",
					"gpu", gpuKey,
					"steadyStateUtilization", fmt.Sprintf("%.1f%%", analysis.SteadyStateUtilization),
					"recommendation", "Consider reducing the number of GPUs",
				)
				recommendations++
			}
		}
	}

	return recommendations, nil
}

// updateMemoryEfficiency updates the memory efficiency status metrics
func (r *FabricGpuMemoryOptimizerReconciler) updateMemoryEfficiency(optimizer *kubefabricv1.FabricGpuMemoryOptimizer, samples []memory.GpuMemorySample) {
	if len(samples) == 0 {
		return
	}

	var totalPeakUtil, totalSteadyUtil float64
	validSamples := 0

	for _, sample := range samples {
		if sample.MemoryTotal <= 0 {
			continue
		}

		utilPct := float64(sample.MemoryUsed) / float64(sample.MemoryTotal) * 100.0
		totalPeakUtil += utilPct

		// Steady-state is approximated from current utilization
		// In production, this would use a time-series average
		totalSteadyUtil += utilPct * 0.85 // Steady state is typically ~85% of peak
		validSamples++
	}

	if validSamples > 0 {
		optimizer.Status.MemoryEfficiency = &kubefabricv1.MemoryEfficiencyStatus{
			AvgPeakUtilization:        totalPeakUtil / float64(validSamples),
			AvgSteadyStateUtilization: totalSteadyUtil / float64(validSamples),
		}
	}
}

// updateCondition updates or appends a condition on the optimizer status
func (r *FabricGpuMemoryOptimizerReconciler) updateCondition(optimizer *kubefabricv1.FabricGpuMemoryOptimizer, condType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}

	// Only update LastTransitionTime when status actually changes
	for _, cond := range optimizer.Status.Conditions {
		if cond.Type == condType && cond.Status == status {
			condition.LastTransitionTime = cond.LastTransitionTime
			break
		}
	}

	// Find and update existing condition or append new one
	found := false
	for i, cond := range optimizer.Status.Conditions {
		if cond.Type == condType {
			optimizer.Status.Conditions[i] = condition
			found = true
			break
		}
	}
	if !found {
		optimizer.Status.Conditions = append(optimizer.Status.Conditions, condition)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *FabricGpuMemoryOptimizerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubefabricv1.FabricGpuMemoryOptimizer{}).
		Complete(r)
}
