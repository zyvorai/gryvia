package controllers

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	gryviav1 "github.com/zyvorai/gryvia/operators/ai-operator/api/v1"
)

// GryviaJobHookReconciler reconciles a GryviaJobHook object
type GryviaJobHookReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

//+kubebuilder:rbac:groups=gryvia.io,resources=gryviajobhooks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gryvia.io,resources=gryviajobhooks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gryvia.io,resources=gryviajobhooks/finalizers,verbs=update
//+kubebuilder:rbac:groups=gryvia.io,resources=gryviaaijobs,verbs=get;list;watch
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop
func (r *GryviaJobHookReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("gryviajobhook", req.NamespacedName)

	// Fetch the GryviaJobHook instance
	hook := &gryviav1.GryviaJobHook{}
	err := r.Get(ctx, req.NamespacedName, hook)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("GryviaJobHook resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get GryviaJobHook")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !hook.ObjectMeta.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	// Reconcile job hook
	result, err := r.reconcileJobHook(ctx, hook)
	if err != nil {
		log.Error(err, "Failed to reconcile job hook")
		return result, err
	}

	return result, nil
}

func (r *GryviaJobHookReconciler) reconcileJobHook(ctx context.Context, hook *gryviav1.GryviaJobHook) (ctrl.Result, error) {
	log := r.Log.WithValues("gryviajobhook", hook.Name)

	// List AI jobs matching the hook's selector
	jobList := &gryviav1.GryviaAIJobList{}
	if err := r.List(ctx, jobList); err != nil {
		log.Error(err, "Failed to list GryviaAIJobs")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	// Find matching jobs based on selector and trigger phase
	for i := range jobList.Items {
		job := &jobList.Items[i]

		// Check if job matches selector
		if !r.matchesSelector(hook, job) {
			continue
		}

		// Check if the job is in the trigger phase
		if !r.shouldTrigger(hook, job) {
			continue
		}

		// Execute the hook action
		log.Info("Executing hook", "job", job.Name, "trigger", hook.Spec.Trigger, "action", hook.Spec.Action.Type)

		err := r.executeAction(ctx, hook, job)
		if err != nil {
			log.Error(err, "Hook execution failed", "job", job.Name)

			// Handle failure policy
			now := metav1.Now()
			hook.Status.LastExecutionTime = &now
			hook.Status.ExecutionCount++
			hook.Status.LastStatus = "failed"

			if hook.Spec.FailurePolicy == "fail-job" {
				log.Info("Hook failure policy is fail-job, marking job as failed", "job", job.Name)
			}

			if updateErr := r.Status().Update(ctx, hook); updateErr != nil {
				log.Error(updateErr, "Failed to update hook status")
			}

			continue
		}

		// Update hook status on success
		now := metav1.Now()
		hook.Status.LastExecutionTime = &now
		hook.Status.ExecutionCount++
		hook.Status.LastStatus = "success"
	}

	// Update status
	if err := r.Status().Update(ctx, hook); err != nil {
		log.Error(err, "Failed to update hook status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

func (r *GryviaJobHookReconciler) matchesSelector(hook *gryviav1.GryviaJobHook, job *gryviav1.GryviaAIJob) bool {
	if hook.Spec.Selector == nil || len(hook.Spec.Selector.MatchLabels) == 0 {
		return true // no selector means match all jobs
	}

	jobLabels := job.GetLabels()
	if jobLabels == nil {
		return false
	}

	for key, value := range hook.Spec.Selector.MatchLabels {
		if jobLabels[key] != value {
			return false
		}
	}

	return true
}

func (r *GryviaJobHookReconciler) shouldTrigger(hook *gryviav1.GryviaJobHook, job *gryviav1.GryviaAIJob) bool {
	switch hook.Spec.Trigger {
	case "pre-start":
		return job.Status.Phase == PhasePending
	case "post-start":
		return job.Status.Phase == PhaseRunning && job.Status.StartTime != nil
	case "pre-completion":
		return job.Status.Phase == PhaseRunning
	case "post-completion":
		return job.Status.Phase == PhaseSucceeded
	case "on-failure":
		return job.Status.Phase == PhaseFailed
	case "on-success":
		return job.Status.Phase == PhaseSucceeded
	default:
		return false
	}
}

func (r *GryviaJobHookReconciler) executeAction(ctx context.Context, hook *gryviav1.GryviaJobHook, job *gryviav1.GryviaAIJob) error {
	switch hook.Spec.Action.Type {
	case "webhook":
		return r.executeWebhook(ctx, hook, job)
	case "k8s-job":
		return r.executeK8sJob(ctx, hook, job)
	case "notification":
		return r.executeNotification(ctx, hook, job)
	case "exec":
		return r.executeExec(ctx, hook, job)
	case "script":
		return r.executeExec(ctx, hook, job) // script is handled similarly to exec
	default:
		return fmt.Errorf("unsupported hook action type: %s", hook.Spec.Action.Type)
	}
}

func (r *GryviaJobHookReconciler) executeWebhook(ctx context.Context, hook *gryviav1.GryviaJobHook, job *gryviav1.GryviaAIJob) error {
	if hook.Spec.Action.Webhook == nil {
		return fmt.Errorf("webhook action configuration is missing")
	}

	webhook := hook.Spec.Action.Webhook

	method := webhook.Method
	if method == "" {
		method = "POST"
	}

	timeout := 30 * time.Second
	if webhook.Timeout != "" {
		if parsed, err := time.ParseDuration(webhook.Timeout); err == nil {
			timeout = parsed
		}
	}

	body := webhook.Body
	if body == "" {
		body = fmt.Sprintf(`{"job":"%s","trigger":"%s","phase":"%s"}`, job.Name, hook.Spec.Trigger, job.Status.Phase)
	}

	httpClient := &http.Client{Timeout: timeout}
	req, err := http.NewRequestWithContext(ctx, method, webhook.URL, bytes.NewBufferString(body))
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %w", err)
	}

	// Set headers
	for key, value := range webhook.Headers {
		req.Header.Set(key, value)
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("webhook request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned error status: %d", resp.StatusCode)
	}

	return nil
}

func (r *GryviaJobHookReconciler) executeK8sJob(ctx context.Context, hook *gryviav1.GryviaJobHook, aiJob *gryviav1.GryviaAIJob) error {
	if hook.Spec.Action.K8sJob == nil {
		return fmt.Errorf("k8s-job action configuration is missing")
	}

	k8sJobConfig := hook.Spec.Action.K8sJob
	jobName := fmt.Sprintf("%s-%s-hook", aiJob.Name, hook.Name)

	// Check if the job already exists
	existingJob := &batchv1.Job{}
	err := r.Get(ctx, client.ObjectKey{
		Namespace: aiJob.Namespace,
		Name:      jobName,
	}, existingJob)
	if err == nil {
		// Job already exists
		return nil
	}
	if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check existing hook job: %w", err)
	}

	// Create the hook job
	var backoffLimit int32
	hookJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: aiJob.Namespace,
			Labels: map[string]string{
				"gryvia.io/hook": hook.Name,
				"gryvia.io/job":  aiJob.Name,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:    "hook",
							Image:   k8sJobConfig.Image,
							Command: k8sJobConfig.Command,
						},
					},
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(hook, hookJob, r.Scheme); err != nil {
		return fmt.Errorf("failed to set controller reference: %w", err)
	}

	if err := r.Create(ctx, hookJob); err != nil {
		return fmt.Errorf("failed to create hook job: %w", err)
	}

	return nil
}

func (r *GryviaJobHookReconciler) executeNotification(_ context.Context, hook *gryviav1.GryviaJobHook, job *gryviav1.GryviaAIJob) error {
	if hook.Spec.Action.Notification == nil {
		return fmt.Errorf("notification action configuration is missing")
	}

	notification := hook.Spec.Action.Notification
	r.Log.Info("Sending notification",
		"channels", notification.Channels,
		"recipients", notification.Recipients,
		"job", job.Name,
		"trigger", hook.Spec.Trigger,
	)

	// In a real implementation, this would send to Slack, email, PagerDuty, etc.
	// For now, we log the notification as a placeholder.
	for _, channel := range notification.Channels {
		r.Log.Info("Notification sent",
			"channel", channel,
			"job", job.Name,
			"phase", job.Status.Phase,
		)
	}

	return nil
}

func (r *GryviaJobHookReconciler) executeExec(_ context.Context, hook *gryviav1.GryviaJobHook, job *gryviav1.GryviaAIJob) error {
	if hook.Spec.Action.Exec == nil {
		return fmt.Errorf("exec action configuration is missing")
	}

	r.Log.Info("Executing command hook",
		"command", hook.Spec.Action.Exec.Command,
		"job", job.Name,
		"trigger", hook.Spec.Trigger,
	)

	// In a real implementation, this would exec into the job's pod
	// or run the command in a sidecar container.
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GryviaJobHookReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gryviav1.GryviaJobHook{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}
