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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	tensorreaperv1 "github.com/ssahani/TensorReaper/operators/ai-operator/api/v1"
)

const (
	autoScalerDefaultCooldownUp   = 5 * time.Minute
	autoScalerDefaultCooldownDown = 15 * time.Minute
)

// CloudScaler is an interface for cloud provider backends
type CloudScaler interface {
	ScaleUp(ctx context.Context, instanceType string, count int32) ([]string, error)
	ScaleDown(ctx context.Context, nodeNames []string) error
	GetNodeStatus(ctx context.Context, nodeNames []string) (map[string]string, error)
}

// FabricAutoScalerReconciler reconciles a FabricAutoScaler object
type FabricAutoScalerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabricautoscalers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabricautoscalers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabricautoscalers/finalizers,verbs=update
//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabricaijobs,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop
func (r *FabricAutoScalerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("fabricautoscaler", req.NamespacedName)

	// Fetch the FabricAutoScaler instance
	scaler := &tensorreaperv1.FabricAutoScaler{}
	err := r.Get(ctx, req.NamespacedName, scaler)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("FabricAutoScaler resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get FabricAutoScaler")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !scaler.ObjectMeta.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	// Reconcile autoscaler
	result, err := r.reconcileAutoScaler(ctx, scaler)
	if err != nil {
		log.Error(err, "Failed to reconcile autoscaler")
		return result, err
	}

	return result, nil
}

