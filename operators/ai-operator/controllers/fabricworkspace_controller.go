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
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	gryviav1 "github.com/zyvorai/gryvia/operators/ai-operator/api/v1"
)

const (
	// Workspace condition types
	ConditionWorkspaceReady = "WorkspaceReady"
	ConditionWorkspaceIdle  = "WorkspaceIdle"

	// Default images
	defaultJupyterImage = "jupyter/gpu-notebook:latest"
	defaultVSCodeImage  = "codercom/code-server:latest"
)

// FabricWorkspaceReconciler reconciles a FabricWorkspace object
type FabricWorkspaceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

//+kubebuilder:rbac:groups=gryvia.io,resources=fabricworkspaces,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gryvia.io,resources=fabricworkspaces/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gryvia.io,resources=fabricworkspaces/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop
func (r *FabricWorkspaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("fabricworkspace", req.NamespacedName)

	// Fetch the FabricWorkspace instance
	ws := &gryviav1.FabricWorkspace{}
	err := r.Get(ctx, req.NamespacedName, ws)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("FabricWorkspace resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get FabricWorkspace")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !ws.ObjectMeta.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	// Initialize status
	if ws.Status.Phase == "" {
		ws.Status.Phase = string(gryviav1.WorkspacePhasePending)
		if err := r.Status().Update(ctx, ws); err != nil {
			log.Error(err, "Failed to initialize workspace status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// If paused, scale down
	if ws.Spec.Paused {
		return r.reconcilePaused(ctx, ws)
	}

	// Reconcile the workspace
	result, err := r.reconcileWorkspace(ctx, ws)
	if err != nil {
		log.Error(err, "Failed to reconcile workspace")
		return result, err
	}

	return result, nil
}

func (r *FabricWorkspaceReconciler) reconcileWorkspace(ctx context.Context, ws *gryviav1.FabricWorkspace) (ctrl.Result, error) {
	log := r.Log.WithValues("fabricworkspace", ws.Name)

	// Phase 1: Ensure PVC for persistent workspace data
	if ws.Spec.Storage != "" {
		if err := r.ensureWorkspacePVC(ctx, ws); err != nil {
			log.Error(err, "Failed to ensure workspace PVC")
			return ctrl.Result{RequeueAfter: 15 * time.Second}, err
		}
	}

	// Phase 2: Ensure workspace pod
	if err := r.ensureWorkspacePod(ctx, ws); err != nil {
		log.Error(err, "Failed to ensure workspace pod")
		return ctrl.Result{RequeueAfter: 15 * time.Second}, err
	}

	// Phase 3: Ensure service for workspace access
	if err := r.ensureWorkspaceService(ctx, ws); err != nil {
		log.Error(err, "Failed to ensure workspace service")
		return ctrl.Result{RequeueAfter: 15 * time.Second}, err
	}

	// Phase 4: Check pod status
	r.syncPodStatus(ctx, ws)

	// Phase 5: Check idle timeout
	if ws.Spec.IdleTimeoutMinutes > 0 {
		r.checkIdleTimeout(ws)
	}

	// Phase 6: Check max lifetime
	if ws.Spec.MaxLifetimeHours > 0 {
		if exceeded, err := r.checkMaxLifetime(ctx, ws); exceeded || err != nil {
			if exceeded {
				return ctrl.Result{}, nil // Workspace terminated
			}
			return ctrl.Result{RequeueAfter: 30 * time.Second}, err
		}
	}

	if err := r.Status().Update(ctx, ws); err != nil {
		log.Error(err, "Failed to update workspace status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: 60 * time.Second}, nil
}

// ensureWorkspacePVC creates a PVC for persistent workspace storage.
func (r *FabricWorkspaceReconciler) ensureWorkspacePVC(ctx context.Context, ws *gryviav1.FabricWorkspace) error {
	pvcName := fmt.Sprintf("%s-workspace", ws.Name)

	pvc := &corev1.PersistentVolumeClaim{}
	err := r.Get(ctx, types.NamespacedName{Namespace: ws.Namespace, Name: pvcName}, pvc)
	if err == nil {
		ws.Status.PVCName = pvcName
		return nil
	}
	if !errors.IsNotFound(err) {
		return err
	}

	storageSize := ws.Spec.Storage
	if storageSize == "" {
		storageSize = "50Gi"
	}
	storageQuantity, parseErr := resource.ParseQuantity(storageSize)
	if parseErr != nil {
		return fmt.Errorf("invalid storage size %q: %w", storageSize, parseErr)
	}

	pvc = &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: ws.Namespace,
			Labels: map[string]string{
				"gryvia.io/workspace": ws.Name,
				"gryvia.io/component": "workspace-data",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: storageQuantity,
				},
			},
		},
	}

	if ws.Spec.StorageClassName != "" {
		pvc.Spec.StorageClassName = &ws.Spec.StorageClassName
	}

	if err := controllerutil.SetControllerReference(ws, pvc, r.Scheme); err != nil {
		return err
	}

	ws.Status.PVCName = pvcName
	return r.Create(ctx, pvc)
}

// ensureWorkspacePod creates or manages the workspace pod.
func (r *FabricWorkspaceReconciler) ensureWorkspacePod(ctx context.Context, ws *gryviav1.FabricWorkspace) error {
	podName := fmt.Sprintf("%s-workspace", ws.Name)

	pod := &corev1.Pod{}
	err := r.Get(ctx, types.NamespacedName{Namespace: ws.Namespace, Name: podName}, pod)
	if err == nil {
		ws.Status.PodName = podName
		return nil
	}
	if !errors.IsNotFound(err) {
		return err
	}

	// Build the workspace pod
	image := r.getWorkspaceImage(ws)

	port := int32(8888) // JupyterLab
	if ws.Spec.Type == gryviav1.WorkspaceTypeVSCode {
		port = 8080
	}

	container := corev1.Container{
		Name:  "workspace",
		Image: image,
		Ports: []corev1.ContainerPort{
			{
				Name:          "http",
				ContainerPort: port,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/",
					Port: intstr.FromInt32(port),
				},
			},
			InitialDelaySeconds: 15,
			PeriodSeconds:       10,
		},
	}

	// Add resource requests/limits
	resources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{},
		Limits:   corev1.ResourceList{},
	}
	if ws.Spec.CPURequest != "" {
		resources.Requests[corev1.ResourceCPU] = resource.MustParse(ws.Spec.CPURequest)
	}
	if ws.Spec.MemRequest != "" {
		resources.Requests[corev1.ResourceMemory] = resource.MustParse(ws.Spec.MemRequest)
	}
	if ws.Spec.CPULimit != "" {
		resources.Limits[corev1.ResourceCPU] = resource.MustParse(ws.Spec.CPULimit)
	}
	if ws.Spec.MemLimit != "" {
		resources.Limits[corev1.ResourceMemory] = resource.MustParse(ws.Spec.MemLimit)
	}
	if ws.Spec.GPUCount > 0 {
		resources.Limits["nvidia.com/gpu"] = *resource.NewQuantity(int64(ws.Spec.GPUCount), resource.DecimalSI)
	}
	container.Resources = resources

	// Add environment variables
	for k, v := range ws.Spec.Env {
		container.Env = append(container.Env, corev1.EnvVar{Name: k, Value: v})
	}

	// Add volume mounts
	var volumeMounts []corev1.VolumeMount
	var volumes []corev1.Volume

	if ws.Spec.Storage != "" {
		pvcName := fmt.Sprintf("%s-workspace", ws.Name)
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "workspace-data",
			MountPath: "/workspace",
		})
		volumes = append(volumes, corev1.Volume{
			Name: "workspace-data",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
				},
			},
		})
	}

	// Add shared memory volume for GPU workloads
	if ws.Spec.GPUCount > 0 {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "shm",
			MountPath: "/dev/shm",
		})
		volumes = append(volumes, corev1.Volume{
			Name: "shm",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					Medium: corev1.StorageMediumMemory,
				},
			},
		})
	}

	container.VolumeMounts = volumeMounts

	// Build node selector
	nodeSelector := map[string]string{}
	if ws.Spec.GPUType != "" && ws.Spec.GPUType != "any" {
		nodeSelector["gryvia.io/gpu"] = ws.Spec.GPUType
	}

	pod = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: ws.Namespace,
			Labels: map[string]string{
				"gryvia.io/workspace": ws.Name,
				"gryvia.io/component": "workspace-pod",
				"gryvia.io/type":      string(ws.Spec.Type),
			},
		},
		Spec: corev1.PodSpec{
			Containers:    []corev1.Container{container},
			Volumes:       volumes,
			NodeSelector:  nodeSelector,
			RestartPolicy: corev1.RestartPolicyAlways,
		},
	}

	if err := controllerutil.SetControllerReference(ws, pod, r.Scheme); err != nil {
		return err
	}

	ws.Status.PodName = podName
	return r.Create(ctx, pod)
}

