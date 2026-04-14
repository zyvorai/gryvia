package controllers

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"k8s.io/client-go/util/retry"

	tensorreaperv1 "github.com/ssahani/TensorReaper/operators/quota-operator/api/v1"
)

const (
	reservationFinalizer = "tensorreaper.ai/reservation-finalizer"

	reservationStatePending   = "pending"
	reservationStateActive    = "active"
	reservationStateExpired   = "expired"
	reservationStateCancelled = "cancelled"
)

// FabricReservationReconciler reconciles a FabricReservation object
type FabricReservationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabricreservations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabricreservations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabricreservations/finalizers,verbs=update
//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabricaijobs,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch;update;patch

// Reconcile is part of the main kubernetes reconciliation loop
func (r *FabricReservationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the FabricReservation instance
	reservation := &tensorreaperv1.FabricReservation{}
	err := r.Get(ctx, req.NamespacedName, reservation)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("FabricReservation resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get FabricReservation")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !reservation.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, reservation)
	}

	// Add finalizer if it doesn't exist
	if !controllerutil.ContainsFinalizer(reservation, reservationFinalizer) {
		controllerutil.AddFinalizer(reservation, reservationFinalizer)
		if err := r.Update(ctx, reservation); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Initialize status if needed
	if reservation.Status.State == "" {
		reservation.Status.State = reservationStatePending
		if err := r.Status().Update(ctx, reservation); err != nil {
			logger.Error(err, "Failed to update initial status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Skip reconciliation for terminal states
	if reservation.Status.State == reservationStateExpired ||
		reservation.Status.State == reservationStateCancelled {
		return ctrl.Result{}, nil
	}

	// Reconcile the reservation
	result, err := r.reconcileReservation(ctx, reservation)
	if err != nil {
		logger.Error(err, "Failed to reconcile reservation")
		return result, err
	}

	return result, nil
}

func (r *FabricReservationReconciler) reconcileReservation(ctx context.Context, reservation *tensorreaperv1.FabricReservation) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	now := time.Now()

	// Check if reservation should be active based on schedule
	switch reservation.Spec.Schedule.Type {
	case "immediate":
		if reservation.Status.State == reservationStatePending {
			reservation.Status.State = reservationStateActive
			metaNow := metav1.Now()
			reservation.Status.ActualStartTime = &metaNow
		}

	case "scheduled":
		if reservation.Spec.Schedule.StartTime != nil {
			if now.Before(reservation.Spec.Schedule.StartTime.Time) {
				// Not yet time to activate
				logger.Info("Reservation not yet active", "startTime", reservation.Spec.Schedule.StartTime)
				if err := r.Status().Update(ctx, reservation); err != nil {
					return ctrl.Result{}, err
				}
				waitDuration := time.Until(reservation.Spec.Schedule.StartTime.Time)
				return ctrl.Result{RequeueAfter: waitDuration}, nil
			}
			if reservation.Status.State == reservationStatePending {
				reservation.Status.State = reservationStateActive
				metaNow := metav1.Now()
				reservation.Status.ActualStartTime = &metaNow
			}
		}

	case "recurring":
		// For recurring reservations, check if we're in an active window
		if reservation.Status.State == reservationStatePending {
			reservation.Status.State = reservationStateActive
			metaNow := metav1.Now()
			reservation.Status.ActualStartTime = &metaNow
		}
	}

	// Check if reservation has expired
	if reservation.Spec.Schedule.EndTime != nil && now.After(reservation.Spec.Schedule.EndTime.Time) {
		// Check auto-extend
		if reservation.Spec.Schedule.AutoExtend {
			hasRunningJobs, err := r.hasRunningJobs(ctx, reservation)
			if err != nil {
				logger.Error(err, "Failed to check running jobs for auto-extend")
			} else if hasRunningJobs {
				logger.Info("Auto-extending reservation due to running jobs")
				// Don't expire yet
			} else {
				return r.expireReservation(ctx, reservation)
			}
		} else {
			return r.expireReservation(ctx, reservation)
		}
	}

	// Allocate nodes for active reservation
	if reservation.Status.State == reservationStateActive {
		if err := r.allocateNodes(ctx, reservation); err != nil {
			logger.Error(err, "Failed to allocate nodes")
			return ctrl.Result{RequeueAfter: 30 * time.Second}, err
		}

		// Update utilization metrics
		r.updateUtilization(ctx, reservation)

		// Label reserved nodes to block scheduler
		if err := r.labelReservedNodes(ctx, reservation); err != nil {
			logger.Error(err, "Failed to label reserved nodes")
		}
	}

	// Update status
	if err := r.Status().Update(ctx, reservation); err != nil {
		logger.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
}

func (r *FabricReservationReconciler) allocateNodes(ctx context.Context, reservation *tensorreaperv1.FabricReservation) error {
	// If specific nodes are requested
	if len(reservation.Spec.Resources.Nodes) > 0 {
		reservation.Status.AllocatedNodes = reservation.Spec.Resources.Nodes
		// Count GPUs on allocated nodes
		var totalGPUs int32
		for _, nodeName := range reservation.Spec.Resources.Nodes {
			node := &corev1.Node{}
			if err := r.Get(ctx, types.NamespacedName{Name: nodeName}, node); err != nil {
				continue
			}
			// Count GPUs from node labels
			if gpuCountStr, ok := node.Labels["tensorreaper.ai/gpu-count"]; ok {
				var gpuCount int
				if _, err := fmt.Sscanf(gpuCountStr, "%d", &gpuCount); err == nil {
					totalGPUs += int32(gpuCount)
				}
			}
		}
		reservation.Status.AllocatedGPUs = totalGPUs
		return nil
	}

	// Find nodes matching GPU type
	if reservation.Spec.Resources.GpuType != "" && reservation.Spec.Resources.GpuCount > 0 {
		nodeList := &corev1.NodeList{}
		if err := r.List(ctx, nodeList, client.MatchingLabels{
			"tensorreaper.ai/gpu": reservation.Spec.Resources.GpuType,
		}); err != nil {
			return fmt.Errorf("failed to list GPU nodes: %w", err)
		}

		var allocatedNodes []string
		var allocatedGPUs int32

		for _, node := range nodeList.Items {
			if allocatedGPUs >= reservation.Spec.Resources.GpuCount {
				break
			}

			// Check if already reserved by another reservation
			if owner, ok := node.Labels["tensorreaper.ai/reserved-by"]; ok {
				if owner != reservation.Name {
					continue // skip nodes reserved by other reservations
				}
			}

			allocatedNodes = append(allocatedNodes, node.Name)

			// Count GPUs from node labels
			if gpuCountStr, ok := node.Labels["tensorreaper.ai/gpu-count"]; ok {
				var gpuCount int
				if _, err := fmt.Sscanf(gpuCountStr, "%d", &gpuCount); err == nil {
					allocatedGPUs += int32(gpuCount)
				}
			} else {
				allocatedGPUs += 8 // default assumption
			}
		}

		reservation.Status.AllocatedNodes = allocatedNodes
		reservation.Status.AllocatedGPUs = allocatedGPUs
	}

	return nil
}

func (r *FabricReservationReconciler) labelReservedNodes(ctx context.Context, reservation *tensorreaperv1.FabricReservation) error {
	for _, nodeName := range reservation.Status.AllocatedNodes {
		if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			node := &corev1.Node{}
			if err := r.Get(ctx, types.NamespacedName{Name: nodeName}, node); err != nil {
				return err
			}

			if node.Labels == nil {
				node.Labels = make(map[string]string)
			}

			needsUpdate := false

			// Set reservation owner label
			if node.Labels["tensorreaper.ai/reserved-by"] != reservation.Name {
				node.Labels["tensorreaper.ai/reserved-by"] = reservation.Name
				needsUpdate = true
			}

			// Set owner info
			ownerLabel := fmt.Sprintf("%s/%s", reservation.Spec.Owner.Type, reservation.Spec.Owner.Name)
			if node.Labels["tensorreaper.ai/reserved-for"] != ownerLabel {
				node.Labels["tensorreaper.ai/reserved-for"] = ownerLabel
				needsUpdate = true
			}

			// Mark exclusive if applicable
			if reservation.Spec.Guarantees != nil && reservation.Spec.Guarantees.Exclusive {
				if node.Labels["tensorreaper.ai/exclusive"] != "true" {
					node.Labels["tensorreaper.ai/exclusive"] = "true"
					needsUpdate = true
				}
			}

			if needsUpdate {
				return r.Update(ctx, node)
			}
			return nil
		}); err != nil {
			return fmt.Errorf("failed to label reserved node %s: %w", nodeName, err)
		}
	}

	return nil
}

func (r *FabricReservationReconciler) removeReservationLabels(ctx context.Context, reservation *tensorreaperv1.FabricReservation) error {
	for _, nodeName := range reservation.Status.AllocatedNodes {
		if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			node := &corev1.Node{}
			if err := r.Get(ctx, types.NamespacedName{Name: nodeName}, node); err != nil {
				if errors.IsNotFound(err) {
					return nil
				}
				return err
			}

			if node.Labels == nil {
				return nil
			}

			needsUpdate := false

			if node.Labels["tensorreaper.ai/reserved-by"] == reservation.Name {
				delete(node.Labels, "tensorreaper.ai/reserved-by")
				delete(node.Labels, "tensorreaper.ai/reserved-for")
				delete(node.Labels, "tensorreaper.ai/exclusive")
				needsUpdate = true
			}

			if needsUpdate {
				return r.Update(ctx, node)
			}
			return nil
		}); err != nil {
			log.FromContext(ctx).Error(err, "Failed to remove reservation labels from node", "node", nodeName)
		}
	}

	return nil
}

