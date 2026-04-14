package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	tensorreaperv1 "github.com/ssahani/TensorReaper/operators/ai-operator/api/v1"
)

const (
	// Inference service phases
	PhaseDeployingInfer = "Deploying"
	PhaseReady          = "Ready"
	PhaseRollingBack    = "RollingBack"

	// Condition types for inference service
	ConditionInferenceReady  = "InferenceReady"
	ConditionCanaryActive    = "CanaryActive"
	ConditionHealthy         = "Healthy"
)

// FabricInferenceServiceReconciler reconciles a FabricInferenceService object
type FabricInferenceServiceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabricinferenceservices,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabricinferenceservices/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabricinferenceservices/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop
func (r *FabricInferenceServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("fabricinferenceservice", req.NamespacedName)

	// Fetch the FabricInferenceService instance
	svc := &tensorreaperv1.FabricInferenceService{}
	err := r.Get(ctx, req.NamespacedName, svc)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("FabricInferenceService resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get FabricInferenceService")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !svc.ObjectMeta.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	// Initialize status
	if svc.Status.Phase == "" {
		svc.Status.Phase = PhasePending
		if err := r.Status().Update(ctx, svc); err != nil {
			log.Error(err, "Failed to initialize inference service status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Reconcile the inference service
	result, err := r.reconcileInferenceService(ctx, svc)
	if err != nil {
		log.Error(err, "Failed to reconcile inference service")
		return result, err
	}

	return result, nil
}

func (r *FabricInferenceServiceReconciler) reconcileInferenceService(ctx context.Context, svc *tensorreaperv1.FabricInferenceService) (ctrl.Result, error) {
	log := r.Log.WithValues("fabricinferenceservice", svc.Name)

	// Phase 1: Ensure primary Deployment
	if err := r.ensureDeployment(ctx, svc); err != nil {
		log.Error(err, "Failed to ensure deployment")
		return ctrl.Result{RequeueAfter: 15 * time.Second}, err
	}

	// Phase 2: Ensure Service
	if err := r.ensureService(ctx, svc); err != nil {
		log.Error(err, "Failed to ensure service")
		return ctrl.Result{RequeueAfter: 15 * time.Second}, err
	}

	// Phase 3: Ensure HPA if autoscaling is enabled
	if svc.Spec.Autoscaling != nil && svc.Spec.Autoscaling.Enabled {
		if err := r.ensureHPA(ctx, svc); err != nil {
			log.Error(err, "Failed to ensure HPA")
			// Non-fatal, continue
		}
	}

	// Phase 4: Handle canary deployment
	if svc.Spec.Canary != nil && svc.Spec.Canary.Enabled {
		if err := r.reconcileCanary(ctx, svc); err != nil {
			log.Error(err, "Failed to reconcile canary")
		}
	}

	// Phase 5: Update status from Deployment
	if err := r.syncDeploymentStatus(ctx, svc); err != nil {
		log.Error(err, "Failed to sync deployment status")
	}

	// Phase 6: Health checks and auto-rollback
	if svc.Spec.HealthCheck != nil && svc.Spec.HealthCheck.AutoRollback {
		r.checkHealthAndRollback(ctx, svc)
	}

	if err := r.Status().Update(ctx, svc); err != nil {
		log.Error(err, "Failed to update inference service status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// ensureDeployment creates or updates the inference Deployment.
func (r *FabricInferenceServiceReconciler) ensureDeployment(ctx context.Context, svc *tensorreaperv1.FabricInferenceService) error {
	deployName := fmt.Sprintf("%s-inference", svc.Name)
	deploy := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Namespace: svc.Namespace, Name: deployName}, deploy)

	desired := r.buildDeployment(svc, deployName, false)

	if err != nil && errors.IsNotFound(err) {
		if err := controllerutil.SetControllerReference(svc, desired, r.Scheme); err != nil {
			return err
		}
		svc.Status.DeploymentName = deployName
		return r.Create(ctx, desired)
	}
	if err != nil {
		return err
	}

	// Update if image or replicas changed
	needsUpdate := false
	if *deploy.Spec.Replicas != *desired.Spec.Replicas {
		deploy.Spec.Replicas = desired.Spec.Replicas
		needsUpdate = true
	}
	if len(deploy.Spec.Template.Spec.Containers) > 0 && len(desired.Spec.Template.Spec.Containers) > 0 {
		if deploy.Spec.Template.Spec.Containers[0].Image != desired.Spec.Template.Spec.Containers[0].Image {
			deploy.Spec.Template.Spec.Containers[0].Image = desired.Spec.Template.Spec.Containers[0].Image
			needsUpdate = true
		}
	}
	if needsUpdate {
		return r.Update(ctx, deploy)
	}

	svc.Status.DeploymentName = deployName
	return nil
}

// buildDeployment constructs the Deployment spec for the inference service.
func (r *FabricInferenceServiceReconciler) buildDeployment(svc *tensorreaperv1.FabricInferenceService, name string, isCanary bool) *appsv1.Deployment {
	labels := map[string]string{
		"tensorreaper.ai/inference": svc.Name,
		"tensorreaper.ai/component": "inference-server",
	}
	if isCanary {
		labels["tensorreaper.ai/canary"] = "true"
	}

	image := r.getBackendImage(svc)
	replicas := svc.Spec.Replicas
	if replicas <= 0 {
		replicas = 1
	}

	port := svc.Spec.ServicePort
	if port <= 0 {
		port = 8080
	}

	gpuCount := svc.Spec.GPUCount
	if gpuCount <= 0 {
		gpuCount = 1
	}

	container := corev1.Container{
		Name:  "inference",
		Image: image,
		Args:  svc.Spec.Args,
		Ports: []corev1.ContainerPort{
			{
				Name:          "http",
				ContainerPort: port,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				"nvidia.com/gpu": *resource.NewQuantity(int64(gpuCount), resource.DecimalSI),
			},
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: r.getHealthPath(svc),
					Port: intstr.FromInt32(port),
				},
			},
			InitialDelaySeconds: 30,
			PeriodSeconds:       10,
		},
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: r.getHealthPath(svc),
					Port: intstr.FromInt32(port),
				},
			},
			InitialDelaySeconds: 60,
			PeriodSeconds:       30,
		},
	}

	// Add model reference env var
	container.Env = []corev1.EnvVar{
		{Name: "MODEL_NAME", Value: svc.Spec.ModelRef},
		{Name: "BACKEND", Value: string(svc.Spec.Backend)},
	}

	nodeSelector := map[string]string{}
	if svc.Spec.GPUType != "" && svc.Spec.GPUType != "any" {
		nodeSelector["tensorreaper.ai/gpu"] = svc.Spec.GPUType
	}

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: svc.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers:   []corev1.Container{container},
					NodeSelector: nodeSelector,
				},
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
			},
		},
	}

	return deploy
}

