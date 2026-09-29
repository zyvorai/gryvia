package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"k8s.io/client-go/util/retry"

	gryviav1 "github.com/zyvorai/gryvia/operators/gpu-operator/api/v1"
	"github.com/zyvorai/gryvia/operators/gpu-operator/pkg/gpu"
)

const (
	gryviaGpuNodeFinalizer = "gryvia.io/finalizer"

	// Status phases
	PhaseInitializing = "Initializing"
	PhaseReady        = "Ready"
	PhaseDegraded     = "Degraded"
	PhaseFailed       = "Failed"

	// Condition types
	ConditionDriversInstalled = "DriversInstalled"
	ConditionNodeLabeled      = "NodeLabeled"
	ConditionHealthy          = "Healthy"
)

// GryviaGpuNodeReconciler reconciles a GryviaGpuNode object
type GryviaGpuNodeReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

//+kubebuilder:rbac:groups=gryvia.io,resources=gryviagpunodes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gryvia.io,resources=gryviagpunodes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gryvia.io,resources=gryviagpunodes/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch;update;patch

// Reconcile is part of the main kubernetes reconciliation loop
func (r *GryviaGpuNodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("gryviagpunode", req.NamespacedName)

	// Fetch the GryviaGpuNode instance
	gryviaNode := &gryviav1.GryviaGpuNode{}
	err := r.Get(ctx, req.NamespacedName, gryviaNode)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("GryviaGpuNode resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get GryviaGpuNode")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !gryviaNode.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, gryviaNode)
	}

	// Add finalizer if it doesn't exist
	if !controllerutil.ContainsFinalizer(gryviaNode, gryviaGpuNodeFinalizer) {
		controllerutil.AddFinalizer(gryviaNode, gryviaGpuNodeFinalizer)
		if err := r.Update(ctx, gryviaNode); err != nil {
			return ctrl.Result{}, err
		}
		// Requeue to avoid working with a stale object after update
		return ctrl.Result{Requeue: true}, nil
	}

	// Initialize status if needed
	if gryviaNode.Status.Phase == "" {
		gryviaNode.Status.Phase = PhaseInitializing
		if err := r.Status().Update(ctx, gryviaNode); err != nil {
			log.Error(err, "Failed to update GryviaGpuNode status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Reconcile the GPU node
	result, err := r.reconcileGpuNode(ctx, gryviaNode)
	if err != nil {
		log.Error(err, "Failed to reconcile GPU node")
		return result, err
	}

	// Use the returned result's RequeueAfter if non-zero, otherwise use default interval
	if result.RequeueAfter > 0 {
		return result, nil
	}

	// Schedule next reconciliation based on health check interval
	interval := 60 * time.Second
	if gryviaNode.Spec.HealthCheck != nil && gryviaNode.Spec.HealthCheck.Enabled {
		if gryviaNode.Spec.HealthCheck.IntervalSeconds > 0 {
			interval = time.Duration(gryviaNode.Spec.HealthCheck.IntervalSeconds) * time.Second
		}
	}

	return ctrl.Result{RequeueAfter: interval}, nil
}

func (r *GryviaGpuNodeReconciler) reconcileGpuNode(ctx context.Context, gryviaNode *gryviav1.GryviaGpuNode) (ctrl.Result, error) {
	log := r.Log.WithValues("gryviagpunode", gryviaNode.Name)

	// Get the Kubernetes node
	node := &corev1.Node{}
	err := r.Get(ctx, types.NamespacedName{Name: gryviaNode.Spec.NodeName}, node)
	if err != nil {
		log.Error(err, "Failed to get Kubernetes node")
		r.updateCondition(gryviaNode, ConditionNodeLabeled, metav1.ConditionFalse, "NodeNotFound", err.Error())
		gryviaNode.Status.Phase = PhaseFailed
		if updateErr := r.Status().Update(ctx, gryviaNode); updateErr != nil {
			log.Error(updateErr, "Failed to update status after node not found")
		}
		return ctrl.Result{}, err
	}

	// Check and install drivers
	if err := r.ensureDrivers(ctx, gryviaNode); err != nil {
		log.Error(err, "Failed to ensure drivers")
		r.updateCondition(gryviaNode, ConditionDriversInstalled, metav1.ConditionFalse, "DriverInstallFailed", err.Error())
		gryviaNode.Status.Phase = PhaseDegraded
		if updateErr := r.Status().Update(ctx, gryviaNode); updateErr != nil {
			log.Error(updateErr, "Failed to update status after driver install failure")
		}
		return ctrl.Result{RequeueAfter: 5 * time.Minute}, err
	}

	r.updateCondition(gryviaNode, ConditionDriversInstalled, metav1.ConditionTrue, "DriversInstalled", "NVIDIA drivers installed successfully")

	// Perform GPU health check
	gpuHealth, err := r.checkGpuHealth(ctx, gryviaNode)
	if err != nil {
		log.Error(err, "Failed to check GPU health")
		r.updateCondition(gryviaNode, ConditionHealthy, metav1.ConditionFalse, "HealthCheckFailed", err.Error())
		gryviaNode.Status.Phase = PhaseDegraded
		// Update status and requeue with exponential backoff
		now := metav1.Now()
		gryviaNode.Status.LastHealthCheck = &now
		if updateErr := r.Status().Update(ctx, gryviaNode); updateErr != nil {
			log.Error(updateErr, "Failed to update status after health check failure")
		}
		backoff := r.calculateBackoff(gryviaNode)
		return ctrl.Result{RequeueAfter: backoff}, err
	} else {
		gryviaNode.Status.GpuStatus = gpuHealth
		r.updateCondition(gryviaNode, ConditionHealthy, metav1.ConditionTrue, "Healthy", "All GPUs are healthy")
	}

	// Label the node
	if err := r.labelNode(ctx, gryviaNode, node); err != nil {
		log.Error(err, "Failed to label node")
		r.updateCondition(gryviaNode, ConditionNodeLabeled, metav1.ConditionFalse, "LabelFailed", err.Error())
	} else {
		r.updateCondition(gryviaNode, ConditionNodeLabeled, metav1.ConditionTrue, "Labeled", "Node labeled successfully")
	}

	// Update phase based on conditions
	if r.allConditionsTrue(gryviaNode) {
		gryviaNode.Status.Phase = PhaseReady
	} else if r.anyConditionFalse(gryviaNode) {
		gryviaNode.Status.Phase = PhaseDegraded
	}

	// Update last health check time
	now := metav1.Now()
	gryviaNode.Status.LastHealthCheck = &now

	// Update status
	if err := r.Status().Update(ctx, gryviaNode); err != nil {
		log.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *GryviaGpuNodeReconciler) ensureDrivers(ctx context.Context, gryviaNode *gryviav1.GryviaGpuNode) error {
	// Check if NVIDIA drivers are installed
	driverVersion, cudaVersion, err := gpu.GetDriverInfo()
	if err != nil {
		r.Log.Info("NVIDIA drivers not detected, triggering installation", "node", gryviaNode.Spec.NodeName, "error", err)

		// In a real implementation, this would:
		// 1. Create a DaemonSet to install drivers on the node
		// 2. Use the NVIDIA GPU Operator or custom installation logic
		// 3. Wait for installation to complete

		gryviaNode.Status.DriverVersion = "pending"
		gryviaNode.Status.CudaVersion = "pending"
		return fmt.Errorf("NVIDIA drivers not yet installed on node %s: %w", gryviaNode.Spec.NodeName, err)
	}

	gryviaNode.Status.DriverVersion = driverVersion
	gryviaNode.Status.CudaVersion = cudaVersion

	return nil
}

func (r *GryviaGpuNodeReconciler) checkGpuHealth(ctx context.Context, gryviaNode *gryviav1.GryviaGpuNode) ([]gryviav1.GpuStatus, error) {
	// Use NVML to get GPU information
	gpuInfoList, err := gpu.GetGpuInfo(gryviaNode.Spec.GpuCount)
	if err != nil {
		return nil, fmt.Errorf("failed to get GPU info: %w", err)
	}

	gpuStatus := make([]gryviav1.GpuStatus, 0, len(gpuInfoList))
	for _, gpuInfo := range gpuInfoList {
		status := gryviav1.GpuStatus{
			Index:       gpuInfo.Index,
			UUID:        gpuInfo.UUID,
			Health:      gpuInfo.Health,
			Temperature: gpuInfo.Temperature,
			PowerUsage:  gpuInfo.PowerUsage,
			MemoryUsed:  gpuInfo.MemoryUsed,
			MemoryTotal: gpuInfo.MemoryTotal,
			Utilization: gpuInfo.Utilization,
		}
		gpuStatus = append(gpuStatus, status)
	}

	return gpuStatus, nil
}

func (r *GryviaGpuNodeReconciler) labelNode(ctx context.Context, gryviaNode *gryviav1.GryviaGpuNode, node *corev1.Node) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Re-fetch node to get latest version
		if err := r.Get(ctx, types.NamespacedName{Name: node.Name}, node); err != nil {
			return err
		}

		// Apply labels to the Kubernetes node
		if node.Labels == nil {
			node.Labels = make(map[string]string)
		}

		// Standard Gryvia labels
		node.Labels["gryvia.io/gpu"] = gryviaNode.Spec.GpuType
		node.Labels["gryvia.io/gpu-count"] = fmt.Sprintf("%d", gryviaNode.Spec.GpuCount)
		node.Labels["gryvia.io/rdma"] = fmt.Sprintf("%t", gryviaNode.Spec.RDMA)
		node.Labels["gryvia.io/sriov"] = fmt.Sprintf("%t", gryviaNode.Spec.SRIOV)

		if gryviaNode.Spec.Interconnect != "" {
			node.Labels["gryvia.io/interconnect"] = gryviaNode.Spec.Interconnect
		}

		// Apply custom labels from spec (only allow gryvia.io/ prefix)
		for k, v := range gryviaNode.Spec.Labels {
			if strings.HasPrefix(k, "gryvia.io/") {
				node.Labels[k] = v
			}
		}

		// Update the node
		return r.Update(ctx, node)
	})
}

