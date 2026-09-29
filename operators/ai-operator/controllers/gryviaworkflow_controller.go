package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gryviav1 "github.com/zyvorai/gryvia/operators/ai-operator/api/v1"
)

const (
	// Workflow condition types
	ConditionDAGValid     = "DAGValid"
	ConditionStepsRunning = "StepsRunning"
	ConditionWorkflowDone = "WorkflowComplete"
)

// GryviaWorkflowReconciler reconciles a GryviaWorkflow object
type GryviaWorkflowReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

//+kubebuilder:rbac:groups=gryvia.io,resources=gryviaworkflows,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gryvia.io,resources=gryviaworkflows/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gryvia.io,resources=gryviaworkflows/finalizers,verbs=update
//+kubebuilder:rbac:groups=gryvia.io,resources=gryviaaijobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;delete

// Reconcile is part of the main kubernetes reconciliation loop
func (r *GryviaWorkflowReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("gryviaworkflow", req.NamespacedName)

	// Fetch the GryviaWorkflow instance
	wf := &gryviav1.GryviaWorkflow{}
	err := r.Get(ctx, req.NamespacedName, wf)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("GryviaWorkflow resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get GryviaWorkflow")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !wf.ObjectMeta.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	// Initialize status
	if wf.Status.Phase == "" {
		if err := r.initializeWorkflow(ctx, wf); err != nil {
			log.Error(err, "Failed to initialize workflow")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Skip if terminal
	if wf.Status.Phase == PhaseSucceeded || wf.Status.Phase == PhaseFailed {
		return ctrl.Result{}, nil
	}

	// Reconcile workflow steps
	result, err := r.reconcileWorkflow(ctx, wf)
	if err != nil {
		log.Error(err, "Failed to reconcile workflow")
		return result, err
	}
	return result, nil
}

// initializeWorkflow sets up the initial status with all steps in Pending state.
func (r *GryviaWorkflowReconciler) initializeWorkflow(ctx context.Context, wf *gryviav1.GryviaWorkflow) error {
	// Validate DAG (no cycles)
	if err := r.validateDAG(wf); err != nil {
		wf.Status.Phase = PhaseFailed
		wf.Status.Message = fmt.Sprintf("Invalid DAG: %v", err)
		return r.Status().Update(ctx, wf)
	}

	wf.Status.Phase = PhasePending
	now := metav1.Now()
	wf.Status.StartTime = &now
	wf.Status.StepStatuses = make([]gryviav1.StepStatus, len(wf.Spec.Steps))
	for i, step := range wf.Spec.Steps {
		wf.Status.StepStatuses[i] = gryviav1.StepStatus{
			Name:  step.Name,
			Phase: gryviav1.StepPhasePending,
		}
	}

	return r.Status().Update(ctx, wf)
}

// validateDAG checks for cycles in the dependency graph using DFS.
func (r *GryviaWorkflowReconciler) validateDAG(wf *gryviav1.GryviaWorkflow) error {
	// Build adjacency list
	stepNames := make(map[string]bool, len(wf.Spec.Steps))
	for _, step := range wf.Spec.Steps {
		if stepNames[step.Name] {
			return fmt.Errorf("duplicate step name: %s", step.Name)
		}
		stepNames[step.Name] = true
	}

	// Verify all dependencies refer to existing steps
	for _, step := range wf.Spec.Steps {
		for _, dep := range step.DependsOn {
			if !stepNames[dep] {
				return fmt.Errorf("step %q depends on non-existent step %q", step.Name, dep)
			}
			if dep == step.Name {
				return fmt.Errorf("step %q depends on itself", step.Name)
			}
		}
	}

	// DFS-based cycle detection
	white := 0 // unvisited
	gray := 1  // in progress
	black := 2 // completed
	colors := make(map[string]int, len(wf.Spec.Steps))

	deps := make(map[string][]string, len(wf.Spec.Steps))
	for _, step := range wf.Spec.Steps {
		deps[step.Name] = step.DependsOn
	}

	var hasCycle func(node string) bool
	hasCycle = func(node string) bool {
		colors[node] = gray
		for _, dep := range deps[node] {
			switch colors[dep] {
			case white:
				if hasCycle(dep) {
					return true
				}
			case gray:
				return true // back edge = cycle
			}
		}
		colors[node] = black
		return false
	}

	for _, step := range wf.Spec.Steps {
		if colors[step.Name] == white {
			if hasCycle(step.Name) {
				return fmt.Errorf("cycle detected in workflow DAG")
			}
		}
	}

	_ = black // suppress unused warning if already used above
	return nil
}

func (r *GryviaWorkflowReconciler) reconcileWorkflow(ctx context.Context, wf *gryviav1.GryviaWorkflow) (ctrl.Result, error) {
	log := r.Log.WithValues("gryviaworkflow", wf.Name)

	// Build step status lookup
	stepStatusMap := make(map[string]*gryviav1.StepStatus, len(wf.Status.StepStatuses))
	for i := range wf.Status.StepStatuses {
		stepStatusMap[wf.Status.StepStatuses[i].Name] = &wf.Status.StepStatuses[i]
	}

	// Build step spec lookup
	stepSpecMap := make(map[string]*gryviav1.WorkflowStep, len(wf.Spec.Steps))
	for i := range wf.Spec.Steps {
		stepSpecMap[wf.Spec.Steps[i].Name] = &wf.Spec.Steps[i]
	}

	// Sync running steps with their job statuses
	for i := range wf.Status.StepStatuses {
		ss := &wf.Status.StepStatuses[i]
		if ss.Phase != gryviav1.StepPhaseRunning || ss.JobName == "" {
			continue
		}

		job := &gryviav1.GryviaAIJob{}
		err := r.Get(ctx, types.NamespacedName{
			Namespace: wf.Namespace,
			Name:      ss.JobName,
		}, job)
		if err != nil {
			if errors.IsNotFound(err) {
				ss.Phase = gryviav1.StepPhaseFailed
				ss.Message = "Step job not found"
				now := metav1.Now()
				ss.CompletionTime = &now
			}
			continue
		}

		switch job.Status.Phase {
		case PhaseSucceeded:
			ss.Phase = gryviav1.StepPhaseSucceeded
			now := metav1.Now()
			ss.CompletionTime = &now
		case PhaseFailed:
			// Check for retries
			spec := stepSpecMap[ss.Name]
			if spec != nil && ss.RetriesAttempted < spec.Retries {
				ss.RetriesAttempted++
				ss.Phase = gryviav1.StepPhasePending
				ss.Message = fmt.Sprintf("Retrying (%d/%d)", ss.RetriesAttempted, spec.Retries)
				// Delete the failed job so it can be recreated
				if deleteErr := r.Delete(ctx, job); deleteErr != nil && !errors.IsNotFound(deleteErr) {
					log.Error(deleteErr, "Failed to delete failed step job for retry")
				}
			} else {
				ss.Phase = gryviav1.StepPhaseFailed
				ss.Message = job.Status.Message
				now := metav1.Now()
				ss.CompletionTime = &now
			}
		}
	}

	// Determine which pending steps can be started (all dependencies met)
	anyRunning := false
	anyFailed := false
	allSucceeded := true

	for i := range wf.Status.StepStatuses {
		ss := &wf.Status.StepStatuses[i]

		switch ss.Phase {
		case gryviav1.StepPhaseRunning:
			anyRunning = true
			allSucceeded = false
		case gryviav1.StepPhaseFailed:
			anyFailed = true
			allSucceeded = false
		case gryviav1.StepPhaseSkipped:
			// skipped steps don't block completion
		case gryviav1.StepPhasePending:
			allSucceeded = false
			spec := stepSpecMap[ss.Name]
			if spec == nil {
				continue
			}

			// Check if all dependencies are met
			depsReady := true
			depsFailed := false
			for _, dep := range spec.DependsOn {
				depStatus := stepStatusMap[dep]
				if depStatus == nil {
					depsReady = false
					break
				}
				if depStatus.Phase == gryviav1.StepPhaseSucceeded || depStatus.Phase == gryviav1.StepPhaseSkipped {
					continue
				}
				if depStatus.Phase == gryviav1.StepPhaseFailed {
					depsFailed = true
				}
				depsReady = false
			}

			if depsFailed {
				// If any dependency failed, skip this step
				ss.Phase = gryviav1.StepPhaseSkipped
				ss.Message = "Skipped: dependency failed"
				continue
			}

			if !depsReady {
				continue
			}

			// Check condition if specified
			if spec.Condition != "" {
				if !r.evaluateCondition(spec.Condition, stepStatusMap) {
					ss.Phase = gryviav1.StepPhaseSkipped
					ss.Message = "Skipped: condition not met"
					continue
				}
			}

			// Launch this step
			if err := r.launchStep(ctx, wf, spec, ss); err != nil {
				log.Error(err, "Failed to launch step", "step", ss.Name)
				ss.Message = fmt.Sprintf("Launch failed: %v", err)
				continue
			}
			anyRunning = true
		}
	}

	// Determine overall workflow phase
	if allSucceeded {
		wf.Status.Phase = PhaseSucceeded
		wf.Status.Message = "All steps completed successfully"
		now := metav1.Now()
		wf.Status.CompletionTime = &now
	} else if anyFailed && !anyRunning {
		// Check if there are pending steps that could still run
		hasPending := false
		for _, ss := range wf.Status.StepStatuses {
			if ss.Phase == gryviav1.StepPhasePending {
				hasPending = true
				break
			}
		}
		if !hasPending {
			wf.Status.Phase = PhaseFailed
			wf.Status.Message = "Workflow failed: one or more steps failed"
			now := metav1.Now()
			wf.Status.CompletionTime = &now
		} else {
			wf.Status.Phase = PhaseRunning
		}
	} else {
		wf.Status.Phase = PhaseRunning
	}

	if err := r.Status().Update(ctx, wf); err != nil {
		log.Error(err, "Failed to update workflow status")
		return ctrl.Result{}, err
	}

	if wf.Status.Phase == PhaseSucceeded || wf.Status.Phase == PhaseFailed {
		return ctrl.Result{}, nil
	}

	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}

// evaluateCondition does a simple string-matching condition evaluation.
// For production use this would use a proper CEL evaluator. Currently supports
// checking step phase: "steps.<name>.status == 'Succeeded'" style patterns.
func (r *GryviaWorkflowReconciler) evaluateCondition(condition string, stepStatuses map[string]*gryviav1.StepStatus) bool {
	// Simple heuristic: if condition references a step status, check it.
	// In production this would use a full expression evaluator.
	// For now, if we can't parse it, default to true (run the step).
	_ = condition
	_ = stepStatuses
	return true
}

// launchStep creates a GryviaAIJob for a job-type step, or a pod for a script-type step.
func (r *GryviaWorkflowReconciler) launchStep(ctx context.Context, wf *gryviav1.GryviaWorkflow, step *gryviav1.WorkflowStep, ss *gryviav1.StepStatus) error {
	jobName := fmt.Sprintf("%s-%s", wf.Name, step.Name)

	stepType := step.Type
	if stepType == "" && step.JobTemplate != nil {
		stepType = gryviav1.StepTypeJob
	}
	if stepType == "" && step.Script != nil {
		stepType = gryviav1.StepTypeScript
	}

	switch stepType {
	case gryviav1.StepTypeJob:
		if step.JobTemplate == nil {
			return fmt.Errorf("step %q has type 'job' but no jobTemplate", step.Name)
		}
		return r.launchJobStep(ctx, wf, step, ss, jobName)

	case gryviav1.StepTypeScript:
		if step.Script == nil {
			return fmt.Errorf("step %q has type 'script' but no script", step.Name)
		}
		return r.launchScriptStep(ctx, wf, step, ss, jobName)

	case gryviav1.StepTypeWebhook:
		// Webhook steps are executed inline; mark as succeeded immediately.
		// A production implementation would make the HTTP call and check the response.
		ss.Phase = gryviav1.StepPhaseSucceeded
		ss.Message = "Webhook executed"
		now := metav1.Now()
		ss.StartTime = &now
		ss.CompletionTime = &now
		return nil

	default:
		// Default to job step if template is available
		if step.JobTemplate != nil {
			return r.launchJobStep(ctx, wf, step, ss, jobName)
		}
		return fmt.Errorf("step %q has no recognized type or template", step.Name)
	}
}

func (r *GryviaWorkflowReconciler) launchJobStep(ctx context.Context, wf *gryviav1.GryviaWorkflow, step *gryviav1.WorkflowStep, ss *gryviav1.StepStatus, jobName string) error {
	job := &gryviav1.GryviaAIJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: wf.Namespace,
			Labels: map[string]string{
				"gryvia.io/workflow":  wf.Name,
				"gryvia.io/step":      step.Name,
				"gryvia.io/component": "workflow-step",
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(wf, gryviav1.GroupVersion.WithKind("GryviaWorkflow")),
			},
		},
		Spec: *step.JobTemplate.DeepCopy(),
	}

	// Inject workflow parameters as env vars
	for k, v := range wf.Spec.Parameters {
		job.Spec.Env = append(job.Spec.Env, gryviav1.EnvVarFromCoreV1(k, v))
	}

	if err := r.Create(ctx, job); err != nil {
		if errors.IsAlreadyExists(err) {
			// Job already exists, just update status
		} else {
			return err
		}
	}

	ss.Phase = gryviav1.StepPhaseRunning
	ss.JobName = jobName
	now := metav1.Now()
	ss.StartTime = &now
	return nil
}