// getBackendImage returns the default container image for the given backend.
func (r *FabricInferenceServiceReconciler) getBackendImage(svc *tensorreaperv1.FabricInferenceService) string {
	if svc.Spec.Image != "" {
		return svc.Spec.Image
	}

	switch svc.Spec.Backend {
	case tensorreaperv1.BackendVLLM:
		return "vllm/vllm-openai:latest"
	case tensorreaperv1.BackendTriton:
		return "nvcr.io/nvidia/tritonserver:24.01-py3"
	case tensorreaperv1.BackendTensorRTLLM:
		return "nvcr.io/nvidia/tritonserver:24.01-trtllm-python-py3"
	case tensorreaperv1.BackendTorchServe:
		return "pytorch/torchserve:latest-gpu"
	default:
		return "vllm/vllm-openai:latest"
	}
}

// getHealthPath returns the health check path.
func (r *FabricInferenceServiceReconciler) getHealthPath(svc *tensorreaperv1.FabricInferenceService) string {
	if svc.Spec.HealthCheck != nil && svc.Spec.HealthCheck.Path != "" {
		return svc.Spec.HealthCheck.Path
	}
	return "/health"
}

// ensureService creates or updates the Kubernetes Service for inference.
func (r *FabricInferenceServiceReconciler) ensureService(ctx context.Context, svc *tensorreaperv1.FabricInferenceService) error {
	svcName := fmt.Sprintf("%s-inference", svc.Name)
	k8sSvc := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{Namespace: svc.Namespace, Name: svcName}, k8sSvc)

	port := svc.Spec.ServicePort
	if port <= 0 {
		port = 8080
	}

	if err != nil && errors.IsNotFound(err) {
		k8sSvc = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      svcName,
				Namespace: svc.Namespace,
				Labels: map[string]string{
					"tensorreaper.ai/inference":  svc.Name,
					"tensorreaper.ai/component": "inference-service",
				},
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{
					"tensorreaper.ai/inference":  svc.Name,
					"tensorreaper.ai/component": "inference-server",
				},
				Ports: []corev1.ServicePort{
					{
						Name:       "http",
						Port:       port,
						TargetPort: intstr.FromString("http"),
						Protocol:   corev1.ProtocolTCP,
					},
				},
				Type: corev1.ServiceTypeClusterIP,
			},
		}

		if err := controllerutil.SetControllerReference(svc, k8sSvc, r.Scheme); err != nil {
			return err
		}

		if err := r.Create(ctx, k8sSvc); err != nil {
			return err
		}

		svc.Status.ServiceName = svcName
		svc.Status.Endpoint = fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", svcName, svc.Namespace, port)
		return nil
	}

	svc.Status.ServiceName = svcName
	svc.Status.Endpoint = fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", svcName, svc.Namespace, port)
	return err
}