func (r *FabricReservationReconciler) updateUtilization(ctx context.Context, reservation *tensorreaperv1.FabricReservation) {
	if reservation.Status.UtilizationMetrics == nil {
		reservation.Status.UtilizationMetrics = &tensorreaperv1.ReservationUtilization{}
	}

	// Calculate total reserved time
	if reservation.Status.ActualStartTime != nil {
		elapsed := time.Since(reservation.Status.ActualStartTime.Time)
		reservation.Status.UtilizationMetrics.TotalReservedTime = formatReservationDuration(elapsed)
	}

	// Count running jobs using this reservation
	jobList := &tensorreaperv1.FabricAIJobList{}
	if err := r.List(ctx, jobList); err == nil {
		var runningJobs int32
		for _, job := range jobList.Items {
			labels := job.GetLabels()
			if labels != nil && labels["tensorreaper.ai/reservation"] == reservation.Name {
				runningJobs++
			}
			// Also check annotation
			annotations := job.GetAnnotations()
			if annotations != nil && annotations["tensorreaper.ai/reservation"] == reservation.Name {
				runningJobs++
			}
		}
		reservation.Status.UtilizationMetrics.JobsRun = runningJobs
	}
}

func (r *FabricReservationReconciler) hasRunningJobs(ctx context.Context, reservation *tensorreaperv1.FabricReservation) (bool, error) {
	jobList := &tensorreaperv1.FabricAIJobList{}
	if err := r.List(ctx, jobList); err != nil {
		return false, err
	}

	for _, job := range jobList.Items {
		if job.Status.Phase != "Running" {
			continue
		}
		labels := job.GetLabels()
		if labels != nil && labels["tensorreaper.ai/reservation"] == reservation.Name {
			return true, nil
		}
		annotations := job.GetAnnotations()
		if annotations != nil && annotations["tensorreaper.ai/reservation"] == reservation.Name {
			return true, nil
		}
	}

	return false, nil
}

