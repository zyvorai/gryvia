package controllers

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	tensorreaperv1 "github.com/ssahani/TensorReaper/operators/ai-operator/api/v1"
	"github.com/ssahani/TensorReaper/operators/ai-operator/pkg/tuner"
)

const (
	// Auto-tuner phases
	PhaseTuning = "Tuning"

	// Condition types for auto-tuner
	ConditionTrialsRunning = "TrialsRunning"
	ConditionBestFound     = "BestTrialFound"
)

// FabricAutoTunerReconciler reconciles a FabricAutoTuner object
type FabricAutoTunerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabricautotuners,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabricautotuners/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabricautotuners/finalizers,verbs=update
//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabricaijobs,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop
func (r *FabricAutoTunerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("fabricautotuner", req.NamespacedName)

	// Fetch the FabricAutoTuner instance
	at := &tensorreaperv1.FabricAutoTuner{}
	err := r.Get(ctx, req.NamespacedName, at)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("FabricAutoTuner resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get FabricAutoTuner")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !at.ObjectMeta.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	// Initialize status if needed
	if at.Status.Phase == "" {
		at.Status.Phase = PhasePending
		now := metav1.Now()
		at.Status.StartTime = &now
		if err := r.Status().Update(ctx, at); err != nil {
			log.Error(err, "Failed to initialize FabricAutoTuner status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Skip if already completed
	if at.Status.Phase == PhaseSucceeded || at.Status.Phase == PhaseFailed {
		return ctrl.Result{}, nil
	}

	// Reconcile the auto-tuner
	result, err := r.reconcileAutoTuner(ctx, at)
	if err != nil {
		log.Error(err, "Failed to reconcile auto-tuner")
		return result, err
	}

	return result, nil
}

func (r *FabricAutoTunerReconciler) reconcileAutoTuner(ctx context.Context, at *tensorreaperv1.FabricAutoTuner) (ctrl.Result, error) {
	log := r.Log.WithValues("fabricautotuner", at.Name)

	// Update trial statuses from their corresponding FabricAIJobs
	if err := r.syncTrialStatuses(ctx, at); err != nil {
		log.Error(err, "Failed to sync trial statuses")
		return ctrl.Result{RequeueAfter: 15 * time.Second}, err
	}

	// Count current state
	running := int32(0)
	completed := int32(0)
	failed := int32(0)
	for _, trial := range at.Status.Trials {
		switch trial.Phase {
		case tensorreaperv1.TrialPhaseRunning:
			running++
		case tensorreaperv1.TrialPhaseSucceeded:
			completed++
		case tensorreaperv1.TrialPhaseFailed:
			failed++
		case tensorreaperv1.TrialPhaseStopped:
			completed++
		}
	}
	at.Status.TrialsRunning = running
	at.Status.TrialsCompleted = completed
	at.Status.TrialsFailed = failed

	// Update best trial
	r.updateBestTrial(at)

	// Check early stopping at the tuner level (no improvement for N rounds)
	if at.Spec.EarlyStoppingRounds > 0 && r.shouldStopEarly(at) {
		at.Status.Phase = PhaseSucceeded
		at.Status.Message = fmt.Sprintf("Early stopping: no improvement in %d trials", at.Spec.EarlyStoppingRounds)
		now := metav1.Now()
		at.Status.CompletionTime = &now
		if err := r.Status().Update(ctx, at); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Check if max trials reached
	totalLaunched := int32(len(at.Status.Trials))
	if totalLaunched >= at.Spec.MaxTrials && running == 0 {
		at.Status.Phase = PhaseSucceeded
		at.Status.Message = fmt.Sprintf("All %d trials completed", at.Spec.MaxTrials)
		now := metav1.Now()
		at.Status.CompletionTime = &now
		if err := r.Status().Update(ctx, at); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// ASHA early stopping: check running trials for pruning
	if at.Spec.SearchAlgorithm == tensorreaperv1.SearchAlgorithmASHA && at.Spec.ASHAConfig != nil {
		if err := r.applyASHAEarlyStopping(ctx, at); err != nil {
			log.Error(err, "Failed to apply ASHA early stopping")
		}
	}

	// Launch new trials if we have capacity
	at.Status.Phase = PhaseTuning
	parallelism := at.Spec.Parallelism
	if parallelism <= 0 {
		parallelism = 1
	}

	slotsAvailable := parallelism - running
	trialsRemaining := at.Spec.MaxTrials - totalLaunched
	if trialsRemaining < 0 {
		trialsRemaining = 0
	}
	toLaunch := slotsAvailable
	if toLaunch > trialsRemaining {
		toLaunch = trialsRemaining
	}

	if toLaunch > 0 {
		strategy := tuner.NewSearchStrategy(at.Spec.SearchAlgorithm, at.Spec.Objective.Direction)
		candidates := strategy.GenerateCandidates(at.Spec.ParameterSpace, at.Status.Trials, int(toLaunch))

		for _, params := range candidates {
			trialIndex := len(at.Status.Trials)
			trialName := fmt.Sprintf("%s-trial-%d", at.Name, trialIndex)

			if err := r.launchTrial(ctx, at, trialName, params); err != nil {
				log.Error(err, "Failed to launch trial", "trial", trialName)
				continue
			}

			now := metav1.Now()
			at.Status.Trials = append(at.Status.Trials, tensorreaperv1.TrialResult{
				Name:       trialName,
				Parameters: params,
				Phase:      tensorreaperv1.TrialPhaseRunning,
				JobName:    trialName,
				StartTime:  &now,
			})
		}
	}

	r.updateCondition2(at, ConditionTrialsRunning, metav1.ConditionTrue, "TrialsActive",
		fmt.Sprintf("%d running, %d completed, %d failed", running, completed, failed))

	if err := r.Status().Update(ctx, at); err != nil {
		log.Error(err, "Failed to update auto-tuner status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
}

// launchTrial creates a FabricAIJob for a single trial with the given hyperparameters.
func (r *FabricAutoTunerReconciler) launchTrial(ctx context.Context, at *tensorreaperv1.FabricAutoTuner, trialName string, params map[string]string) error {
	// Build env vars from hyperparameters
	envVars := make([]corev1.EnvVar, 0, len(params)+len(at.Spec.JobTemplate.Env))
	// Copy template env vars
	envVars = append(envVars, at.Spec.JobTemplate.Env...)
	// Add hyperparameters as env vars prefixed with HP_
	for k, v := range params {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "HP_" + k,
			Value: v,
		})
	}
	// Add trial metadata
	envVars = append(envVars, corev1.EnvVar{Name: "TRIAL_NAME", Value: trialName})
	envVars = append(envVars, corev1.EnvVar{Name: "TUNER_NAME", Value: at.Name})

	job := &tensorreaperv1.FabricAIJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      trialName,
			Namespace: at.Namespace,
			Labels: map[string]string{
				"tensorreaper.ai/tuner":     at.Name,
				"tensorreaper.ai/component": "trial",
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(at, tensorreaperv1.GroupVersion.WithKind("FabricAutoTuner")),
			},
		},
		Spec: tensorreaperv1.FabricAIJobSpec{
			Type:            at.Spec.JobTemplate.Type,
			Model:           at.Spec.JobTemplate.Model,
			GPUs:            at.Spec.JobTemplate.GPUs,
			GpuType:         at.Spec.JobTemplate.GpuType,
			Storage:         at.Spec.JobTemplate.Storage,
			StorageRequest:  at.Spec.JobTemplate.StorageRequest,
			Network:         at.Spec.JobTemplate.Network,
			Image:           at.Spec.JobTemplate.Image,
			ImagePullPolicy: at.Spec.JobTemplate.ImagePullPolicy,
			Command:         at.Spec.JobTemplate.Command,
			Args:            at.Spec.JobTemplate.Args,
			Env:             envVars,
			WorkingDir:      at.Spec.JobTemplate.WorkingDir,
			Distributed:     at.Spec.JobTemplate.Distributed,
			Resources:       at.Spec.JobTemplate.Resources,
			Volumes:         at.Spec.JobTemplate.Volumes,
			VolumeMounts:    at.Spec.JobTemplate.VolumeMounts,
			Priority:        at.Spec.JobTemplate.Priority,
			NodeSelector:    at.Spec.JobTemplate.NodeSelector,
			Tolerations:     at.Spec.JobTemplate.Tolerations,
			Affinity:        at.Spec.JobTemplate.Affinity,
		},
	}

	return r.Create(ctx, job)
}

// syncTrialStatuses updates trial results based on the status of their FabricAIJobs.
func (r *FabricAutoTunerReconciler) syncTrialStatuses(ctx context.Context, at *tensorreaperv1.FabricAutoTuner) error {
	for i, trial := range at.Status.Trials {
		if trial.Phase == tensorreaperv1.TrialPhaseSucceeded ||
			trial.Phase == tensorreaperv1.TrialPhaseFailed ||
			trial.Phase == tensorreaperv1.TrialPhaseStopped {
			continue
		}

		job := &tensorreaperv1.FabricAIJob{}
		err := r.Get(ctx, types.NamespacedName{
			Namespace: at.Namespace,
			Name:      trial.JobName,
		}, job)
		if err != nil {
			if errors.IsNotFound(err) {
				at.Status.Trials[i].Phase = tensorreaperv1.TrialPhaseFailed
				at.Status.Trials[i].Message = "Trial job not found"
			}
			continue
		}

		switch job.Status.Phase {
		case PhaseSucceeded:
			at.Status.Trials[i].Phase = tensorreaperv1.TrialPhaseSucceeded
			now := metav1.Now()
			at.Status.Trials[i].CompletionTime = &now
			// Extract metric from job annotations or metrics
			if job.Status.Metrics != nil {
				metricValue := job.Status.Metrics.Loss
				at.Status.Trials[i].MetricValue = &metricValue
			}
			// Also check for metric in job annotations
			if val, ok := job.Annotations[fmt.Sprintf("tensorreaper.ai/metric-%s", at.Spec.Objective.MetricName)]; ok {
				if parsed, err := strconv.ParseFloat(val, 64); err == nil {
					at.Status.Trials[i].MetricValue = &parsed
				}
			}

		case PhaseFailed:
			at.Status.Trials[i].Phase = tensorreaperv1.TrialPhaseFailed
			now := metav1.Now()
			at.Status.Trials[i].CompletionTime = &now
			at.Status.Trials[i].Message = job.Status.Message

		case PhaseRunning:
			at.Status.Trials[i].Phase = tensorreaperv1.TrialPhaseRunning
			// Collect intermediate metrics for ASHA
			if job.Status.Metrics != nil && job.Status.Metrics.Epoch > 0 {
				im := tensorreaperv1.IntermediateMetric{
					Step:  job.Status.Metrics.Epoch,
					Value: job.Status.Metrics.Loss,
				}
				// Append only if this step hasn't been recorded yet
				existing := at.Status.Trials[i].IntermediateMetrics
				alreadyRecorded := false
				for _, e := range existing {
					if e.Step == im.Step {
						alreadyRecorded = true
						break
					}
				}
				if !alreadyRecorded {
					at.Status.Trials[i].IntermediateMetrics = append(at.Status.Trials[i].IntermediateMetrics, im)
				}
			}
		}
	}
	return nil
}

// updateBestTrial selects the best trial based on the objective.
func (r *FabricAutoTunerReconciler) updateBestTrial(at *tensorreaperv1.FabricAutoTuner) {
	var best *tensorreaperv1.TrialResult

	for i := range at.Status.Trials {
		trial := &at.Status.Trials[i]
		if trial.MetricValue == nil {
			continue
		}
		if best == nil || best.MetricValue == nil {
			best = trial
			continue
		}

		if at.Spec.Objective.Direction == tensorreaperv1.ObjectiveMinimize {
			if *trial.MetricValue < *best.MetricValue {
				best = trial
			}
		} else {
			if *trial.MetricValue > *best.MetricValue {
				best = trial
			}
		}
	}

	if best != nil {
		bestCopy := *best
		at.Status.BestTrial = &bestCopy
	}
}

// shouldStopEarly returns true if the last N completed trials showed no improvement.
func (r *FabricAutoTunerReconciler) shouldStopEarly(at *tensorreaperv1.FabricAutoTuner) bool {
	if at.Status.BestTrial == nil || at.Status.BestTrial.MetricValue == nil {
		return false
	}

	rounds := at.Spec.EarlyStoppingRounds
	if rounds <= 0 {
		return false
	}

	// Count consecutive completed trials after the best trial without improvement
	bestValue := *at.Status.BestTrial.MetricValue
	bestName := at.Status.BestTrial.Name
	foundBest := false
	noImprovement := int32(0)

	for _, trial := range at.Status.Trials {
		if trial.Name == bestName {
			foundBest = true
			continue
		}
		if !foundBest {
			continue
		}
		if trial.MetricValue == nil {
			continue
		}

		improved := false
		if at.Spec.Objective.Direction == tensorreaperv1.ObjectiveMinimize {
			improved = *trial.MetricValue < bestValue
		} else {
			improved = *trial.MetricValue > bestValue
		}

		if improved {
			return false // Found an improvement, reset
		}
		noImprovement++
	}

	return noImprovement >= rounds
}

// applyASHAEarlyStopping checks running trials and stops underperforming ones.
func (r *FabricAutoTunerReconciler) applyASHAEarlyStopping(ctx context.Context, at *tensorreaperv1.FabricAutoTuner) error {
	scheduler := &tuner.ASHAScheduler{
		MaxEpochs:       at.Spec.ASHAConfig.MaxEpochs,
		ReductionFactor: at.Spec.ASHAConfig.ReductionFactor,
		MinResource:     at.Spec.ASHAConfig.MinResource,
	}

	for i, trial := range at.Status.Trials {
		if trial.Phase != tensorreaperv1.TrialPhaseRunning {
			continue
		}

		if scheduler.ShouldStop(trial, at.Status.Trials, at.Spec.Objective.Direction) {
			// Delete the trial job to stop it
			job := &tensorreaperv1.FabricAIJob{}
			err := r.Get(ctx, types.NamespacedName{
				Namespace: at.Namespace,
				Name:      trial.JobName,
			}, job)
			if err == nil {
				if deleteErr := r.Delete(ctx, job); deleteErr != nil && !errors.IsNotFound(deleteErr) {
					return deleteErr
				}
			}

			at.Status.Trials[i].Phase = tensorreaperv1.TrialPhaseStopped
			at.Status.Trials[i].Message = "Stopped by ASHA early stopping"
			now := metav1.Now()
			at.Status.Trials[i].CompletionTime = &now
		}
	}
	return nil
}

func (r *FabricAutoTunerReconciler) updateCondition2(at *tensorreaperv1.FabricAutoTuner, condType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: at.Generation,
		LastTransitionTime: metav1.Now(),
	}

	for _, cond := range at.Status.Conditions {
		if cond.Type == condType && cond.Status == status {
			condition.LastTransitionTime = cond.LastTransitionTime
			break
		}
	}

	found := false
	for i, cond := range at.Status.Conditions {
		if cond.Type == condType {
			at.Status.Conditions[i] = condition
			found = true
			break
		}
	}
	if !found {
		at.Status.Conditions = append(at.Status.Conditions, condition)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *FabricAutoTunerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tensorreaperv1.FabricAutoTuner{}).
		Owns(&tensorreaperv1.FabricAIJob{}).
		Complete(r)
}