func (r *GryviaGpuNodeReconciler) handleDeletion(ctx context.Context, gryviaNode *gryviav1.GryviaGpuNode) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(gryviaNode, gryviaGpuNodeFinalizer) {
		// Cleanup: remove labels from node
		node := &corev1.Node{}
		err := r.Get(ctx, types.NamespacedName{Name: gryviaNode.Spec.NodeName}, node)
		if err == nil {
			if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				if err := r.Get(ctx, types.NamespacedName{Name: gryviaNode.Spec.NodeName}, node); err != nil {
					return err
				}
				// Remove Gryvia labels
				if node.Labels != nil {
					delete(node.Labels, "gryvia.io/gpu")
					delete(node.Labels, "gryvia.io/gpu-count")
					delete(node.Labels, "gryvia.io/rdma")
					delete(node.Labels, "gryvia.io/sriov")
					delete(node.Labels, "gryvia.io/interconnect")

					// Remove custom labels with gryvia.io/ prefix from spec
					for k := range gryviaNode.Spec.Labels {
						if strings.HasPrefix(k, "gryvia.io/") {
							delete(node.Labels, k)
						}
					}
				}

				return r.Update(ctx, node)
			}); err != nil {
				r.Log.Error(err, "Failed to remove labels from node during cleanup", "node", gryviaNode.Spec.NodeName)
				return ctrl.Result{}, err
			}
		} else if !errors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("failed to get node %s during cleanup: %w", gryviaNode.Spec.NodeName, err)
		}

		// Remove finalizer
		controllerutil.RemoveFinalizer(gryviaNode, gryviaGpuNodeFinalizer)
		if err := r.Update(ctx, gryviaNode); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *GryviaGpuNodeReconciler) updateCondition(gryviaNode *gryviav1.GryviaGpuNode, condType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}

	// Only update LastTransitionTime when status actually changes
	for _, cond := range gryviaNode.Status.Conditions {
		if cond.Type == condType && cond.Status == status {
			condition.LastTransitionTime = cond.LastTransitionTime
			break
		}
	}

	// Find and update existing condition or append new one
	found := false
	for i, cond := range gryviaNode.Status.Conditions {
		if cond.Type == condType {
			gryviaNode.Status.Conditions[i] = condition
			found = true
			break
		}
	}
	if !found {
		gryviaNode.Status.Conditions = append(gryviaNode.Status.Conditions, condition)
	}
}

