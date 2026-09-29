package controllers

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gryviav1 "github.com/zyvorai/gryvia/operators/ai-operator/api/v1"
)

const (
	maxPreemptionHistory = 50
	// Default grace period for preempted jobs to checkpoint (seconds)
	defaultGracePeriodSeconds = 60
)

// FabricPriorityReconciler reconciles a FabricPriority object
type FabricPriorityReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

//+kubebuilder:rbac:groups=gryvia.io,resources=fabricpriorities,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gryvia.io,resources=fabricpriorities/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gryvia.io,resources=fabricpriorities/finalizers,verbs=update
//+kubebuilder:rbac:groups=gryvia.io,resources=fabricaijobs,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *FabricPriorityReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("fabricpriority", req.NamespacedName)

	// Fetch the FabricPriority instance
	priority := &gryviav1.FabricPriority{}
	err := r.Get(ctx, req.NamespacedName, priority)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("FabricPriority resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get FabricPriority")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !priority.ObjectMeta.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	log.Info("Reconciling FabricPriority", "name", priority.Name, "value", priority.Spec.Value)

	// Reconcile the priority
	result, err := r.reconcilePriority(ctx, priority)
	if err != nil {
		log.Error(err, "Failed to reconcile priority")
		return result, err
	}

	return result, nil
}

