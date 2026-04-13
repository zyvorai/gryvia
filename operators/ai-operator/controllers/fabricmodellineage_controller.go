package controllers

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	tensorreaperv1 "github.com/ssahani/tensor-reaper/operators/ai-operator/api/v1"
	"github.com/ssahani/tensor-reaper/operators/ai-operator/pkg/lineage"
)

// FabricModelLineageReconciler reconciles a FabricModelLineage object
type FabricModelLineageReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabricmodellineages,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabricmodellineages/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabricmodellineages/finalizers,verbs=update
//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabricaijobs,verbs=get;list;watch

func (r *FabricModelLineageReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the FabricModelLineage instance
	ml := &tensorreaperv1.FabricModelLineage{}
	err := r.Get(ctx, req.NamespacedName, ml)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("FabricModelLineage resource not found, ignoring")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get FabricModelLineage")
		return ctrl.Result{}, err
	}

	logger.Info("Reconciling FabricModelLineage",
		"name", ml.Name,
		"model", ml.Spec.Model.Name,
		"version", ml.Spec.Model.Version,
	)

	// Set creation timestamp on first reconcile
	if ml.Status.CreatedAt.IsZero() {
		ml.Status.CreatedAt = metav1.Now()
	}

	// Auto-collect provenance from referenced resources
	if ml.Spec.Provenance.AutoCapture {
		if err := r.autoCollectProvenance(ctx, ml); err != nil {
			logger.Error(err, "Failed to auto-collect provenance")
			r.setCondition(ml, "ProvenanceCollected", metav1.ConditionFalse,
				"CollectionFailed", err.Error())
		} else {
			r.setCondition(ml, "ProvenanceCollected", metav1.ConditionTrue,
				"Collected", "Provenance data collected successfully")
		}
	}

	// Check lineage completeness
	ml.Status.LineageComplete = lineage.CheckLineageCompleteness(ml)

	// Check reproducibility
	ml.Status.ReproducibilityVerified = lineage.CheckReproducibility(ml)

	// Compute provenance hash
	previousHash := ml.Status.ProvenanceHash
	hash, err := lineage.ComputeProvenanceHash(ml, previousHash)
	if err != nil {
		logger.Error(err, "Failed to compute provenance hash")
		r.setCondition(ml, "HashComputed", metav1.ConditionFalse,
			"HashFailed", err.Error())
	} else {
		ml.Status.ProvenanceHash = hash
		r.setCondition(ml, "HashComputed", metav1.ConditionTrue,
			"Computed", "Provenance hash computed")
	}

	// Evaluate compliance
	ml.Status.ComplianceStatus = lineage.EvaluateCompliance(ml)

	// Set attestation status
	if ml.Spec.Compliance.Attestation.AttachToRegistry {
		// In a production system, this would actually attach to the registry.
		// For now, mark it as attached if hash is computed and compliance passes.
		ml.Status.AttestationAttached = ml.Status.ProvenanceHash != "" &&
			ml.Status.ComplianceStatus == "Compliant"
	}

	// Set overall readiness
	if ml.Status.LineageComplete && ml.Status.ProvenanceHash != "" {
		r.setCondition(ml, "Ready", metav1.ConditionTrue,
			"LineageComplete", "Model lineage is complete and verified")
	} else {
		r.setCondition(ml, "Ready", metav1.ConditionFalse,
			"LineageIncomplete", "Model lineage is not yet complete")
	}

	// Update status
	if err := r.Status().Update(ctx, ml); err != nil {
		logger.Error(err, "Failed to update FabricModelLineage status")
		return ctrl.Result{}, err
	}

	// If lineage is not yet complete and auto-capture is on, requeue to try again
	if !ml.Status.LineageComplete && ml.Spec.Provenance.AutoCapture {
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

// autoCollectProvenance uses the lineage collector to gather provenance data
// from referenced FabricAIJob and related resources.
func (r *FabricModelLineageReconciler) autoCollectProvenance(ctx context.Context, ml *tensorreaperv1.FabricModelLineage) error {
	logger := log.FromContext(ctx)
	collector := lineage.NewCollector(r.Client)

	collected, err := collector.CollectProvenance(ctx, ml)
	if err != nil {
		return err
	}

	if collected == nil {
		logger.Info("No provenance data collected")
		return nil
	}

	if !collected.JobFound {
		logger.Info("Referenced job not found, skipping provenance auto-collection",
			"jobRef", ml.Spec.Provenance.Training.JobRef)
		return nil
	}

	// Only update provenance for jobs that have completed or are running
	if collected.JobPhase != "Succeeded" && collected.JobPhase != "Running" && collected.JobPhase != "Failed" {
		logger.Info("Referenced job not yet in a collectible state",
			"jobRef", ml.Spec.Provenance.Training.JobRef,
			"phase", collected.JobPhase)
		return nil
	}

	// Merge collected infrastructure data
	if len(collected.Infrastructure.GPUNodes) > 0 {
		ml.Spec.Provenance.Infrastructure.GPUNodes = collected.Infrastructure.GPUNodes
	}
	if collected.Infrastructure.NetworkType != "" {
		ml.Spec.Provenance.Infrastructure.NetworkType = collected.Infrastructure.NetworkType
	}
	if collected.Infrastructure.StorageBackend != "" {
		ml.Spec.Provenance.Infrastructure.StorageBackend = collected.Infrastructure.StorageBackend
	}
	if collected.Infrastructure.TotalGpuHours > 0 {
		ml.Spec.Provenance.Infrastructure.TotalGpuHours = collected.Infrastructure.TotalGpuHours
	}

	// Merge training data
	if collected.Training.DistributedConfig != "" {
		ml.Spec.Provenance.Training.DistributedConfig = collected.Training.DistributedConfig
	}

	// Merge code provenance (auto-fill container image)
	if collected.Code.ContainerImage != "" && ml.Spec.Provenance.Code.ContainerImage == "" {
		ml.Spec.Provenance.Code.ContainerImage = collected.Code.ContainerImage
	}

	// Merge events
	if len(collected.Events.Anomalies) > 0 {
		ml.Spec.Provenance.Events.Anomalies = collected.Events.Anomalies
	}
	if len(collected.Events.Interventions) > 0 {
		ml.Spec.Provenance.Events.Interventions = collected.Events.Interventions
	}

	// Update the spec with collected data
	if err := r.Update(ctx, ml); err != nil {
		return err
	}

	logger.Info("Auto-collected provenance from job",
		"jobRef", ml.Spec.Provenance.Training.JobRef,
		"gpuNodes", len(ml.Spec.Provenance.Infrastructure.GPUNodes),
		"gpuHours", ml.Spec.Provenance.Infrastructure.TotalGpuHours,
	)

	return nil
}

func (r *FabricModelLineageReconciler) setCondition(ml *tensorreaperv1.FabricModelLineage, condType string, status metav1.ConditionStatus, reason, message string) {
	meta.SetStatusCondition(&ml.Status.Conditions, metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	})
}

// SetupWithManager sets up the controller with the Manager
func (r *FabricModelLineageReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tensorreaperv1.FabricModelLineage{}).
		Watches(&tensorreaperv1.FabricAIJob{}, handler.EnqueueRequestsFromMapFunc(
			func(ctx context.Context, obj client.Object) []reconcile.Request {
				// When a FabricAIJob completes, re-reconcile lineages that
				// reference it so provenance can be auto-collected.
				job, ok := obj.(*tensorreaperv1.FabricAIJob)
				if !ok {
					return nil
				}

				// Only trigger for terminal states
				if job.Status.Phase != "Succeeded" && job.Status.Phase != "Failed" {
					return nil
				}

				// Find lineages that reference this job
				lineageList := &tensorreaperv1.FabricModelLineageList{}
				if err := mgr.GetClient().List(ctx, lineageList,
					client.InNamespace(job.Namespace)); err != nil {
					return nil
				}

				var requests []reconcile.Request
				for _, ml := range lineageList.Items {
					if ml.Spec.Provenance.Training.JobRef == job.Name {
						requests = append(requests, reconcile.Request{
							NamespacedName: client.ObjectKeyFromObject(&ml),
						})
					}
				}
				return requests
			},
		)).
		Complete(r)
}