// ensureWorkspaceService creates a Service to expose the workspace.
func (r *FabricWorkspaceReconciler) ensureWorkspaceService(ctx context.Context, ws *gryviav1.FabricWorkspace) error {
	svcName := fmt.Sprintf("%s-workspace", ws.Name)

	svc := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{Namespace: ws.Namespace, Name: svcName}, svc)
	if err == nil {
		ws.Status.ServiceName = svcName
		return nil
	}
	if !errors.IsNotFound(err) {
		return err
	}

	port := int32(8888)
	if ws.Spec.Type == gryviav1.WorkspaceTypeVSCode {
		port = 8080
	}

	svc = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svcName,
			Namespace: ws.Namespace,
			Labels: map[string]string{
				"gryvia.io/workspace": ws.Name,
				"gryvia.io/component": "workspace-service",
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"gryvia.io/workspace": ws.Name,
				"gryvia.io/component": "workspace-pod",
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

	if err := controllerutil.SetControllerReference(ws, svc, r.Scheme); err != nil {
		return err
	}

	ws.Status.ServiceName = svcName
	ws.Status.URL = fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", svcName, ws.Namespace, port)
	return r.Create(ctx, svc)
}

// syncPodStatus updates workspace status based on the pod state.
func (r *FabricWorkspaceReconciler) syncPodStatus(ctx context.Context, ws *gryviav1.FabricWorkspace) {
	if ws.Status.PodName == "" {
		return
	}

	pod := &corev1.Pod{}
	err := r.Get(ctx, types.NamespacedName{Namespace: ws.Namespace, Name: ws.Status.PodName}, pod)
	if err != nil {
		if errors.IsNotFound(err) {
			ws.Status.Phase = string(gryviav1.WorkspacePhasePending)
			ws.Status.PodName = ""
		}
		return
	}

	switch pod.Status.Phase {
	case corev1.PodRunning:
		ws.Status.Phase = string(gryviav1.WorkspacePhaseRunning)
		if ws.Status.StartTime == nil {
			now := metav1.Now()
			ws.Status.StartTime = &now
		}
		// Assume activity when running; a real implementation would check proxy logs
		now := metav1.Now()
		ws.Status.LastActivity = &now
		r.updateWSCondition(ws, ConditionWorkspaceReady, metav1.ConditionTrue, "Running", "Workspace pod is running")

	case corev1.PodPending:
		ws.Status.Phase = string(gryviav1.WorkspacePhasePending)

	case corev1.PodFailed:
		ws.Status.Phase = string(gryviav1.WorkspacePhaseFailed)
		ws.Status.Message = "Workspace pod failed"
		r.updateWSCondition(ws, ConditionWorkspaceReady, metav1.ConditionFalse, "Failed", "Workspace pod failed")
	}
}