// calculateBackoff returns an exponential backoff duration based on the number
// of consecutive health check failures (indicated by the Healthy condition being False).
func (r *GryviaGpuNodeReconciler) calculateBackoff(gryviaNode *gryviav1.GryviaGpuNode) time.Duration {
	const (
		minBackoff = 30 * time.Second
		maxBackoff = 10 * time.Minute
	)
	backoff := minBackoff
	for _, cond := range gryviaNode.Status.Conditions {
		if cond.Type == ConditionHealthy && cond.Status == metav1.ConditionFalse {
			elapsed := time.Since(cond.LastTransitionTime.Time)
			// Double backoff for each minute the condition has been false
			multiplier := int(elapsed.Minutes()) + 1
			backoff = time.Duration(multiplier) * minBackoff
			break
		}
	}
	if backoff > maxBackoff {
		backoff = maxBackoff
	}
	return backoff
}

func (r *GryviaGpuNodeReconciler) allConditionsTrue(gryviaNode *gryviav1.GryviaGpuNode) bool {
	requiredConditions := []string{ConditionDriversInstalled, ConditionNodeLabeled, ConditionHealthy}
	for _, reqCond := range requiredConditions {
		found := false
		for _, cond := range gryviaNode.Status.Conditions {
			if cond.Type == reqCond && cond.Status == metav1.ConditionTrue {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (r *GryviaGpuNodeReconciler) anyConditionFalse(gryviaNode *gryviav1.GryviaGpuNode) bool {
	for _, cond := range gryviaNode.Status.Conditions {
		if cond.Status == metav1.ConditionFalse {
			return true
		}
	}
	return false
}

// SetupWithManager sets up the controller with the Manager.
func (r *GryviaGpuNodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gryviav1.GryviaGpuNode{}).
		Complete(r)
}