func (r *FabricReservationReconciler) expireReservation(ctx context.Context, reservation *tensorreaperv1.FabricReservation) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Expiring reservation", "name", reservation.Name)

	reservation.Status.State = reservationStateExpired
	now := metav1.Now()
	reservation.Status.ActualEndTime = &now

	// Remove reservation labels from nodes
	if err := r.removeReservationLabels(ctx, reservation); err != nil {
		logger.Error(err, "Failed to remove reservation labels during expiry")
	}

	// Update status
	if err := r.Status().Update(ctx, reservation); err != nil {
		logger.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *FabricReservationReconciler) handleDeletion(ctx context.Context, reservation *tensorreaperv1.FabricReservation) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if controllerutil.ContainsFinalizer(reservation, reservationFinalizer) {
		logger.Info("Running cleanup for FabricReservation", "name", reservation.Name)

		// Remove reservation labels from nodes
		if err := r.removeReservationLabels(ctx, reservation); err != nil {
			logger.Error(err, "Failed to remove reservation labels during cleanup")
		}

		// Remove finalizer
		controllerutil.RemoveFinalizer(reservation, reservationFinalizer)
		if err := r.Update(ctx, reservation); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func formatReservationDuration(d time.Duration) string {
	hours := int(d.Hours())
	if hours < 24 {
		return fmt.Sprintf("%dh", hours)
	}
	days := hours / 24
	remainingHours := hours % 24
	return fmt.Sprintf("%dd%dh", days, remainingHours)
}

// SetupWithManager sets up the controller with the Manager.
func (r *FabricReservationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tensorreaperv1.FabricReservation{}).
		Complete(r)
}