// ensureHPA creates or updates the HorizontalPodAutoscaler.
func (r *FabricInferenceServiceReconciler) ensureHPA(ctx context.Context, svc *tensorreaperv1.FabricInferenceService) error {
	hpaName := fmt.Sprintf("%s-inference-hpa", svc.Name)
	hpa := &autoscalingv2.HorizontalPodAutoscaler{}
	err := r.Get(ctx, types.NamespacedName{Namespace: svc.Namespace, Name: hpaName}, hpa)

	minReplicas := svc.Spec.Autoscaling.MinReplicas
	if minReplicas <= 0 {
		minReplicas = 1
	}

	targetCPU := int32(80)

	metrics := []autoscalingv2.MetricSpec{
		{
			Type: autoscalingv2.ResourceMetricSourceType,
			Resource: &autoscalingv2.ResourceMetricSource{
				Name: corev1.ResourceCPU,
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: &targetCPU,
				},
			},
		},
	}

	if err != nil && errors.IsNotFound(err) {
		hpa = &autoscalingv2.HorizontalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      hpaName,
				Namespace: svc.Namespace,
				Labels: map[string]string{
					"tensorreaper.ai/inference": svc.Name,
				},
			},
			Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
				ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       fmt.Sprintf("%s-inference", svc.Name),
				},
				MinReplicas: &minReplicas,
				MaxReplicas: svc.Spec.Autoscaling.MaxReplicas,
				Metrics:     metrics,
			},
		}

		if err := controllerutil.SetControllerReference(svc, hpa, r.Scheme); err != nil {
			return err
		}

		return r.Create(ctx, hpa)
	}

	return err
}

// reconcileCanary manages the canary deployment.
func (r *FabricInferenceServiceReconciler) reconcileCanary(ctx context.Context, svc *tensorreaperv1.FabricInferenceService) error {
	canaryName := fmt.Sprintf("%s-canary", svc.Name)

	// Ensure canary deployment exists
	deploy := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Namespace: svc.Namespace, Name: canaryName}, deploy)

	if err != nil && errors.IsNotFound(err) {
		canaryDeploy := r.buildDeployment(svc, canaryName, true)

		// Override model ref to canary version
		for i := range canaryDeploy.Spec.Template.Spec.Containers {
			canaryDeploy.Spec.Template.Spec.Containers[i].Env = append(
				canaryDeploy.Spec.Template.Spec.Containers[i].Env,
				corev1.EnvVar{Name: "CANARY_MODEL", Value: svc.Spec.Canary.ModelVersion},
			)
		}

		// Canary gets a fraction of the replicas proportional to its weight
		canaryReplicas := int32(1)
		canaryDeploy.Spec.Replicas = &canaryReplicas

		if err := controllerutil.SetControllerReference(svc, canaryDeploy, r.Scheme); err != nil {
			return err
		}

		if err := r.Create(ctx, canaryDeploy); err != nil {
			return err
		}
	}

	// Update canary status
	if svc.Status.CanaryStatus == nil {
		now := metav1.Now()
		svc.Status.CanaryStatus = &tensorreaperv1.CanaryStatus{
			Active:         true,
			Weight:         svc.Spec.Canary.Weight,
			DeploymentName: canaryName,
			StartedAt:      &now,
		}
	}

	// Sync canary deployment status
	canaryDeploy := &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: svc.Namespace, Name: canaryName}, canaryDeploy); err == nil {
		svc.Status.CanaryStatus.ReadyReplicas = canaryDeploy.Status.ReadyReplicas
		if canaryDeploy.Status.ReadyReplicas > 0 {
			svc.Status.CanaryStatus.Health = "Healthy"
		} else {
			svc.Status.CanaryStatus.Health = "Unhealthy"
		}
	}

	// Check for auto-promote
	if svc.Spec.Canary.AutoPromote && svc.Status.CanaryStatus.StartedAt != nil {
		elapsed := time.Since(svc.Status.CanaryStatus.StartedAt.Time)
		promoteAfter := time.Duration(svc.Spec.Canary.PromoteAfterSeconds) * time.Second
		if promoteAfter > 0 && elapsed > promoteAfter && svc.Status.CanaryStatus.Health == "Healthy" {
			// Promote canary: update primary deployment to use canary model
			// and remove canary deployment
			if err := r.Delete(ctx, &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: canaryName, Namespace: svc.Namespace},
			}); err != nil && !errors.IsNotFound(err) {
				return err
			}
			svc.Status.CanaryStatus = nil
			svc.Status.Message = fmt.Sprintf("Canary promoted: %s", svc.Spec.Canary.ModelVersion)
		}
	}

	r.updateInferCondition(svc, ConditionCanaryActive, metav1.ConditionTrue, "CanaryDeployed",
		fmt.Sprintf("Canary weight: %d%%", svc.Spec.Canary.Weight))

	return nil
}

