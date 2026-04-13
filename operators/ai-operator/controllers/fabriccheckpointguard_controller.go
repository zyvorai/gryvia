package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	tensorreaperv1 "github.com/ssahani/TensorReaper/operators/ai-operator/api/v1"
	"github.com/ssahani/TensorReaper/operators/ai-operator/pkg/checkpoint"
)

const (
	// Checkpoint guard condition types
	ConditionGuardActive       = "GuardActive"
	ConditionCheckpointValid   = "CheckpointValid"
	ConditionEmergencyTriggered = "EmergencyTriggered"

	// Checkpoint guard phases
	PhaseIdle           = "Idle"
	PhaseMonitoring     = "Monitoring"
	PhaseCheckpointing  = "Checkpointing"
	PhaseRestoring      = "Restoring"
	PhaseError          = "Error"

	// Emergency trigger constants
	TriggerGpuHealthDegraded    = "GpuHealthDegraded"
	TriggerSpotPreemptionSignal = "SpotPreemptionSignal"
	TriggerMemoryPressure       = "MemoryPressure"
	TriggerLossDivergence       = "LossDivergence"
	TriggerNvlinkDegraded       = "NvlinkDegraded"
)

// FabricCheckpointGuardReconciler reconciles a FabricCheckpointGuard object
type FabricCheckpointGuardReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabriccheckpointguards,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabriccheckpointguards/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabriccheckpointguards/finalizers,verbs=update
//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabricaijobs,verbs=get;list;watch
//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabricgpunodes,verbs=get;list;watch
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop
func (r *FabricCheckpointGuardReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("fabriccheckpointguard", req.NamespacedName)

	// Fetch the FabricCheckpointGuard instance
	guard := &tensorreaperv1.FabricCheckpointGuard{}
	err := r.Get(ctx, req.NamespacedName, guard)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("FabricCheckpointGuard resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get FabricCheckpointGuard")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !guard.ObjectMeta.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	// Initialize status if needed
	if guard.Status.Phase == "" {
		guard.Status.Phase = PhaseIdle
		if err := r.Status().Update(ctx, guard); err != nil {
			log.Error(err, "Failed to initialize FabricCheckpointGuard status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Reconcile the checkpoint guard
	result, err := r.reconcileCheckpointGuard(ctx, guard)
	if err != nil {
		log.Error(err, "Failed to reconcile checkpoint guard")
		return result, err
	}

	return result, nil
}

func (r *FabricCheckpointGuardReconciler) reconcileCheckpointGuard(ctx context.Context, guard *tensorreaperv1.FabricCheckpointGuard) (ctrl.Result, error) {
	log := r.Log.WithValues("fabriccheckpointguard", guard.Name)

	// Step 1: Find matching FabricAIJob resources using the jobSelector
	matchedJobs, err := r.findMatchingJobs(ctx, guard)
	if err != nil {
		log.Error(err, "Failed to find matching jobs")
		guard.Status.Phase = PhaseError
		r.updateGuardCondition(guard, ConditionGuardActive, metav1.ConditionFalse, "JobLookupFailed", err.Error())
		if updateErr := r.Status().Update(ctx, guard); updateErr != nil {
			log.Error(updateErr, "Failed to update status after job lookup failure")
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	guard.Status.MatchedJobs = int32(len(matchedJobs))

	// If no jobs matched, set to Idle
	if len(matchedJobs) == 0 {
		guard.Status.Phase = PhaseIdle
		r.updateGuardCondition(guard, ConditionGuardActive, metav1.ConditionFalse, "NoMatchingJobs", "No FabricAIJob resources match the jobSelector")
		if err := r.Status().Update(ctx, guard); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: 60 * time.Second}, nil
	}

	// Step 2: Check running jobs and their pod/GPU health
	guard.Status.Phase = PhaseMonitoring
	r.updateGuardCondition(guard, ConditionGuardActive, metav1.ConditionTrue, "Monitoring", fmt.Sprintf("Monitoring %d matched job(s)", len(matchedJobs)))

	emergencyNeeded := false
	emergencyReason := ""

	for _, job := range matchedJobs {
		// Only monitor running jobs
		if job.Status.Phase != PhaseRunning {
			continue
		}

		// Step 3: Check StatefulSet pods for the job
		podHealthy, podReason := r.checkJobPodHealth(ctx, &job)
		if !podHealthy {
			log.Info("Pod health issue detected", "job", job.Name, "reason", podReason)
			if r.isEmergencyTrigger(guard, TriggerMemoryPressure) {
				emergencyNeeded = true
				emergencyReason = fmt.Sprintf("Pod health issue on job %s: %s", job.Name, podReason)
			}
		}

		// Step 4: Check GPU health from FabricGpuNode resources
		gpuHealthy, gpuReason := r.checkGpuHealth(ctx, &job)
		if !gpuHealthy {
			log.Info("GPU health issue detected", "job", job.Name, "reason", gpuReason)
			if r.isEmergencyTrigger(guard, TriggerGpuHealthDegraded) {
				emergencyNeeded = true
				emergencyReason = fmt.Sprintf("GPU health degraded for job %s: %s", job.Name, gpuReason)
			}
		}

		// Step 5: Check for NVLink degradation via GPU node status
		nvlinkHealthy := r.checkNvlinkHealth(ctx, &job)
		if !nvlinkHealthy {
			log.Info("NVLink degradation detected", "job", job.Name)
			if r.isEmergencyTrigger(guard, TriggerNvlinkDegraded) {
				emergencyNeeded = true
				emergencyReason = fmt.Sprintf("NVLink degraded for job %s", job.Name)
			}
		}

		// Step 6: Check for spot preemption signals via pod conditions
		if r.detectSpotPreemption(ctx, &job) {
			log.Info("Spot preemption signal detected", "job", job.Name)
			if r.isEmergencyTrigger(guard, TriggerSpotPreemptionSignal) {
				emergencyNeeded = true
				emergencyReason = fmt.Sprintf("Spot preemption signal for job %s", job.Name)
			}
		}

		// Step 7: Check for loss divergence via job metrics
		if r.detectLossDivergence(&job) {
			log.Info("Loss divergence detected", "job", job.Name)
			if r.isEmergencyTrigger(guard, TriggerLossDivergence) {
				emergencyNeeded = true
				emergencyReason = fmt.Sprintf("Loss divergence for job %s", job.Name)
			}
		}
	}

	// Step 8: Handle emergency checkpoint if triggered
	if emergencyNeeded {
		log.Info("Emergency checkpoint triggered", "reason", emergencyReason)
		guard.Status.Phase = PhaseCheckpointing
		r.updateGuardCondition(guard, ConditionEmergencyTriggered, metav1.ConditionTrue, "EmergencyCheckpoint", emergencyReason)

		if err := r.performCheckpoint(ctx, guard, matchedJobs, true); err != nil {
			log.Error(err, "Failed to perform emergency checkpoint")
			guard.Status.Phase = PhaseError
			r.updateGuardCondition(guard, ConditionCheckpointValid, metav1.ConditionFalse, "CheckpointFailed", err.Error())
		} else {
			guard.Status.EmergencyCheckpointsTaken++
			guard.Status.Phase = PhaseMonitoring
		}
	}

	// Step 9: Check if a periodic checkpoint is due
	if r.isPeriodicCheckpointDue(guard) {
		log.Info("Periodic checkpoint due")
		guard.Status.Phase = PhaseCheckpointing

		if err := r.performCheckpoint(ctx, guard, matchedJobs, false); err != nil {
			log.Error(err, "Failed to perform periodic checkpoint")
			guard.Status.Phase = PhaseError
			r.updateGuardCondition(guard, ConditionCheckpointValid, metav1.ConditionFalse, "CheckpointFailed", err.Error())
		} else {
			guard.Status.Phase = PhaseMonitoring
		}
	}

	// Step 10: Update status
	if err := r.Status().Update(ctx, guard); err != nil {
		log.Error(err, "Failed to update checkpoint guard status")
		return ctrl.Result{}, err
	}

	// Requeue based on checkpoint interval
	requeueAfter := r.getRequeueInterval(guard)
	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

// findMatchingJobs finds FabricAIJob resources that match the guard's jobSelector
func (r *FabricCheckpointGuardReconciler) findMatchingJobs(ctx context.Context, guard *tensorreaperv1.FabricCheckpointGuard) ([]tensorreaperv1.FabricAIJob, error) {
	jobList := &tensorreaperv1.FabricAIJobList{}

	listOpts := []client.ListOption{
		client.InNamespace(guard.Namespace),
	}

	if len(guard.Spec.JobSelector.MatchLabels) > 0 {
		listOpts = append(listOpts, client.MatchingLabels(guard.Spec.JobSelector.MatchLabels))
	}

	if err := r.List(ctx, jobList, listOpts...); err != nil {
		return nil, fmt.Errorf("failed to list FabricAIJobs: %w", err)
	}

	return jobList.Items, nil
}

// checkJobPodHealth checks the health of pods belonging to a job's StatefulSet
func (r *FabricCheckpointGuardReconciler) checkJobPodHealth(ctx context.Context, job *tensorreaperv1.FabricAIJob) (bool, string) {
	// Look up the StatefulSet for this job
	stsName := fmt.Sprintf("%s-training", job.Name)
	sts := &appsv1.StatefulSet{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: job.Namespace,
		Name:      stsName,
	}, sts)
	if err != nil {
		if errors.IsNotFound(err) {
			return true, "" // No StatefulSet yet, nothing to check
		}
		return false, fmt.Sprintf("failed to get StatefulSet %s: %v", stsName, err)
	}

	// List pods for the StatefulSet
	pods := &corev1.PodList{}
	if err := r.List(ctx, pods, client.InNamespace(job.Namespace), client.MatchingLabels{
		"tensorreaper.ai/job": job.Name,
	}); err != nil {
		return false, fmt.Sprintf("failed to list pods: %v", err)
	}

	for _, pod := range pods.Items {
		// Check for memory pressure via pod conditions
		for _, condition := range pod.Status.Conditions {
			if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionFalse {
				return false, fmt.Sprintf("pod %s is not ready: %s", pod.Name, condition.Message)
			}
		}

		// Check for containers in OOMKilled or CrashLoopBackOff state
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.State.Waiting != nil && cs.State.Waiting.Reason == "CrashLoopBackOff" {
				return false, fmt.Sprintf("pod %s container %s in CrashLoopBackOff", pod.Name, cs.Name)
			}
			if cs.LastTerminationState.Terminated != nil && cs.LastTerminationState.Terminated.Reason == "OOMKilled" {
				return false, fmt.Sprintf("pod %s container %s OOMKilled", pod.Name, cs.Name)
			}
		}
	}

	return true, ""
}

// checkGpuHealth checks GPU health status from FabricGpuNode resources for nodes running the job
func (r *FabricCheckpointGuardReconciler) checkGpuHealth(ctx context.Context, job *tensorreaperv1.FabricAIJob) (bool, string) {
	if len(job.Status.NodesAllocated) == 0 {
		return true, ""
	}

	// List all FabricGpuNode resources (they are cluster-scoped)
	gpuNodeList := &tensorreaperv1.FabricGpuNodeList{}
	if err := r.List(ctx, gpuNodeList); err != nil {
		// If the CRD is not installed, skip the check gracefully
		return true, ""
	}

	// Build a map of node names to GPU nodes for quick lookup
	gpuNodesByNodeName := make(map[string]*tensorreaperv1.FabricGpuNode)
	for i := range gpuNodeList.Items {
		gpuNode := &gpuNodeList.Items[i]
		gpuNodesByNodeName[gpuNode.Spec.NodeName] = gpuNode
	}

	for _, nodeName := range job.Status.NodesAllocated {
		gpuNode, exists := gpuNodesByNodeName[nodeName]
		if !exists {
			continue // No FabricGpuNode for this node, skip
		}

		// Check GPU node phase
		if gpuNode.Status.Phase == "Degraded" || gpuNode.Status.Phase == "Failed" {
			return false, fmt.Sprintf("GPU node %s is in %s phase", nodeName, gpuNode.Status.Phase)
		}

		// Check individual GPU health statuses
		for _, gpuStatus := range gpuNode.Status.GpuStatus {
			if gpuStatus.Health != "Healthy" && gpuStatus.Health != "" {
				return false, fmt.Sprintf("GPU %d on node %s health: %s", gpuStatus.Index, nodeName, gpuStatus.Health)
			}

			// Check for thermal throttling (temperature > 85C)
			if gpuStatus.Temperature > 85 {
				return false, fmt.Sprintf("GPU %d on node %s overheating: %d°C", gpuStatus.Index, nodeName, gpuStatus.Temperature)
			}
		}
	}

	return true, ""
}

// checkNvlinkHealth checks NVLink interconnect health from FabricGpuNode resources
func (r *FabricCheckpointGuardReconciler) checkNvlinkHealth(ctx context.Context, job *tensorreaperv1.FabricAIJob) bool {
	if len(job.Status.NodesAllocated) == 0 {
		return true
	}

	gpuNodeList := &tensorreaperv1.FabricGpuNodeList{}
	if err := r.List(ctx, gpuNodeList); err != nil {
		return true // Cannot check, assume healthy
	}

	for _, nodeName := range job.Status.NodesAllocated {
		for i := range gpuNodeList.Items {
			gpuNode := &gpuNodeList.Items[i]
			if gpuNode.Spec.NodeName != nodeName {
				continue
			}

			// Check conditions for NVLink-related issues
			for _, condition := range gpuNode.Status.Conditions {
				if condition.Type == "NvlinkHealthy" && condition.Status == metav1.ConditionFalse {
					return false
				}
			}

			// If interconnect is NVLink and node is degraded, NVLink may be affected
			if gpuNode.Spec.Interconnect == "nvlink" && gpuNode.Status.Phase == "Degraded" {
				return false
			}
		}
	}

	return true
}

// detectSpotPreemption checks if any pods have received spot preemption signals
func (r *FabricCheckpointGuardReconciler) detectSpotPreemption(ctx context.Context, job *tensorreaperv1.FabricAIJob) bool {
	pods := &corev1.PodList{}
	if err := r.List(ctx, pods, client.InNamespace(job.Namespace), client.MatchingLabels{
		"tensorreaper.ai/job": job.Name,
	}); err != nil {
		return false
	}

	for _, pod := range pods.Items {
		// Check for spot preemption annotations (set by cloud provider or node termination handler)
		if _, hasAnnotation := pod.Annotations["tensorreaper.ai/spot-preemption"]; hasAnnotation {
			return true
		}

		// Check node conditions for spot termination notices
		if pod.Spec.NodeName != "" {
			node := &corev1.Node{}
			if err := r.Get(ctx, types.NamespacedName{Name: pod.Spec.NodeName}, node); err == nil {
				for _, condition := range node.Status.Conditions {
					if condition.Type == "SpotTermination" && condition.Status == corev1.ConditionTrue {
						return true
					}
				}
				// Check for preemption-related taints
				for _, taint := range node.Spec.Taints {
					if taint.Key == "cloud.google.com/impending-node-termination" ||
						taint.Key == "aws-node-termination-handler/spot-itn" {
						return true
					}
				}
			}
		}
	}

	return false
}

// detectLossDivergence checks if the job's training loss is diverging
func (r *FabricCheckpointGuardReconciler) detectLossDivergence(job *tensorreaperv1.FabricAIJob) bool {
	if job.Status.Metrics == nil {
		return false
	}

	// Detect loss divergence: NaN, Inf, or extremely large values
	loss := job.Status.Metrics.Loss
	if loss != loss { // NaN check
		return true
	}
	// Loss values above 1e6 typically indicate divergence in most training scenarios
	if loss > 1e6 {
		return true
	}

	return false
}

// isEmergencyTrigger checks if a given trigger is configured in the guard's emergency checkpoint policy
func (r *FabricCheckpointGuardReconciler) isEmergencyTrigger(guard *tensorreaperv1.FabricCheckpointGuard, trigger string) bool {
	if guard.Spec.CheckpointPolicy.EmergencyCheckpoint == nil {
		return false
	}

	for _, t := range guard.Spec.CheckpointPolicy.EmergencyCheckpoint.Triggers {
		if t == trigger {
			return true
		}
	}

	return false
}

// isPeriodicCheckpointDue checks if enough time has elapsed since the last checkpoint
func (r *FabricCheckpointGuardReconciler) isPeriodicCheckpointDue(guard *tensorreaperv1.FabricCheckpointGuard) bool {
	interval := guard.Spec.CheckpointPolicy.IntervalMinutes
	if interval <= 0 {
		interval = 30 // Default 30 minutes
	}

	if guard.Status.LastCheckpointTime == nil {
		return true // Never checkpointed, checkpoint now
	}

	elapsed := time.Since(guard.Status.LastCheckpointTime.Time)
	return elapsed >= time.Duration(interval)*time.Minute
}

// performCheckpoint executes a checkpoint operation for the matched jobs
func (r *FabricCheckpointGuardReconciler) performCheckpoint(ctx context.Context, guard *tensorreaperv1.FabricCheckpointGuard, jobs []tensorreaperv1.FabricAIJob, isEmergency bool) error {
	log := r.Log.WithValues("fabriccheckpointguard", guard.Name, "emergency", isEmergency)

	checkpointStart := time.Now()
	checkpointName := fmt.Sprintf("ckpt-%s-%d", guard.Name, time.Now().Unix())

	for _, job := range jobs {
		if job.Status.Phase != PhaseRunning {
			continue
		}

		log.Info("Performing checkpoint for job", "job", job.Name, "checkpoint", checkpointName)

		// Build the checkpoint path
		checkpointPath := fmt.Sprintf("/checkpoints/%s/%s", job.Name, checkpointName)

		// Validate checkpoint configuration
		retentionCount := int32(5) // default
		if guard.Spec.Validation != nil && guard.Spec.Validation.RetentionCount > 0 {
			retentionCount = guard.Spec.Validation.RetentionCount
		}

		// Create checkpoint metadata for the index
		ckptMeta := checkpoint.CheckpointMetadata{
			Name:        checkpointName,
			Path:        checkpointPath,
			JobName:     job.Name,
			CreatedAt:   metav1.Now(),
			IsEmergency: isEmergency,
		}

		// Validate the checkpoint using the validator
		if guard.Spec.Validation != nil {
			opts := checkpoint.ValidationOptions{
				ChecksumVerify:    guard.Spec.Validation.ChecksumVerify,
				TensorShapeVerify: guard.Spec.Validation.TensorShapeVerify,
				LoadTest:          guard.Spec.Validation.LoadTest,
			}

			valid, reason := checkpoint.ValidateCheckpoint(checkpointPath, opts)
			ckptMeta.Valid = valid
			ckptMeta.ValidationMessage = reason

			if valid {
				guard.Status.ValidCheckpoints++
				guard.Status.LastValidCheckpoint = checkpointName
				r.updateGuardCondition(guard, ConditionCheckpointValid, metav1.ConditionTrue, "CheckpointValid",
					fmt.Sprintf("Checkpoint %s passed validation", checkpointName))
			} else {
				log.Info("Checkpoint failed validation", "checkpoint", checkpointName, "reason", reason)
				r.updateGuardCondition(guard, ConditionCheckpointValid, metav1.ConditionFalse, "ValidationFailed",
					fmt.Sprintf("Checkpoint %s failed validation: %s", checkpointName, reason))

				// If retainValidOnly, skip adding this checkpoint
				if guard.Spec.Validation.RetainValidOnly {
					continue
				}
			}
		} else {
			// No validation configured, mark as valid
			ckptMeta.Valid = true
			guard.Status.ValidCheckpoints++
			guard.Status.LastValidCheckpoint = checkpointName
		}

		// Manage the checkpoint index with retention
		checkpoint.AddToIndex(checkpointName, ckptMeta, int(retentionCount))
	}

	// Update checkpoint timing and counts
	now := metav1.Now()
	guard.Status.LastCheckpointTime = &now
	guard.Status.TotalCheckpoints++

	// Calculate average checkpoint duration
	duration := time.Since(checkpointStart)
	guard.Status.AvgCheckpointDuration = r.updateAvgDuration(guard.Status.AvgCheckpointDuration, duration, guard.Status.TotalCheckpoints)

	log.Info("Checkpoint completed", "checkpoint", checkpointName, "duration", duration)
	return nil
}

// updateAvgDuration computes a running average of checkpoint duration
func (r *FabricCheckpointGuardReconciler) updateAvgDuration(currentAvg string, newDuration time.Duration, totalCount int32) string {
	if totalCount <= 1 {
		return newDuration.Round(time.Second).String()
	}

	// Parse existing average
	existing, err := time.ParseDuration(currentAvg)
	if err != nil {
		return newDuration.Round(time.Second).String()
	}

	// Weighted running average
	avg := (existing*time.Duration(totalCount-1) + newDuration) / time.Duration(totalCount)
	return avg.Round(time.Second).String()
}

// getRequeueInterval returns the requeue interval based on checkpoint policy
func (r *FabricCheckpointGuardReconciler) getRequeueInterval(guard *tensorreaperv1.FabricCheckpointGuard) time.Duration {
	interval := guard.Spec.CheckpointPolicy.IntervalMinutes
	if interval <= 0 {
		interval = 30
	}

	// Requeue at half the checkpoint interval for responsive monitoring,
	// but no more frequently than every 30 seconds
	requeue := time.Duration(interval) * time.Minute / 2
	if requeue < 30*time.Second {
		requeue = 30 * time.Second
	}

	return requeue
}

// updateGuardCondition updates a condition on the checkpoint guard status
func (r *FabricCheckpointGuardReconciler) updateGuardCondition(guard *tensorreaperv1.FabricCheckpointGuard, condType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: guard.Generation,
		LastTransitionTime: metav1.Now(),
	}

	// Only update LastTransitionTime when status actually changes
	for _, cond := range guard.Status.Conditions {
		if cond.Type == condType && cond.Status == status {
			condition.LastTransitionTime = cond.LastTransitionTime
			break
		}
	}

	// Find and update existing condition or append new one
	found := false
	for i, cond := range guard.Status.Conditions {
		if cond.Type == condType {
			guard.Status.Conditions[i] = condition
			found = true
			break
		}
	}
	if !found {
		guard.Status.Conditions = append(guard.Status.Conditions, condition)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *FabricCheckpointGuardReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tensorreaperv1.FabricCheckpointGuard{}).
		Complete(r)
}
