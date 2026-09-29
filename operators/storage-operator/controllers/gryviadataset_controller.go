package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	gryviav1 "github.com/zyvorai/gryvia/operators/storage-operator/api/v1"
)

const (
	datasetFinalizer = "gryvia.io/dataset-finalizer"

	datasetStateInitializing = "initializing"
	datasetStateReady        = "ready"
	datasetStateError        = "error"
	datasetStateSyncing      = "syncing"

	// Namespace for dataset PVCs
	datasetPVCNamespace = "gryvia-datasets"

	// Cache garbage collection interval
	cacheGCInterval = 24 * time.Hour

	// Default cache TTL when not accessed
	defaultCacheIdleTTL = 7 * 24 * time.Hour
)

// FabricDatasetReconciler reconciles a FabricDataset object
type FabricDatasetReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

//+kubebuilder:rbac:groups=gryvia.io,resources=fabricdatasets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gryvia.io,resources=fabricdatasets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gryvia.io,resources=fabricdatasets/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=persistentvolumes,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop
func (r *FabricDatasetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("fabricdataset", req.NamespacedName)

	// Fetch the FabricDataset instance
	dataset := &gryviav1.FabricDataset{}
	err := r.Get(ctx, req.NamespacedName, dataset)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("FabricDataset resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get FabricDataset")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !dataset.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, dataset)
	}

	// Add finalizer if it doesn't exist
	if !controllerutil.ContainsFinalizer(dataset, datasetFinalizer) {
		controllerutil.AddFinalizer(dataset, datasetFinalizer)
		if err := r.Update(ctx, dataset); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Initialize status if needed
	if dataset.Status.State == "" {
		dataset.Status.State = datasetStateInitializing
		if err := r.Status().Update(ctx, dataset); err != nil {
			log.Error(err, "Failed to update initial status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Reconcile the dataset
	result, err := r.reconcileDataset(ctx, dataset)
	if err != nil {
		log.Error(err, "Failed to reconcile dataset")
		return result, err
	}

	return result, nil
}

func (r *FabricDatasetReconciler) reconcileDataset(ctx context.Context, dataset *gryviav1.FabricDataset) (ctrl.Result, error) {
	log := r.Log.WithValues("fabricdataset", dataset.Name)

	// Step 1: Provision cache PVC if caching is enabled
	if dataset.Spec.Cache != nil && dataset.Spec.Cache.Enabled {
		if err := r.ensureCachePVC(ctx, dataset); err != nil {
			log.Error(err, "Failed to ensure cache PVC")
			dataset.Status.State = datasetStateError
			if updateErr := r.Status().Update(ctx, dataset); updateErr != nil {
				log.Error(updateErr, "Failed to update status")
			}
			return ctrl.Result{RequeueAfter: 30 * time.Second}, err
		}
	}

	// Step 2: Update version info
	r.updateVersionInfo(dataset)

	// Step 3: Update cache status
	if dataset.Spec.Cache != nil && dataset.Spec.Cache.Enabled {
		r.updateCacheStatus(ctx, dataset)
	}

	// Step 4: Garbage collect unused cached datasets
	if err := r.garbageCollectCache(ctx, dataset); err != nil {
		log.Error(err, "Failed to garbage collect cache")
	}

	// Step 5: Set state to ready
	if dataset.Status.State == datasetStateInitializing || dataset.Status.State == datasetStateSyncing {
		dataset.Status.State = datasetStateReady
	}

	// Update status
	if err := r.Status().Update(ctx, dataset); err != nil {
		log.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

func (r *FabricDatasetReconciler) ensureCachePVC(ctx context.Context, dataset *gryviav1.FabricDataset) error {
	pvcName := r.getCachePVCName(dataset)

	pvc := &corev1.PersistentVolumeClaim{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: datasetPVCNamespace,
		Name:      pvcName,
	}, pvc)

	if err != nil && errors.IsNotFound(err) {
		// Parse cache size
		cacheSize := "100Gi"
		if dataset.Spec.Cache.Size != "" {
			cacheSize = dataset.Spec.Cache.Size
		}

		storageQuantity, parseErr := resource.ParseQuantity(cacheSize)
		if parseErr != nil {
			return fmt.Errorf("invalid cache size %q: %w", cacheSize, parseErr)
		}

		storageClass := "vast-data"
		if dataset.Spec.Cache.StorageClass != "" {
			storageClass = dataset.Spec.Cache.StorageClass
		}

		// Create cache PVC
		pvc = &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvcName,
				Namespace: datasetPVCNamespace,
				Labels: map[string]string{
					"gryvia.io/dataset":    dataset.Name,
					"gryvia.io/cache-type": "dataset",
				},
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteMany,
				},
				StorageClassName: &storageClass,
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: storageQuantity,
					},
				},
			},
		}

		if err := controllerutil.SetControllerReference(dataset, pvc, r.Scheme); err != nil {
			return fmt.Errorf("failed to set controller reference: %w", err)
		}

		if err := r.Create(ctx, pvc); err != nil {
			return fmt.Errorf("failed to create cache PVC: %w", err)
		}

		r.Log.Info("Created cache PVC", "name", pvcName, "size", cacheSize)
		return nil
	}

	return err
}

