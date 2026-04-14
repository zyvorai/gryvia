package controllers

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	tensorreaperv1 "github.com/ssahani/TensorReaper/operators/ai-operator/api/v1"
)

// FabricRetryPolicyReconciler reconciles a FabricRetryPolicy object
type FabricRetryPolicyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabricretrypolicies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabricretrypolicies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabricretrypolicies/finalizers,verbs=update
//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabricaijobs,verbs=get;list;watch;update;patch

// Reconcile is part of the main kubernetes reconciliation loop
func (r *FabricRetryPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("fabricretrypolicy", req.NamespacedName)

	// Fetch the FabricRetryPolicy instance
	policy := &tensorreaperv1.FabricRetryPolicy{}
	err := r.Get(ctx, req.NamespacedName, policy)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("FabricRetryPolicy resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get FabricRetryPolicy")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !policy.ObjectMeta.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	// Initialize status state
	if policy.Status.State == "" {
		policy.Status.State = "active"
		if err := r.Status().Update(ctx, policy); err != nil {
			log.Error(err, "Failed to update initial status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Reconcile retry policy
	result, err := r.reconcileRetryPolicy(ctx, policy)
	if err != nil {
		log.Error(err, "Failed to reconcile retry policy")
		return result, err
	}

	return result, nil
}

func (r *FabricRetryPolicyReconciler) reconcileRetryPolicy(ctx context.Context, policy *tensorreaperv1.FabricRetryPolicy) (ctrl.Result, error) {
	log := r.Log.WithValues("fabricretrypolicy", policy.Name)

	// Check circuit breaker state
	if policy.Status.State == "circuit-open" {
		if r.shouldResetCircuitBreaker(policy) {
			log.Info("Resetting circuit breaker")
			policy.Status.State = "active"
		} else {
			log.Info("Circuit breaker is open, skipping retry evaluation")
			if err := r.Status().Update(ctx, policy); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
		}
	}

	// Check budget
	if policy.Status.State == "budget-exceeded" {
		log.Info("Retry budget exceeded, no more retries")
		if err := r.Status().Update(ctx, policy); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
	}

	// List failed jobs that reference this retry policy
	jobList := &tensorreaperv1.FabricAIJobList{}
	if err := r.List(ctx, jobList); err != nil {
		log.Error(err, "Failed to list FabricAIJobs")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	retried := false
	for i := range jobList.Items {
		job := &jobList.Items[i]

		// Only process failed jobs
		if job.Status.Phase != PhaseFailed {
			continue
		}

		// Check if this job should be retried with this policy
		if !r.shouldRetryJob(policy, job) {
			continue
		}

		// Check retry limit
		maxRetries := policy.Spec.MaxRetries
		if maxRetries == 0 {
			maxRetries = 3
		}
		if job.Status.Retries >= maxRetries {
			log.Info("Job has exceeded max retries", "job", job.Name, "retries", job.Status.Retries, "max", maxRetries)
			continue
		}

		// Check if failure type matches retry conditions
		if !r.matchesRetryConditions(policy, job) {
			continue
		}

		// Check cooldown / backoff
		delay := r.calculateBackoff(policy, job.Status.Retries)
		if job.Status.CompletionTime != nil {
			elapsed := time.Since(job.Status.CompletionTime.Time)
			if elapsed < delay {
				log.Info("Backoff not expired, waiting", "job", job.Name, "remaining", delay-elapsed)
				continue
			}
		}

		// Retry the job
		log.Info("Retrying failed job", "job", job.Name, "retry", job.Status.Retries+1, "maxRetries", maxRetries)
		job.Status.Phase = PhasePending
		job.Status.Retries++
		job.Status.CompletionTime = nil

		if err := r.Status().Update(ctx, job); err != nil {
			log.Error(err, "Failed to update job for retry", "job", job.Name)
			continue
		}

		retried = true

		// Update policy status
		now := metav1.Now()
		policy.Status.Attempts++
		policy.Status.LastAttemptTime = &now
	}

	// Calculate success rate
	if policy.Status.Attempts > 0 {
		// Count successful retried jobs
		var successCount int32
		for _, job := range jobList.Items {
			if job.Status.Phase == PhaseSucceeded && job.Status.Retries > 0 {
				successCount++
			}
		}
		policy.Status.SuccessRate = float64(successCount) / float64(policy.Status.Attempts) * 100
	}

	// Check circuit breaker
	if policy.Spec.CircuitBreaker != nil && policy.Spec.CircuitBreaker.Enabled {
		if r.shouldOpenCircuitBreaker(policy, jobList) {
			log.Info("Opening circuit breaker due to consecutive failures")
			policy.Status.State = "circuit-open"
		}
	}

	// Update status
	if err := r.Status().Update(ctx, policy); err != nil {
		log.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	requeueAfter := 30 * time.Second
	if retried {
		requeueAfter = 10 * time.Second
	}

	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

func (r *FabricRetryPolicyReconciler) shouldRetryJob(policy *tensorreaperv1.FabricRetryPolicy, job *tensorreaperv1.FabricAIJob) bool {
	// Check if job has a label or annotation referencing this policy
	labels := job.GetLabels()
	if labels != nil {
		if labels["tensorreaper.ai/retry-policy"] == policy.Name {
			return true
		}
	}

	annotations := job.GetAnnotations()
	if annotations != nil {
		if annotations["tensorreaper.ai/retry-policy"] == policy.Name {
			return true
		}
	}

	return false
}

func (r *FabricRetryPolicyReconciler) matchesRetryConditions(policy *tensorreaperv1.FabricRetryPolicy, job *tensorreaperv1.FabricAIJob) bool {
	// Check noRetryOn conditions first
	if policy.Spec.NoRetryOn != nil {
		// Check exit codes to NOT retry on
		for _, code := range policy.Spec.NoRetryOn.ExitCodes {
			// Check job message for exit code
			if job.Status.Message != "" {
				if strings.Contains(job.Status.Message, fmt.Sprintf("exit code: %d", code)) {
					return false
				}
			}
		}

		// Check error messages to NOT retry on
		for _, errMsg := range policy.Spec.NoRetryOn.Errors {
			if job.Status.Message != "" && strings.Contains(job.Status.Message, errMsg) {
				return false
			}
		}
	}

	// If retryOn is not specified, retry on all failures
	if policy.Spec.RetryOn == nil {
		return true
	}

	// Check retryOn conditions
	if len(policy.Spec.RetryOn.Conditions) > 0 {
		for _, cond := range policy.Spec.RetryOn.Conditions {
			if !cond.Enabled {
				continue
			}
			// Match failure type from job status message
			switch cond.Type {
			case "OutOfMemory":
				if strings.Contains(strings.ToLower(job.Status.Message), "out of memory") ||
					strings.Contains(strings.ToLower(job.Status.Message), "oom") {
					return true
				}
			case "NodeFailure":
				if strings.Contains(strings.ToLower(job.Status.Message), "node failure") ||
					strings.Contains(strings.ToLower(job.Status.Message), "node not ready") {
					return true
				}
			case "NetworkError":
				if strings.Contains(strings.ToLower(job.Status.Message), "network") ||
					strings.Contains(strings.ToLower(job.Status.Message), "connection") {
					return true
				}
			case "SpotInterruption":
				if strings.Contains(strings.ToLower(job.Status.Message), "spot") ||
					strings.Contains(strings.ToLower(job.Status.Message), "preempt") {
					return true
				}
			case "StorageError":
				if strings.Contains(strings.ToLower(job.Status.Message), "storage") ||
					strings.Contains(strings.ToLower(job.Status.Message), "disk") {
					return true
				}
			case "Timeout":
				if strings.Contains(strings.ToLower(job.Status.Message), "timeout") {
					return true
				}
			}
		}
		return false
	}

	return true
}

func (r *FabricRetryPolicyReconciler) calculateBackoff(policy *tensorreaperv1.FabricRetryPolicy, retryCount int32) time.Duration {
	if policy.Spec.Backoff == nil {
		return 1 * time.Minute
	}

	initialDelay := 1 * time.Minute
	if policy.Spec.Backoff.InitialDelay != "" {
		if parsed, err := time.ParseDuration(policy.Spec.Backoff.InitialDelay); err == nil {
			initialDelay = parsed
		}
	}

	maxDelay := 1 * time.Hour
	if policy.Spec.Backoff.MaxDelay != "" {
		if parsed, err := time.ParseDuration(policy.Spec.Backoff.MaxDelay); err == nil {
			maxDelay = parsed
		}
	}

	var delay time.Duration
	switch policy.Spec.Backoff.Type {
	case "fixed":
		delay = initialDelay

	case "exponential":
		multiplier := policy.Spec.Backoff.Multiplier
		if multiplier <= 0 {
			multiplier = 2
		}
		delay = time.Duration(float64(initialDelay) * math.Pow(multiplier, float64(retryCount)))

	case "fibonacci":
		delay = initialDelay * time.Duration(fibonacci(int(retryCount)+1))

	case "random":
		delay = initialDelay

	default:
		delay = initialDelay
	}

	if delay > maxDelay {
		delay = maxDelay
	}

	return delay
}

func fibonacci(n int) int {
	if n <= 0 {
		return 0
	}
	if n == 1 {
		return 1
	}
	a, b := 0, 1
	for i := 2; i <= n; i++ {
		a, b = b, a+b
	}
	return b
}

func (r *FabricRetryPolicyReconciler) shouldOpenCircuitBreaker(policy *tensorreaperv1.FabricRetryPolicy, jobList *tensorreaperv1.FabricAIJobList) bool {
	if policy.Spec.CircuitBreaker == nil || !policy.Spec.CircuitBreaker.Enabled {
		return false
	}

	threshold := policy.Spec.CircuitBreaker.FailureThreshold
	if threshold <= 0 {
		threshold = 5
	}

	// Count consecutive recent failures
	var consecutiveFailures int32
	for _, job := range jobList.Items {
		if job.Status.Phase == PhaseFailed && job.Status.Retries > 0 {
			consecutiveFailures++
		}
	}

	return consecutiveFailures >= threshold
}

func (r *FabricRetryPolicyReconciler) shouldResetCircuitBreaker(policy *tensorreaperv1.FabricRetryPolicy) bool {
	if policy.Spec.CircuitBreaker == nil || policy.Status.LastAttemptTime == nil {
		return true
	}

	resetTimeout := 1 * time.Hour
	if policy.Spec.CircuitBreaker.ResetTimeout != "" {
		if parsed, err := time.ParseDuration(policy.Spec.CircuitBreaker.ResetTimeout); err == nil {
			resetTimeout = parsed
		}
	}

	return time.Since(policy.Status.LastAttemptTime.Time) >= resetTimeout
}

// SetupWithManager sets up the controller with the Manager.
func (r *FabricRetryPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tensorreaperv1.FabricRetryPolicy{}).
		Complete(r)
}