func (r *FabricPriorityReconciler) reconcilePriority(ctx context.Context, priority *gryviav1.FabricPriority) (ctrl.Result, error) {
	log := r.Log.WithValues("priority", priority.Name)

	// List all jobs
	jobList := &gryviav1.FabricAIJobList{}
	if err := r.List(ctx, jobList); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to list FabricAIJobs: %w", err)
	}

	// Count active and queued jobs with this priority
	activeJobs := 0
	queuedJobs := 0
	var queueTimes []time.Duration

	for _, job := range jobList.Items {
		jobPriority := r.getJobPriorityClass(job)
		if jobPriority != priority.Name {
			continue
		}

		switch job.Status.Phase {
		case "Running":
			activeJobs++
		case "Pending", "Queued":
			queuedJobs++
			queueTime := time.Since(job.CreationTimestamp.Time)
			queueTimes = append(queueTimes, queueTime)
		}
	}

	priority.Status.ActiveJobs = activeJobs
	priority.Status.QueuedJobs = queuedJobs

	// Calculate average queue time
	if len(queueTimes) > 0 {
		total := time.Duration(0)
		for _, qt := range queueTimes {
			total += qt
		}
		avg := total / time.Duration(len(queueTimes))
		priority.Status.AvgQueueTime = avg.Round(time.Second).String()
	} else {
		priority.Status.AvgQueueTime = "0s"
	}

	// Check if preemption is needed
	if priority.Spec.PreemptionPolicy == "PreemptLowerPriority" {
		r.evaluatePreemption(ctx, priority, jobList)
	}

	// Update condition
	r.updatePriorityCondition(priority, "Ready", metav1.ConditionTrue, "PriorityActive",
		fmt.Sprintf("Priority %s (value=%d): %d active, %d queued jobs",
			priority.Name, priority.Spec.Value, activeJobs, queuedJobs))

	// Update status
	if err := r.Status().Update(ctx, priority); err != nil {
		log.Error(err, "Failed to update priority status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

func (r *FabricPriorityReconciler) getJobPriorityClass(job gryviav1.FabricAIJob) string {
	// Check for priority class label or annotation
	if pc, ok := job.Labels["gryvia.io/priority-class"]; ok {
		return pc
	}
	if pc, ok := job.Annotations["gryvia.io/priority-class"]; ok {
		return pc
	}
	// Default to "normal"
	return "normal"
}

func (r *FabricPriorityReconciler) evaluatePreemption(ctx context.Context, priority *gryviav1.FabricPriority, jobList *gryviav1.FabricAIJobList) {
	log := r.Log.WithValues("priority", priority.Name)

	// Get all priority classes to build priority value map
	priorityList := &gryviav1.FabricPriorityList{}
	if err := r.List(ctx, priorityList); err != nil {
		log.Error(err, "Failed to list priority classes")
		return
	}

	priorityValues := make(map[string]int)
	for _, p := range priorityList.Items {
		priorityValues[p.Name] = p.Spec.Value
	}

	// Find pending jobs with this priority that need resources
	var pendingHighPriJobs []gryviav1.FabricAIJob
	for _, job := range jobList.Items {
		jobPriority := r.getJobPriorityClass(job)
		if jobPriority == priority.Name && (job.Status.Phase == "Pending" || job.Status.Phase == "Queued") {
			// Check SLA queue time
			if priority.Spec.SLA != nil && priority.Spec.SLA.MaxQueueTimeMinutes > 0 {
				queueTime := time.Since(job.CreationTimestamp.Time)
				maxQueueTime := time.Duration(priority.Spec.SLA.MaxQueueTimeMinutes) * time.Minute
				if queueTime > maxQueueTime {
					pendingHighPriJobs = append(pendingHighPriJobs, job)
				}
			}
		}
	}

	if len(pendingHighPriJobs) == 0 {
		return
	}

	// Find running lower-priority jobs that can be preempted
	var preemptCandidates []gryviav1.FabricAIJob
	for _, job := range jobList.Items {
		if job.Status.Phase != "Running" {
			continue
		}
		jobPriority := r.getJobPriorityClass(job)
		jobPriorityValue := priorityValues[jobPriority]

		// Only preempt jobs with strictly lower priority
		if jobPriorityValue < priority.Spec.Value {
			preemptCandidates = append(preemptCandidates, job)
		}
	}

	if len(preemptCandidates) == 0 {
		return
	}

	// Sort candidates by priority value (lowest first = preempt first)
	sort.Slice(preemptCandidates, func(i, j int) bool {
		pi := priorityValues[r.getJobPriorityClass(preemptCandidates[i])]
		pj := priorityValues[r.getJobPriorityClass(preemptCandidates[j])]
		return pi < pj
	})

	// Preempt jobs to make room for pending high-priority jobs
	for _, pendingJob := range pendingHighPriJobs {
		gpusNeeded := int(pendingJob.Spec.GPUs)
		gpusFreed := 0

		for i := range preemptCandidates {
			if gpusFreed >= gpusNeeded {
				break
			}

			candidate := &preemptCandidates[i]
			if candidate.Status.Phase != "Running" {
				continue // Already preempted
			}

			log.Info("Preempting job for higher priority job",
				"preemptedJob", candidate.Name,
				"preemptingJob", pendingJob.Name,
				"preemptedPriority", r.getJobPriorityClass(*candidate),
				"preemptingPriority", priority.Name,
			)

			// Record preemption event
			preemptionEvent := gryviav1.PreemptionEvent{
				Timestamp:            metav1.Now(),
				PreemptedJob:         candidate.Name,
				PreemptedJobPriority: priorityValues[r.getJobPriorityClass(*candidate)],
				PreemptingJob:        pendingJob.Name,
				Reason:               fmt.Sprintf("Higher priority job %s needs %d GPUs", pendingJob.Name, pendingJob.Spec.GPUs),
				GracePeriodUsed:      true,
			}
			r.addPreemptionEvent(priority, preemptionEvent)

			// Mark the candidate as preempted (in practice, this would
			// signal the job to checkpoint and then terminate it)
			candidate.Status.Phase = "Preempted"
			candidate.Status.Message = fmt.Sprintf("Preempted by higher priority job %s", pendingJob.Name)
			if err := r.Status().Update(ctx, candidate); err != nil {
				log.Error(err, "Failed to preempt job", "job", candidate.Name)
				continue
			}

			gpusFreed += int(candidate.Spec.GPUs)
			priority.Status.PreemptionEvents++
		}
	}
}

func (r *FabricPriorityReconciler) addPreemptionEvent(priority *gryviav1.FabricPriority, event gryviav1.PreemptionEvent) {
	priority.Status.PreemptionHistory = append(priority.Status.PreemptionHistory, event)

	// Maintain rolling buffer
	if len(priority.Status.PreemptionHistory) > maxPreemptionHistory {
		priority.Status.PreemptionHistory = priority.Status.PreemptionHistory[len(priority.Status.PreemptionHistory)-maxPreemptionHistory:]
	}
}

func (r *FabricPriorityReconciler) updatePriorityCondition(priority *gryviav1.FabricPriority, condType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: priority.Generation,
		LastTransitionTime: metav1.Now(),
	}

	found := false
	for i, cond := range priority.Status.Conditions {
		if cond.Type == condType {
			priority.Status.Conditions[i] = condition
			found = true
			break
		}
	}
	if !found {
		priority.Status.Conditions = append(priority.Status.Conditions, condition)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *FabricPriorityReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gryviav1.FabricPriority{}).
		Complete(r)
}