// checkIdleTimeout checks if the workspace has been idle for too long.
func (r *FabricWorkspaceReconciler) checkIdleTimeout(ws *gryviav1.FabricWorkspace) {
	if ws.Status.LastActivity == nil || ws.Status.Phase != string(gryviav1.WorkspacePhaseRunning) {
		return
	}

	idleDuration := time.Since(ws.Status.LastActivity.Time)
	timeout := time.Duration(ws.Spec.IdleTimeoutMinutes) * time.Minute

	if idleDuration > timeout {
		ws.Status.Phase = string(gryviav1.WorkspacePhaseIdle)
		ws.Status.Message = fmt.Sprintf("Workspace idle for %s (timeout: %s)", idleDuration.Truncate(time.Minute), timeout)
		r.updateWSCondition(ws, ConditionWorkspaceIdle, metav1.ConditionTrue, "Idle",
			fmt.Sprintf("No activity for %d minutes", ws.Spec.IdleTimeoutMinutes))
	}
}

// checkMaxLifetime checks if the workspace has exceeded its maximum lifetime.
func (r *FabricWorkspaceReconciler) checkMaxLifetime(ctx context.Context, ws *gryviav1.FabricWorkspace) (bool, error) {
	if ws.Status.StartTime == nil {
		return false, nil
	}

	lifetime := time.Since(ws.Status.StartTime.Time)
	maxLifetime := time.Duration(ws.Spec.MaxLifetimeHours) * time.Hour

	if lifetime > maxLifetime {
		// Terminate workspace by deleting the pod
		if ws.Status.PodName != "" {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      ws.Status.PodName,
					Namespace: ws.Namespace,
				},
			}
			if err := r.Delete(ctx, pod); err != nil && !errors.IsNotFound(err) {
				return false, err
			}
		}

		ws.Status.Phase = string(gryviav1.WorkspacePhaseTerminating)
		ws.Status.Message = fmt.Sprintf("Workspace exceeded max lifetime of %d hours", ws.Spec.MaxLifetimeHours)
		ws.Status.PodName = ""

		if err := r.Status().Update(ctx, ws); err != nil {
			return true, err
		}
		return true, nil
	}

	return false, nil
}

