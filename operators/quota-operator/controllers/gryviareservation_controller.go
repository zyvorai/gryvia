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

	gryviav1 "github.com/zyvorai/gryvia/operators/quota-operator/api/v1"
)

const (
	reservationFinalizer = "gryvia.io/reservation-finalizer"

	reservationStatePending   = "pending"
	reservationStateActive    = "active"
	reservationStateExpired   = "expired"
	reservationStateCancelled = "cancelled"
)

// GryviaReservationReconciler reconciles a GryviaReservation object
type GryviaReservationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=gryvia.io,resources=gryviareservations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gryvia.io,resources=gryviareservations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gryvia.io,resources=gryviareservations/finalizers,verbs=update
//+kubebuilder:rbac:groups=gryvia.io,resources=gryviaaijobs,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch;update;patch

// Reconcile is part of the main kubernetes reconciliation loop
func (r *GryviaReservationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the GryviaReservation instance
	reservation := &gryviav1.GryviaReservation{}
	err := r.Get(ctx, req.NamespacedName, reservation)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("GryviaReservation resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get GryviaReservation")
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

func (r *GryviaReservationReconciler) reconcileReservation(ctx context.Context, reservation *gryviav1.GryviaReservation) (ctrl.Result, error) {
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

func (r *GryviaReservationReconciler) allocateNodes(ctx context.Context, reservation *gryviav1.GryviaReservation) error {
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
			if gpuCountStr, ok := node.Labels["gryvia.io/gpu-count"]; ok {
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
			"gryvia.io/gpu": reservation.Spec.Resources.GpuType,
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
			if owner, ok := node.Labels["gryvia.io/reserved-by"]; ok {
				if owner != reservation.Name {
					continue // skip nodes reserved by other reservations
				}
			}

			allocatedNodes = append(allocatedNodes, node.Name)

			// Count GPUs from node labels
			if gpuCountStr, ok := node.Labels["gryvia.io/gpu-count"]; ok {
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

func (r *GryviaReservationReconciler) labelReservedNodes(ctx context.Context, reservation *gryviav1.GryviaReservation) error {
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
			if node.Labels["gryvia.io/reserved-by"] != reservation.Name {
				node.Labels["gryvia.io/reserved-by"] = reservation.Name
				needsUpdate = true
			}

			// Set owner info
			ownerLabel := fmt.Sprintf("%s/%s", reservation.Spec.Owner.Type, reservation.Spec.Owner.Name)
			if node.Labels["gryvia.io/reserved-for"] != ownerLabel {
				node.Labels["gryvia.io/reserved-for"] = ownerLabel
				needsUpdate = true
			}

			// Mark exclusive if applicable
			if reservation.Spec.Guarantees != nil && reservation.Spec.Guarantees.Exclusive {
				if node.Labels["gryvia.io/exclusive"] != "true" {
					node.Labels["gryvia.io/exclusive"] = "true"
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

func (r *GryviaReservationReconciler) removeReservationLabels(ctx context.Context, reservation *gryviav1.GryviaReservation) error {
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

			if node.Labels["gryvia.io/reserved-by"] == reservation.Name {
				delete(node.Labels, "gryvia.io/reserved-by")
				delete(node.Labels, "gryvia.io/reserved-for")
				delete(node.Labels, "gryvia.io/exclusive")
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

func (r *GryviaReservationReconciler) updateUtilization(ctx context.Context, reservation *gryviav1.GryviaReservation) {
	if reservation.Status.UtilizationMetrics == nil {
		reservation.Status.UtilizationMetrics = &gryviav1.ReservationUtilization{}
	}

	// Calculate total reserved time
	if reservation.Status.ActualStartTime != nil {
		elapsed := time.Since(reservation.Status.ActualStartTime.Time)
		reservation.Status.UtilizationMetrics.TotalReservedTime = formatReservationDuration(elapsed)
	}

	// Count running jobs using this reservation
	jobList := &gryviav1.GryviaAIJobList{}
	if err := r.List(ctx, jobList); err == nil {
		var runningJobs int32
		for _, job := range jobList.Items {
			labels := job.GetLabels()
			if labels != nil && labels["gryvia.io/reservation"] == reservation.Name {
				runningJobs++
			}
			// Also check annotation
			annotations := job.GetAnnotations()
			if annotations != nil && annotations["gryvia.io/reservation"] == reservation.Name {
				runningJobs++
			}
		}
		reservation.Status.UtilizationMetrics.JobsRun = runningJobs
	}
}

func (r *GryviaReservationReconciler) hasRunningJobs(ctx context.Context, reservation *gryviav1.GryviaReservation) (bool, error) {
	jobList := &gryviav1.GryviaAIJobList{}
	if err := r.List(ctx, jobList); err != nil {
		return false, err
	}

	for _, job := range jobList.Items {
		if job.Status.Phase != "Running" {
			continue
		}
		labels := job.GetLabels()
		if labels != nil && labels["gryvia.io/reservation"] == reservation.Name {
			return true, nil
		}
		annotations := job.GetAnnotations()
		if annotations != nil && annotations["gryvia.io/reservation"] == reservation.Name {
			return true, nil
		}
	}

	return false, nil
}

func (r *GryviaReservationReconciler) expireReservation(ctx context.Context, reservation *gryviav1.GryviaReservation) (ctrl.Result, error) {
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

func (r *GryviaReservationReconciler) handleDeletion(ctx context.Context, reservation *gryviav1.GryviaReservation) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if controllerutil.ContainsFinalizer(reservation, reservationFinalizer) {
		logger.Info("Running cleanup for GryviaReservation", "name", reservation.Name)

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
func (r *GryviaReservationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gryviav1.GryviaReservation{}).
		Complete(r)
}
