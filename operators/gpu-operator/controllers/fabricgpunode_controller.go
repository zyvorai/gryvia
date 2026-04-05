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

	kubefabricv1 "github.com/ssahani/kube-fabric/operators/gpu-operator/api/v1"
	"github.com/ssahani/kube-fabric/operators/gpu-operator/pkg/gpu"
)

const (
	fabricGpuNodeFinalizer = "kubefabric.ai/finalizer"

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

// FabricGpuNodeReconciler reconciles a FabricGpuNode object
type FabricGpuNodeReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

//+kubebuilder:rbac:groups=kubefabric.ai,resources=fabricgpunodes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kubefabric.ai,resources=fabricgpunodes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kubefabric.ai,resources=fabricgpunodes/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch;update;patch

// Reconcile is part of the main kubernetes reconciliation loop
func (r *FabricGpuNodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("fabricgpunode", req.NamespacedName)

	// Fetch the FabricGpuNode instance
	fabricNode := &kubefabricv1.FabricGpuNode{}
	err := r.Get(ctx, req.NamespacedName, fabricNode)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("FabricGpuNode resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get FabricGpuNode")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !fabricNode.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, fabricNode)
	}

	// Add finalizer if it doesn't exist
	if !controllerutil.ContainsFinalizer(fabricNode, fabricGpuNodeFinalizer) {
		controllerutil.AddFinalizer(fabricNode, fabricGpuNodeFinalizer)
		if err := r.Update(ctx, fabricNode); err != nil {
			return ctrl.Result{}, err
		}
		// Requeue to avoid working with a stale object after update
		return ctrl.Result{Requeue: true}, nil
	}

	// Initialize status if needed
	if fabricNode.Status.Phase == "" {
		fabricNode.Status.Phase = PhaseInitializing
		if err := r.Status().Update(ctx, fabricNode); err != nil {
			log.Error(err, "Failed to update FabricGpuNode status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Reconcile the GPU node
	result, err := r.reconcileGpuNode(ctx, fabricNode)
	if err != nil {
		log.Error(err, "Failed to reconcile GPU node")
		return result, err
	}

	// Schedule next reconciliation based on health check interval
	interval := 60 * time.Second
	if fabricNode.Spec.HealthCheck != nil && fabricNode.Spec.HealthCheck.Enabled {
		if fabricNode.Spec.HealthCheck.IntervalSeconds > 0 {
			interval = time.Duration(fabricNode.Spec.HealthCheck.IntervalSeconds) * time.Second
		}
	}

	return ctrl.Result{RequeueAfter: interval}, nil
}

func (r *FabricGpuNodeReconciler) reconcileGpuNode(ctx context.Context, fabricNode *kubefabricv1.FabricGpuNode) (ctrl.Result, error) {
	log := r.Log.WithValues("fabricgpunode", fabricNode.Name)

	// Get the Kubernetes node
	node := &corev1.Node{}
	err := r.Get(ctx, types.NamespacedName{Name: fabricNode.Spec.NodeName}, node)
	if err != nil {
		log.Error(err, "Failed to get Kubernetes node")
		r.updateCondition(fabricNode, ConditionNodeLabeled, metav1.ConditionFalse, "NodeNotFound", err.Error())
		fabricNode.Status.Phase = PhaseFailed
		if updateErr := r.Status().Update(ctx, fabricNode); updateErr != nil {
			log.Error(updateErr, "Failed to update status after node not found")
		}
		return ctrl.Result{}, err
	}

	// Check and install drivers
	if err := r.ensureDrivers(ctx, fabricNode); err != nil {
		log.Error(err, "Failed to ensure drivers")
		r.updateCondition(fabricNode, ConditionDriversInstalled, metav1.ConditionFalse, "DriverInstallFailed", err.Error())
		fabricNode.Status.Phase = PhaseDegraded
		if updateErr := r.Status().Update(ctx, fabricNode); updateErr != nil {
			log.Error(updateErr, "Failed to update status after driver install failure")
		}
		return ctrl.Result{RequeueAfter: 5 * time.Minute}, err
	}

	r.updateCondition(fabricNode, ConditionDriversInstalled, metav1.ConditionTrue, "DriversInstalled", "NVIDIA drivers installed successfully")

	// Perform GPU health check
	gpuHealth, err := r.checkGpuHealth(ctx, fabricNode)
	if err != nil {
		log.Error(err, "Failed to check GPU health")
		r.updateCondition(fabricNode, ConditionHealthy, metav1.ConditionFalse, "HealthCheckFailed", err.Error())
		fabricNode.Status.Phase = PhaseDegraded
	} else {
		fabricNode.Status.GpuStatus = gpuHealth
		r.updateCondition(fabricNode, ConditionHealthy, metav1.ConditionTrue, "Healthy", "All GPUs are healthy")
	}

	// Label the node
	if err := r.labelNode(ctx, fabricNode, node); err != nil {
		log.Error(err, "Failed to label node")
		r.updateCondition(fabricNode, ConditionNodeLabeled, metav1.ConditionFalse, "LabelFailed", err.Error())
	} else {
		r.updateCondition(fabricNode, ConditionNodeLabeled, metav1.ConditionTrue, "Labeled", "Node labeled successfully")
	}

	// Update phase based on conditions
	if r.allConditionsTrue(fabricNode) {
		fabricNode.Status.Phase = PhaseReady
	} else if r.anyConditionFalse(fabricNode) {
		fabricNode.Status.Phase = PhaseDegraded
	}

	// Update last health check time
	now := metav1.Now()
	fabricNode.Status.LastHealthCheck = &now

	// Update status
	if err := r.Status().Update(ctx, fabricNode); err != nil {
		log.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *FabricGpuNodeReconciler) ensureDrivers(ctx context.Context, fabricNode *kubefabricv1.FabricGpuNode) error {
	// Check if NVIDIA drivers are installed
	driverVersion, cudaVersion, err := gpu.GetDriverInfo()
	if err != nil {
		r.Log.Info("NVIDIA drivers not detected, triggering installation", "node", fabricNode.Spec.NodeName, "error", err)

		// In a real implementation, this would:
		// 1. Create a DaemonSet to install drivers on the node
		// 2. Use the NVIDIA GPU Operator or custom installation logic
		// 3. Wait for installation to complete

		fabricNode.Status.DriverVersion = "pending"
		fabricNode.Status.CudaVersion = "pending"
		return fmt.Errorf("NVIDIA drivers not yet installed on node %s: %w", fabricNode.Spec.NodeName, err)
	}

	fabricNode.Status.DriverVersion = driverVersion
	fabricNode.Status.CudaVersion = cudaVersion

	return nil
}

func (r *FabricGpuNodeReconciler) checkGpuHealth(ctx context.Context, fabricNode *kubefabricv1.FabricGpuNode) ([]kubefabricv1.GpuStatus, error) {
	// Use NVML to get GPU information
	gpuInfoList, err := gpu.GetGpuInfo(fabricNode.Spec.GpuCount)
	if err != nil {
		return nil, fmt.Errorf("failed to get GPU info: %w", err)
	}

	gpuStatus := make([]kubefabricv1.GpuStatus, 0, len(gpuInfoList))
	for _, gpuInfo := range gpuInfoList {
		status := kubefabricv1.GpuStatus{
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

func (r *FabricGpuNodeReconciler) labelNode(ctx context.Context, fabricNode *kubefabricv1.FabricGpuNode, node *corev1.Node) error {
	// Apply labels to the Kubernetes node
	if node.Labels == nil {
		node.Labels = make(map[string]string)
	}

	// Standard KubeFabric labels
	node.Labels["kubefabric.ai/gpu"] = fabricNode.Spec.GpuType
	node.Labels["kubefabric.ai/gpu-count"] = fmt.Sprintf("%d", fabricNode.Spec.GpuCount)
	node.Labels["kubefabric.ai/rdma"] = fmt.Sprintf("%t", fabricNode.Spec.RDMA)
	node.Labels["kubefabric.ai/sriov"] = fmt.Sprintf("%t", fabricNode.Spec.SRIOV)

	if fabricNode.Spec.Interconnect != "" {
		node.Labels["kubefabric.ai/interconnect"] = fabricNode.Spec.Interconnect
	}

	// Apply custom labels from spec (only allow kubefabric.ai/ prefix)
	for k, v := range fabricNode.Spec.Labels {
		if strings.HasPrefix(k, "kubefabric.ai/") {
			node.Labels[k] = v
		}
	}

	// Update the node
	return r.Update(ctx, node)
}

func (r *FabricGpuNodeReconciler) handleDeletion(ctx context.Context, fabricNode *kubefabricv1.FabricGpuNode) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(fabricNode, fabricGpuNodeFinalizer) {
		// Cleanup: remove labels from node
		node := &corev1.Node{}
		err := r.Get(ctx, types.NamespacedName{Name: fabricNode.Spec.NodeName}, node)
		if err == nil {
			// Remove KubeFabric labels
			delete(node.Labels, "kubefabric.ai/gpu")
			delete(node.Labels, "kubefabric.ai/gpu-count")
			delete(node.Labels, "kubefabric.ai/rdma")
			delete(node.Labels, "kubefabric.ai/sriov")
			delete(node.Labels, "kubefabric.ai/interconnect")
			if err := r.Update(ctx, node); err != nil {
				r.Log.Error(err, "Failed to remove labels from node during cleanup", "node", fabricNode.Spec.NodeName)
				return ctrl.Result{}, err
			}
		} else if !errors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("failed to get node %s during cleanup: %w", fabricNode.Spec.NodeName, err)
		}

		// Remove finalizer
		controllerutil.RemoveFinalizer(fabricNode, fabricGpuNodeFinalizer)
		if err := r.Update(ctx, fabricNode); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *FabricGpuNodeReconciler) updateCondition(fabricNode *kubefabricv1.FabricGpuNode, condType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}

	// Only update LastTransitionTime when status actually changes
	for _, cond := range fabricNode.Status.Conditions {
		if cond.Type == condType && cond.Status == status {
			condition.LastTransitionTime = cond.LastTransitionTime
			break
		}
	}

	// Find and update existing condition or append new one
	found := false
	for i, cond := range fabricNode.Status.Conditions {
		if cond.Type == condType {
			fabricNode.Status.Conditions[i] = condition
			found = true
			break
		}
	}
	if !found {
		fabricNode.Status.Conditions = append(fabricNode.Status.Conditions, condition)
	}
}

func (r *FabricGpuNodeReconciler) allConditionsTrue(fabricNode *kubefabricv1.FabricGpuNode) bool {
	requiredConditions := []string{ConditionDriversInstalled, ConditionNodeLabeled, ConditionHealthy}
	for _, reqCond := range requiredConditions {
		found := false
		for _, cond := range fabricNode.Status.Conditions {
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

func (r *FabricGpuNodeReconciler) anyConditionFalse(fabricNode *kubefabricv1.FabricGpuNode) bool {
	for _, cond := range fabricNode.Status.Conditions {
		if cond.Status == metav1.ConditionFalse {
			return true
		}
	}
	return false
}

// SetupWithManager sets up the controller with the Manager.
func (r *FabricGpuNodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubefabricv1.FabricGpuNode{}).
		Complete(r)
}