// reconcilePaused handles pausing a workspace by deleting its pod (PVC persists for resume).
func (r *FabricWorkspaceReconciler) reconcilePaused(ctx context.Context, ws *gryviav1.FabricWorkspace) (ctrl.Result, error) {
	if ws.Status.Phase == string(gryviav1.WorkspacePhasePaused) {
		return ctrl.Result{}, nil
	}

	// Delete the pod to free resources
	if ws.Status.PodName != "" {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ws.Status.PodName,
				Namespace: ws.Namespace,
			},
		}
		if err := r.Delete(ctx, pod); err != nil && !errors.IsNotFound(err) {
			return ctrl.Result{RequeueAfter: 15 * time.Second}, err
		}
		ws.Status.PodName = ""
	}

	ws.Status.Phase = string(gryviav1.WorkspacePhasePaused)
	ws.Status.Message = "Workspace paused - PVC retained for resume"
	r.updateWSCondition(ws, ConditionWorkspaceReady, metav1.ConditionFalse, "Paused", "Workspace paused by user")

	if err := r.Status().Update(ctx, ws); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// getWorkspaceImage returns the container image for the workspace type.
func (r *FabricWorkspaceReconciler) getWorkspaceImage(ws *gryviav1.FabricWorkspace) string {
	if ws.Spec.Image != "" {
		return ws.Spec.Image
	}

	switch ws.Spec.Type {
	case gryviav1.WorkspaceTypeJupyter:
		return defaultJupyterImage
	case gryviav1.WorkspaceTypeVSCode:
		return defaultVSCodeImage
	default:
		return defaultJupyterImage
	}
}

func (r *FabricWorkspaceReconciler) updateWSCondition(ws *gryviav1.FabricWorkspace, condType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: ws.Generation,
		LastTransitionTime: metav1.Now(),
	}

	for _, cond := range ws.Status.Conditions {
		if cond.Type == condType && cond.Status == status {
			condition.LastTransitionTime = cond.LastTransitionTime
			break
		}
	}

	found := false
	for i, cond := range ws.Status.Conditions {
		if cond.Type == condType {
			ws.Status.Conditions[i] = condition
			found = true
			break
		}
	}
	if !found {
		ws.Status.Conditions = append(ws.Status.Conditions, condition)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *FabricWorkspaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gryviav1.FabricWorkspace{}).
		Owns(&corev1.Pod{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Complete(r)
}