func (r *FabricAutoScalerReconciler) reconcileAutoScaler(ctx context.Context, scaler *tensorreaperv1.FabricAutoScaler) (ctrl.Result, error) {
	log := r.Log.WithValues("fabricautoscaler", scaler.Name)

	// Collect current metrics
	metrics, err := r.collectMetrics(ctx, scaler)
	if err != nil {
		log.Error(err, "Failed to collect metrics")
		r.updateCondition(scaler, "MetricsCollected", metav1.ConditionFalse, "CollectionFailed", err.Error())
		if updateErr := r.Status().Update(ctx, scaler); updateErr != nil {
			log.Error(updateErr, "Failed to update status")
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	scaler.Status.Metrics = metrics
	r.updateCondition(scaler, "MetricsCollected", metav1.ConditionTrue, "Collected", "Metrics collected successfully")

	// Count current GPU nodes matching the GPU type
	currentNodes, err := r.countGPUNodes(ctx, scaler.Spec.GpuType)
	if err != nil {
		log.Error(err, "Failed to count GPU nodes")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}
	scaler.Status.CurrentNodes = currentNodes

	// Evaluate scaling decision
	desiredNodes := currentNodes
	scaleAction := "none"

	// Check scale up triggers
	if r.shouldScaleUp(scaler, metrics) {
		if r.isCooldownExpired(scaler, true) {
			increment := int32(1)
			if scaler.Spec.ScaleUpPolicy != nil && scaler.Spec.ScaleUpPolicy.Increment > 0 {
				increment = scaler.Spec.ScaleUpPolicy.Increment
			}
			desiredNodes = currentNodes + increment
			if desiredNodes > scaler.Spec.MaxNodes {
				desiredNodes = scaler.Spec.MaxNodes
			}
			if desiredNodes > currentNodes {
				scaleAction = "scale-up"
				log.Info("Scaling up", "current", currentNodes, "desired", desiredNodes, "increment", increment)
			}
		} else {
			log.Info("Scale up needed but cooldown not expired")
		}
	}

	// Check scale down triggers (only if no scale up)
	if scaleAction == "none" && r.shouldScaleDown(scaler, metrics) {
		if r.isCooldownExpired(scaler, false) {
			decrement := int32(1)
			if scaler.Spec.ScaleDownPolicy != nil && scaler.Spec.ScaleDownPolicy.Decrement > 0 {
				decrement = scaler.Spec.ScaleDownPolicy.Decrement
			}
			desiredNodes = currentNodes - decrement
			if desiredNodes < scaler.Spec.MinNodes {
				desiredNodes = scaler.Spec.MinNodes
			}
			// Check scaleToZero
			if desiredNodes < 0 {
				desiredNodes = 0
			}
			if scaler.Spec.Advanced == nil || !scaler.Spec.Advanced.ScaleToZero {
				if desiredNodes == 0 && scaler.Spec.MinNodes == 0 {
					desiredNodes = 0
				}
			}
			if desiredNodes < currentNodes {
				scaleAction = "scale-down"
				log.Info("Scaling down", "current", currentNodes, "desired", desiredNodes, "decrement", decrement)
			}
		} else {
			log.Info("Scale down needed but cooldown not expired")
		}
	}

	scaler.Status.DesiredNodes = desiredNodes

	// Execute scaling action
	if scaleAction != "none" && desiredNodes != currentNodes {
		now := metav1.Now()
		scaler.Status.LastScaleTime = &now
		scaler.Status.LastScaleAction = scaleAction
		scaler.Status.PendingNodes = desiredNodes - currentNodes
		if scaler.Status.PendingNodes < 0 {
			scaler.Status.PendingNodes = 0
		}

		r.updateCondition(scaler, "ScalingActive", metav1.ConditionTrue, "Scaling",
			fmt.Sprintf("Scaling %s: %d -> %d nodes", scaleAction, currentNodes, desiredNodes))
	} else {
		scaler.Status.PendingNodes = 0
		r.updateCondition(scaler, "ScalingActive", metav1.ConditionFalse, "Stable", "No scaling needed")
	}

	// Update at-capacity condition
	if currentNodes >= scaler.Spec.MaxNodes {
		r.updateCondition(scaler, "AtMaxCapacity", metav1.ConditionTrue, "AtMax",
			fmt.Sprintf("Current: %d, Max: %d", currentNodes, scaler.Spec.MaxNodes))
	} else {
		r.updateCondition(scaler, "AtMaxCapacity", metav1.ConditionFalse, "BelowMax",
			fmt.Sprintf("Current: %d, Max: %d", currentNodes, scaler.Spec.MaxNodes))
	}

	// Update status
	if err := r.Status().Update(ctx, scaler); err != nil {
		log.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

func (r *FabricAutoScalerReconciler) collectMetrics(ctx context.Context, scaler *tensorreaperv1.FabricAutoScaler) (*tensorreaperv1.AutoScalerMetrics, error) {
	metrics := &tensorreaperv1.AutoScalerMetrics{}

	// Count pending jobs for the referenced queue
	jobList := &tensorreaperv1.FabricAIJobList{}
	if err := r.List(ctx, jobList); err != nil {
		return nil, fmt.Errorf("failed to list FabricAIJobs: %w", err)
	}

	var pendingCount int32
	for _, job := range jobList.Items {
		if job.Status.Phase == PhasePending || job.Status.Phase == PhaseScheduling {
			if scaler.Spec.GpuType == "" || job.Spec.GpuType == scaler.Spec.GpuType {
				pendingCount++
			}
		}
	}
	metrics.PendingJobs = pendingCount

	// Calculate GPU utilization from nodes
	nodeList := &corev1.NodeList{}
	if err := r.List(ctx, nodeList, client.MatchingLabels{
		"tensorreaper.ai/gpu": scaler.Spec.GpuType,
	}); err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	// Estimate utilization based on running jobs vs capacity
	var totalGPUs, usedGPUs int32
	for range nodeList.Items {
		totalGPUs += 8 // default assumption per node
	}

	for _, job := range jobList.Items {
		if job.Status.Phase == PhaseRunning {
			if scaler.Spec.GpuType == "" || job.Spec.GpuType == scaler.Spec.GpuType {
				usedGPUs += job.Spec.GPUs
			}
		}
	}

	if totalGPUs > 0 {
		metrics.UtilizationPercent = float64(usedGPUs) / float64(totalGPUs) * 100
	}

	return metrics, nil
}

func (r *FabricAutoScalerReconciler) countGPUNodes(ctx context.Context, gpuType string) (int32, error) {
	nodeList := &corev1.NodeList{}
	labels := map[string]string{}
	if gpuType != "" {
		labels["tensorreaper.ai/gpu"] = gpuType
	}

	if err := r.List(ctx, nodeList, client.MatchingLabels(labels)); err != nil {
		return 0, fmt.Errorf("failed to list GPU nodes: %w", err)
	}

	return int32(len(nodeList.Items)), nil
}

func (r *FabricAutoScalerReconciler) shouldScaleUp(scaler *tensorreaperv1.FabricAutoScaler, metrics *tensorreaperv1.AutoScalerMetrics) bool {
	if scaler.Spec.ScaleUpPolicy == nil || metrics == nil {
		return false
	}

	policy := scaler.Spec.ScaleUpPolicy

	// Check pending jobs threshold
	if policy.PendingJobs > 0 && metrics.PendingJobs > policy.PendingJobs {
		return true
	}

	// Check utilization threshold
	if policy.UtilizationPercent > 0 && metrics.UtilizationPercent > float64(policy.UtilizationPercent) {
		return true
	}

	// Check queue time threshold
	if policy.QueueTimeMinutes > 0 && metrics.AvgQueueTimeMinutes > float64(policy.QueueTimeMinutes) {
		return true
	}

	return false
}

func (r *FabricAutoScalerReconciler) shouldScaleDown(scaler *tensorreaperv1.FabricAutoScaler, metrics *tensorreaperv1.AutoScalerMetrics) bool {
	if scaler.Spec.ScaleDownPolicy == nil || metrics == nil {
		return false
	}

	policy := scaler.Spec.ScaleDownPolicy

	// Check utilization below threshold
	if policy.UtilizationPercent > 0 && metrics.UtilizationPercent < float64(policy.UtilizationPercent) {
		return true
	}

	return false
}

func (r *FabricAutoScalerReconciler) isCooldownExpired(scaler *tensorreaperv1.FabricAutoScaler, isScaleUp bool) bool {
	if scaler.Status.LastScaleTime == nil {
		return true
	}

	var cooldown time.Duration
	if isScaleUp {
		cooldown = autoScalerDefaultCooldownUp
		if scaler.Spec.ScaleUpPolicy != nil && scaler.Spec.ScaleUpPolicy.CooldownMinutes > 0 {
			cooldown = time.Duration(scaler.Spec.ScaleUpPolicy.CooldownMinutes) * time.Minute
		}
	} else {
		cooldown = autoScalerDefaultCooldownDown
		if scaler.Spec.ScaleDownPolicy != nil && scaler.Spec.ScaleDownPolicy.CooldownMinutes > 0 {
			cooldown = time.Duration(scaler.Spec.ScaleDownPolicy.CooldownMinutes) * time.Minute
		}
	}

	return time.Since(scaler.Status.LastScaleTime.Time) >= cooldown
}

func (r *FabricAutoScalerReconciler) updateCondition(scaler *tensorreaperv1.FabricAutoScaler, condType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: scaler.Generation,
		LastTransitionTime: metav1.Now(),
	}

	for _, cond := range scaler.Status.Conditions {
		if cond.Type == condType && cond.Status == status {
			condition.LastTransitionTime = cond.LastTransitionTime
			break
		}
	}

	found := false
	for i, cond := range scaler.Status.Conditions {
		if cond.Type == condType {
			scaler.Status.Conditions[i] = condition
			found = true
			break
		}
	}
	if !found {
		scaler.Status.Conditions = append(scaler.Status.Conditions, condition)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *FabricAutoScalerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tensorreaperv1.FabricAutoScaler{}).
		Complete(r)
}