func (r *FabricDatasetReconciler) updateVersionInfo(dataset *gryviav1.FabricDataset) {
	// Set current version
	if dataset.Spec.Version != "" {
		dataset.Status.CurrentVersion = dataset.Spec.Version
	}

	// Check if this version is already in the list
	versionExists := false
	for _, v := range dataset.Status.Versions {
		if v.Version == dataset.Spec.Version {
			versionExists = true
			break
		}
	}

	// Add new version if not present
	if !versionExists && dataset.Spec.Version != "" {
		now := metav1.Now()
		versionInfo := gryviav1.DatasetVersionInfo{
			Version:   dataset.Spec.Version,
			CreatedAt: &now,
		}

		if dataset.Spec.Statistics != nil {
			versionInfo.Size = dataset.Spec.Statistics.TotalSizeBytes
		}

		dataset.Status.Versions = append(dataset.Status.Versions, versionInfo)
	}

	// Enforce retention policy
	if dataset.Spec.Versioning != nil && dataset.Spec.Versioning.RetentionPolicy != nil {
		r.enforceRetentionPolicy(dataset)
	}
}

func (r *FabricDatasetReconciler) enforceRetentionPolicy(dataset *gryviav1.FabricDataset) {
	if dataset.Spec.Versioning == nil || dataset.Spec.Versioning.RetentionPolicy == nil {
		return
	}

	retention := dataset.Spec.Versioning.RetentionPolicy

	// Keep last N versions
	if retention.KeepLast > 0 && int32(len(dataset.Status.Versions)) > retention.KeepLast {
		dataset.Status.Versions = dataset.Status.Versions[len(dataset.Status.Versions)-int(retention.KeepLast):]
	}

	// Remove versions older than KeepDays
	if retention.KeepDays > 0 {
		cutoff := time.Now().AddDate(0, 0, -int(retention.KeepDays))
		filtered := make([]gryviav1.DatasetVersionInfo, 0)
		for _, v := range dataset.Status.Versions {
			if v.CreatedAt != nil && v.CreatedAt.Time.After(cutoff) {
				filtered = append(filtered, v)
			}
		}
		// Always keep at least the current version
		if len(filtered) == 0 && len(dataset.Status.Versions) > 0 {
			filtered = append(filtered, dataset.Status.Versions[len(dataset.Status.Versions)-1])
		}
		dataset.Status.Versions = filtered
	}
}

func (r *FabricDatasetReconciler) updateCacheStatus(ctx context.Context, dataset *gryviav1.FabricDataset) {
	pvcName := r.getCachePVCName(dataset)

	pvc := &corev1.PersistentVolumeClaim{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: datasetPVCNamespace,
		Name:      pvcName,
	}, pvc)

	if err == nil {
		if dataset.Status.CacheStatus == nil {
			dataset.Status.CacheStatus = &gryviav1.DatasetCacheStatus{}
		}
		dataset.Status.CacheStatus.Cached = pvc.Status.Phase == corev1.ClaimBound
	} else {
		if dataset.Status.CacheStatus != nil {
			dataset.Status.CacheStatus.Cached = false
		}
	}
}

func (r *FabricDatasetReconciler) garbageCollectCache(ctx context.Context, dataset *gryviav1.FabricDataset) error {
	if dataset.Spec.Cache == nil || !dataset.Spec.Cache.Enabled {
		return nil
	}

	// Check if dataset cache has been idle for too long
	if dataset.Status.CacheStatus != nil && dataset.Status.CacheStatus.LastAccess != nil {
		idleDuration := time.Since(dataset.Status.CacheStatus.LastAccess.Time)

		// Check if usage is zero and idle beyond TTL
		if dataset.Status.Usage != nil && dataset.Status.Usage.JobsUsing == 0 && idleDuration > defaultCacheIdleTTL {
			r.Log.Info("Dataset cache idle beyond TTL, eligible for garbage collection",
				"dataset", dataset.Name,
				"idle", idleDuration,
				"ttl", defaultCacheIdleTTL,
			)
			// In a real implementation, this would delete the cache PVC contents
			// but keep the PVC for future use. Full PVC deletion would need to be
			// explicitly requested.
		}
	}

	return nil
}

func (r *FabricDatasetReconciler) handleDeletion(ctx context.Context, dataset *gryviav1.FabricDataset) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(dataset, datasetFinalizer) {
		r.Log.Info("Running cleanup for FabricDataset", "name", dataset.Name)

		// Delete cache PVC
		if dataset.Spec.Cache != nil && dataset.Spec.Cache.Enabled {
			pvcName := r.getCachePVCName(dataset)
			pvc := &corev1.PersistentVolumeClaim{}
			err := r.Get(ctx, types.NamespacedName{
				Namespace: datasetPVCNamespace,
				Name:      pvcName,
			}, pvc)
			if err == nil {
				if delErr := r.Delete(ctx, pvc); delErr != nil {
					r.Log.Error(delErr, "Failed to delete cache PVC during cleanup", "pvc", pvcName)
					return ctrl.Result{}, fmt.Errorf("failed to delete cache PVC %s: %w", pvcName, delErr)
				}
				r.Log.Info("Deleted cache PVC", "pvc", pvcName)
			} else if !errors.IsNotFound(err) {
				return ctrl.Result{}, fmt.Errorf("failed to get cache PVC %s during cleanup: %w", pvcName, err)
			}
		}

		// Remove finalizer
		controllerutil.RemoveFinalizer(dataset, datasetFinalizer)
		if err := r.Update(ctx, dataset); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *FabricDatasetReconciler) getCachePVCName(dataset *gryviav1.FabricDataset) string {
	return fmt.Sprintf("dataset-cache-%s", dataset.Name)
}

// SetupWithManager sets up the controller with the Manager.
func (r *FabricDatasetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gryviav1.FabricDataset{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Complete(r)
}