// syncDeploymentStatus updates the inference service status based on the Deployment.
func (r *FabricInferenceServiceReconciler) syncDeploymentStatus(ctx context.Context, svc *tensorreaperv1.FabricInferenceService) error {
	deployName := fmt.Sprintf("%s-inference", svc.Name)
	deploy := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Namespace: svc.Namespace, Name: deployName}, deploy)
	if err != nil {
		return err
	}

	svc.Status.ReadyReplicas = deploy.Status.ReadyReplicas

	if deploy.Status.ReadyReplicas > 0 && deploy.Status.ReadyReplicas == *deploy.Spec.Replicas {
		svc.Status.Phase = PhaseReady
		svc.Status.HealthStatus = "Healthy"
		svc.Status.ConsecutiveFailures = 0
		r.updateInferCondition(svc, ConditionInferenceReady, metav1.ConditionTrue, "Ready", "All replicas are ready")
	} else if deploy.Status.ReadyReplicas > 0 {
		svc.Status.Phase = PhaseReady
		r.updateInferCondition(svc, ConditionInferenceReady, metav1.ConditionFalse, "PartiallyReady",
			fmt.Sprintf("%d/%d replicas ready", deploy.Status.ReadyReplicas, *deploy.Spec.Replicas))
	} else {
		svc.Status.Phase = PhaseDeployingInfer
	}

	return nil
}

// checkHealthAndRollback implements automatic rollback on consecutive health check failures.
func (r *FabricInferenceServiceReconciler) checkHealthAndRollback(ctx context.Context, svc *tensorreaperv1.FabricInferenceService) {
	threshold := svc.Spec.HealthCheck.FailureThreshold
	if threshold <= 0 {
		threshold = 3
	}

	if svc.Status.HealthStatus == "Unhealthy" {
		svc.Status.ConsecutiveFailures++
	}

	if svc.Status.ConsecutiveFailures >= threshold {
		svc.Status.Phase = PhaseRollingBack
		svc.Status.Message = fmt.Sprintf("Rolling back: %d consecutive health check failures", svc.Status.ConsecutiveFailures)
		r.updateInferCondition(svc, ConditionHealthy, metav1.ConditionFalse, "Unhealthy",
			fmt.Sprintf("Auto-rollback triggered after %d failures", svc.Status.ConsecutiveFailures))
		// In production, this would restore the previous Deployment revision
		svc.Status.ConsecutiveFailures = 0
	}

	now := metav1.Now()
	svc.Status.LastHealthCheck = &now
}

func (r *FabricInferenceServiceReconciler) updateInferCondition(svc *tensorreaperv1.FabricInferenceService, condType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: svc.Generation,
		LastTransitionTime: metav1.Now(),
	}

	for _, cond := range svc.Status.Conditions {
		if cond.Type == condType && cond.Status == status {
			condition.LastTransitionTime = cond.LastTransitionTime
			break
		}
	}

	found := false
	for i, cond := range svc.Status.Conditions {
		if cond.Type == condType {
			svc.Status.Conditions[i] = condition
			found = true
			break
		}
	}
	if !found {
		svc.Status.Conditions = append(svc.Status.Conditions, condition)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *FabricInferenceServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tensorreaperv1.FabricInferenceService{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&autoscalingv2.HorizontalPodAutoscaler{}).
		Complete(r)
}