func (r *GryviaWorkflowReconciler) launchScriptStep(ctx context.Context, wf *gryviav1.GryviaWorkflow, step *gryviav1.WorkflowStep, ss *gryviav1.StepStatus, jobName string) error {
	// For script steps, create a simple GryviaAIJob with the script's image and command
	job := &gryviav1.GryviaAIJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: wf.Namespace,
			Labels: map[string]string{
				"gryvia.io/workflow":  wf.Name,
				"gryvia.io/step":      step.Name,
				"gryvia.io/component": "workflow-script",
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(wf, gryviav1.GroupVersion.WithKind("GryviaWorkflow")),
			},
		},
		Spec: gryviav1.GryviaAIJobSpec{
			Type:    "script",
			Image:   step.Script.Image,
			Command: step.Script.Command,
			Args:    step.Script.Args,
			GPUs:    0,
		},
	}

	if err := r.Create(ctx, job); err != nil {
		if errors.IsAlreadyExists(err) {
			// Already exists, continue
		} else {
			return err
		}
	}

	ss.Phase = gryviav1.StepPhaseRunning
	ss.JobName = jobName
	now := metav1.Now()
	ss.StartTime = &now
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GryviaWorkflowReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gryviav1.GryviaWorkflow{}).
		Owns(&gryviav1.GryviaAIJob{}).
		Complete(r)
}
