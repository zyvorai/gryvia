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
	// Model registry phases
	PhaseRegistered = "Registered"
	PhaseDeploying  = "Deploying"
	PhaseServing    = "Serving"

	// Condition types for model registry
	ConditionModelRegistered = "ModelRegistered"
	ConditionModelServing    = "ModelServing"
)

// FabricModelRegistryReconciler reconciles a FabricModelRegistry object
type FabricModelRegistryReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

//+kubebuilder:rbac:groups=gryvia.io,resources=fabricmodelregistries,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gryvia.io,resources=fabricmodelregistries/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gryvia.io,resources=fabricmodelregistries/finalizers,verbs=update
//+kubebuilder:rbac:groups=gryvia.io,resources=fabricinferenceservices,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop
func (r *FabricModelRegistryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("fabricmodelregistry", req.NamespacedName)

	// Fetch the FabricModelRegistry instance
	model := &gryviav1.FabricModelRegistry{}
	err := r.Get(ctx, req.NamespacedName, model)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("FabricModelRegistry resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get FabricModelRegistry")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !model.ObjectMeta.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	// Initialize status if needed
	if model.Status.Phase == "" {
		model.Status.Phase = PhaseRegistered
		now := metav1.Now()
		model.Status.RegisteredAt = &now
		r.updateModelCondition(model, ConditionModelRegistered, metav1.ConditionTrue, "Registered",
			fmt.Sprintf("Model %s version %s registered", model.Spec.ModelName, model.Spec.Version))
		if err := r.Status().Update(ctx, model); err != nil {
			log.Error(err, "Failed to initialize model registry status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Reconcile the model registry
	result, err := r.reconcileModelRegistry(ctx, model)
	if err != nil {
		log.Error(err, "Failed to reconcile model registry")
		return result, err
	}

	return result, nil
}

func (r *FabricModelRegistryReconciler) reconcileModelRegistry(ctx context.Context, model *gryviav1.FabricModelRegistry) (ctrl.Result, error) {
	log := r.Log.WithValues("fabricmodelregistry", model.Name)

	// Check if model should be auto-served when it reaches production stage
	if model.Spec.Stage == gryviav1.ModelStageProduction && model.Spec.AutoServe {
		if model.Status.Phase == PhaseRegistered || model.Status.Phase == PhaseDeploying {
			if err := r.ensureInferenceService(ctx, model); err != nil {
				log.Error(err, "Failed to ensure inference service")
				model.Status.Phase = PhaseFailed
				model.Status.Message = fmt.Sprintf("Failed to deploy: %v", err)
				if updateErr := r.Status().Update(ctx, model); updateErr != nil {
					log.Error(updateErr, "Failed to update model status after deployment failure")
				}
				return ctrl.Result{RequeueAfter: 30 * time.Second}, err
			}
		}

		// Check inference service health
		if model.Status.InferenceServiceName != "" {
			r.syncInferenceHealth(ctx, model)
		}
	}

	// Track previous version for rollback support
	if model.Spec.Stage == gryviav1.ModelStageProduction {
		r.trackPreviousVersion(ctx, model)
	}

	if err := r.Status().Update(ctx, model); err != nil {
		log.Error(err, "Failed to update model registry status")
		return ctrl.Result{}, err
	}

	// Requeue periodically if serving to monitor health
	if model.Status.Phase == PhaseServing {
		return ctrl.Result{RequeueAfter: 60 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

// ensureInferenceService creates a FabricInferenceService for a production model.
func (r *FabricModelRegistryReconciler) ensureInferenceService(ctx context.Context, model *gryviav1.FabricModelRegistry) error {
	inferName := fmt.Sprintf("%s-%s-serving", model.Spec.ModelName, model.Spec.Version)

	// Check if it already exists
	existing := &gryviav1.FabricInferenceService{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: model.Namespace,
		Name:      inferName,
	}, existing)
	if err == nil {
		// Already exists
		model.Status.Phase = PhaseServing
		model.Status.InferenceServiceName = inferName
		return nil
	}
	if !errors.IsNotFound(err) {
		return err
	}

	// Determine serving configuration
	backend := gryviav1.BackendVLLM
	replicas := int32(1)
	gpuCount := int32(1)
	gpuType := ""

	if model.Spec.ServingConfig != nil {
		if model.Spec.ServingConfig.Backend != "" {
			backend = gryviav1.InferenceBackend(model.Spec.ServingConfig.Backend)
		}
		if model.Spec.ServingConfig.Replicas > 0 {
			replicas = model.Spec.ServingConfig.Replicas
		}
		if model.Spec.ServingConfig.GPUCount > 0 {
			gpuCount = model.Spec.ServingConfig.GPUCount
		}
		gpuType = model.Spec.ServingConfig.GPUType
	}

	inferSvc := &gryviav1.FabricInferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      inferName,
			Namespace: model.Namespace,
			Labels: map[string]string{
				"gryvia.io/model":     model.Spec.ModelName,
				"gryvia.io/version":   model.Spec.Version,
				"gryvia.io/component": "inference",
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(model, gryviav1.GroupVersion.WithKind("FabricModelRegistry")),
			},
		},
		Spec: gryviav1.FabricInferenceServiceSpec{
			ModelRef: model.Name,
			Backend:  backend,
			Replicas: replicas,
			GPUCount: gpuCount,
			GPUType:  gpuType,
		},
	}

	if err := r.Create(ctx, inferSvc); err != nil {
		return fmt.Errorf("failed to create inference service: %w", err)
	}

	model.Status.Phase = PhaseDeploying
	model.Status.InferenceServiceName = inferName
	model.Status.Message = "Deploying inference service"
	r.updateModelCondition(model, ConditionModelServing, metav1.ConditionFalse, "Deploying", "Inference service being deployed")

	return nil
}

// syncInferenceHealth checks the health of the inference service and updates model status.
func (r *FabricModelRegistryReconciler) syncInferenceHealth(ctx context.Context, model *gryviav1.FabricModelRegistry) {
	inferSvc := &gryviav1.FabricInferenceService{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: model.Namespace,
		Name:      model.Status.InferenceServiceName,
	}, inferSvc)
	if err != nil {
		if errors.IsNotFound(err) {
			model.Status.Health = "Unknown"
			model.Status.Phase = PhaseRegistered
			model.Status.InferenceServiceName = ""
			model.Status.Message = "Inference service not found"
		}
		return
	}

	switch inferSvc.Status.Phase {
	case "Ready":
		model.Status.Phase = PhaseServing
		model.Status.Health = "Healthy"
		model.Status.ServingEndpoint = inferSvc.Status.Endpoint
		now := metav1.Now()
		model.Status.DeployedAt = &now
		r.updateModelCondition(model, ConditionModelServing, metav1.ConditionTrue, "Serving", "Model is being served")
	case "Failed":
		model.Status.Health = "Unhealthy"
		model.Status.Message = inferSvc.Status.Message
		r.updateModelCondition(model, ConditionModelServing, metav1.ConditionFalse, "Failed", inferSvc.Status.Message)
	default:
		model.Status.Phase = PhaseDeploying
		model.Status.Health = "Unknown"
	}
}

// trackPreviousVersion records the currently serving version for rollback support.
func (r *FabricModelRegistryReconciler) trackPreviousVersion(ctx context.Context, model *gryviav1.FabricModelRegistry) {
	if model.Status.PreviousVersion != "" {
		return // Already tracked
	}

	// Find any other model with the same modelName in production stage
	models := &gryviav1.FabricModelRegistryList{}
	if err := r.List(ctx, models,
		client.InNamespace(model.Namespace),
		client.MatchingLabels{"gryvia.io/model": model.Spec.ModelName},
	); err != nil {
		return
	}

	for _, m := range models.Items {
		if m.Name == model.Name {
			continue
		}
		if m.Spec.Stage == gryviav1.ModelStageProduction && m.Spec.ModelName == model.Spec.ModelName {
			model.Status.PreviousVersion = m.Spec.Version
			break
		}
	}
}

func (r *FabricModelRegistryReconciler) updateModelCondition(model *gryviav1.FabricModelRegistry, condType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: model.Generation,
		LastTransitionTime: metav1.Now(),
	}

	for _, cond := range model.Status.Conditions {
		if cond.Type == condType && cond.Status == status {
			condition.LastTransitionTime = cond.LastTransitionTime
			break
		}
	}

	found := false
	for i, cond := range model.Status.Conditions {
		if cond.Type == condType {
			model.Status.Conditions[i] = condition
			found = true
			break
		}
	}
	if !found {
		model.Status.Conditions = append(model.Status.Conditions, condition)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *FabricModelRegistryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gryviav1.FabricModelRegistry{}).
		Owns(&gryviav1.FabricInferenceService{}).
		Complete(r)
}
