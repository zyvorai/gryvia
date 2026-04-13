package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kubefabricv1 "github.com/ssahani/kube-fabric/operators/ai-operator/api/v1"
	"github.com/ssahani/kube-fabric/operators/ai-operator/pkg/scheduler"
)

const (
	// Status phases
	PhasePending    = "Pending"
	PhaseScheduling = "Scheduling"
	PhaseRunning    = "Running"
	PhaseSucceeded  = "Succeeded"
	PhaseFailed     = "Failed"
	PhaseUnknown    = "Unknown"

	// Condition types
	ConditionScheduled = "Scheduled"
	ConditionReady     = "Ready"
)

// FabricAIJobReconciler reconciles a FabricAIJob object
type FabricAIJobReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

//+kubebuilder:rbac:groups=kubefabric.ai,resources=fabricaijobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kubefabric.ai,resources=fabricaijobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kubefabric.ai,resources=fabricaijobs/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop
func (r *FabricAIJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("fabricaijob", req.NamespacedName)

	// Fetch the FabricAIJob instance
	job := &kubefabricv1.FabricAIJob{}
	err := r.Get(ctx, req.NamespacedName, job)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("FabricAIJob resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get FabricAIJob")
		return ctrl.Result{}, err
	}

	// Handle deletion - owned resources (StatefulSet, Service, PVC) are
	// garbage-collected via controller references, so no finalizer is needed.
	if !job.ObjectMeta.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	// Initialize status if needed
	if job.Status.Phase == "" {
		job.Status.Phase = PhasePending
		if err := r.Status().Update(ctx, job); err != nil {
			log.Error(err, "Failed to update FabricAIJob status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Reconcile the AI job
	result, err := r.reconcileAIJob(ctx, job)
	if err != nil {
		log.Error(err, "Failed to reconcile AI job")
		return result, err
	}

	return result, nil
}

func (r *FabricAIJobReconciler) reconcileAIJob(ctx context.Context, job *kubefabricv1.FabricAIJob) (ctrl.Result, error) {
	log := r.Log.WithValues("fabricaijob", job.Name)

	// Phase 1: Scheduling - Find suitable GPU nodes
	if job.Status.Phase == PhasePending || (job.Status.Phase == PhaseScheduling && len(job.Status.NodesAllocated) == 0) {
		log.Info("Scheduling AI job")
		job.Status.Phase = PhaseScheduling

		// Use GPU-aware scheduler to find optimal nodes
		nodes, err := scheduler.FindOptimalNodes(ctx, r.Client, job)
		if err != nil {
			log.Error(err, "Failed to schedule job")
			r.updateCondition(job, ConditionScheduled, metav1.ConditionFalse, "SchedulingFailed", err.Error())
			job.Status.Phase = PhasePending
			if updateErr := r.Status().Update(ctx, job); updateErr != nil {
				log.Error(updateErr, "Failed to update status after scheduling failure")
			}
			return ctrl.Result{RequeueAfter: 30 * time.Second}, err
		}

		job.Status.NodesAllocated = nodes
		job.Status.GpusAllocated = job.Spec.GPUs
		r.updateCondition(job, ConditionScheduled, metav1.ConditionTrue, "Scheduled", "Job scheduled successfully")

		if err := r.Status().Update(ctx, job); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Phase 2: Create PVC if storage is specified
	if job.Spec.Storage != "" {
		if err := r.ensurePVC(ctx, job); err != nil {
			log.Error(err, "Failed to create PVC")
			return ctrl.Result{RequeueAfter: 10 * time.Second}, err
		}
	}

	// Phase 3: Create headless service (required by StatefulSet's ServiceName field)
	if err := r.ensureHeadlessService(ctx, job); err != nil {
		log.Error(err, "Failed to create headless service")
		return ctrl.Result{RequeueAfter: 10 * time.Second}, err
	}

	// Phase 4: Create StatefulSet for the training workload
	if err := r.ensureStatefulSet(ctx, job); err != nil {
		log.Error(err, "Failed to create StatefulSet")
		return ctrl.Result{RequeueAfter: 10 * time.Second}, err
	}

	// Phase 5: Update status based on StatefulSet status
	sts := &appsv1.StatefulSet{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: job.Namespace,
		Name:      r.getStatefulSetName(job),
	}, sts)
	if err != nil {
		return ctrl.Result{}, err
	}

	job.Status.ReplicasReady = sts.Status.ReadyReplicas

	// Update phase based on replicas
	desiredReplicas := r.getReplicaCount(job)
	if sts.Status.ReadyReplicas > 0 {
		if job.Status.Phase != PhaseRunning {
			if job.Status.StartTime == nil {
				now := metav1.Now()
				job.Status.StartTime = &now
			}
			job.Status.Phase = PhaseRunning
		}
		if sts.Status.ReadyReplicas == desiredReplicas {
			r.updateCondition(job, ConditionReady, metav1.ConditionTrue, "Ready", "All replicas are ready")
		} else {
			r.updateCondition(job, ConditionReady, metav1.ConditionFalse, "PartiallyReady",
				fmt.Sprintf("%d/%d replicas are ready", sts.Status.ReadyReplicas, desiredReplicas))
		}
	}

	// Check for job completion by examining pod status
	if job.Status.Phase == PhaseRunning {
		pods := &corev1.PodList{}
		if err := r.List(ctx, pods, client.InNamespace(job.Namespace), client.MatchingLabels{"kubefabric.ai/job": job.Name}); err == nil {
			completedPods := 0
			failedPods := 0
			for _, pod := range pods.Items {
				if pod.Status.Phase == corev1.PodSucceeded {
					completedPods++
				} else if pod.Status.Phase == corev1.PodFailed {
					failedPods++
				}
			}
			replicas := int(r.getReplicaCount(job))
			if completedPods >= replicas {
				job.Status.Phase = PhaseSucceeded
			} else if failedPods > 0 && (completedPods+failedPods) >= replicas {
				// Only mark Failed when all pods have terminated and at least one failed
				job.Status.Phase = PhaseFailed
				job.Status.Message = fmt.Sprintf("%d/%d pod(s) failed", failedPods, replicas)
			}
		}
	}

	// Set CompletionTime when job transitions to terminal state
	if (job.Status.Phase == PhaseSucceeded || job.Status.Phase == PhaseFailed) && job.Status.CompletionTime == nil {
		now := metav1.Now()
		job.Status.CompletionTime = &now
	}

	if err := r.Status().Update(ctx, job); err != nil {
		log.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	// Only requeue if job is still active (not completed or failed)
	if job.Status.Phase == PhaseSucceeded || job.Status.Phase == PhaseFailed {
		return ctrl.Result{}, nil
	}

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

func (r *FabricAIJobReconciler) ensurePVC(ctx context.Context, job *kubefabricv1.FabricAIJob) error {
	pvcName := fmt.Sprintf("%s-data", job.Name)

	pvc := &corev1.PersistentVolumeClaim{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: job.Namespace,
		Name:      pvcName,
	}, pvc)

	if err != nil && errors.IsNotFound(err) {
		// Parse storage size safely
		storageQuantity, parseErr := resource.ParseQuantity(job.Spec.StorageSize())
		if parseErr != nil {
			return fmt.Errorf("invalid storage size %q: %w", job.Spec.StorageSize(), parseErr)
		}

		// Create PVC
		pvc = &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvcName,
				Namespace: job.Namespace,
				Labels: map[string]string{
					"kubefabric.ai/job": job.Name,
				},
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteMany,
				},
				StorageClassName: &job.Spec.Storage,
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: storageQuantity,
					},
				},
			},
		}

		if err := controllerutil.SetControllerReference(job, pvc, r.Scheme); err != nil {
			return err
		}

		return r.Create(ctx, pvc)
	}

	return err
}

func (r *FabricAIJobReconciler) ensureHeadlessService(ctx context.Context, job *kubefabricv1.FabricAIJob) error {
	svcName := fmt.Sprintf("%s-headless", job.Name)

	svc := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: job.Namespace,
		Name:      svcName,
	}, svc)

	if err != nil && errors.IsNotFound(err) {
		// Create headless service
		svc = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      svcName,
				Namespace: job.Namespace,
				Labels: map[string]string{
					"kubefabric.ai/job": job.Name,
				},
			},
			Spec: corev1.ServiceSpec{
				ClusterIP: "None",
				Selector: map[string]string{
					"kubefabric.ai/job": job.Name,
				},
				Ports: []corev1.ServicePort{
					{
						Name: "torch-dist",
						Port: 29500,
					},
				},
			},
		}

		if err := controllerutil.SetControllerReference(job, svc, r.Scheme); err != nil {
			return err
		}

		return r.Create(ctx, svc)
	}

	return err
}

func (r *FabricAIJobReconciler) ensureStatefulSet(ctx context.Context, job *kubefabricv1.FabricAIJob) error {
	stsName := r.getStatefulSetName(job)

	sts := &appsv1.StatefulSet{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: job.Namespace,
		Name:      stsName,
	}, sts)

	if err != nil && errors.IsNotFound(err) {
		// Create StatefulSet
		sts = r.buildStatefulSet(job)

		if err := controllerutil.SetControllerReference(job, sts, r.Scheme); err != nil {
			return err
		}

		return r.Create(ctx, sts)
	}
	if err != nil {
		return err
	}

	// Update StatefulSet if the job spec has changed
	desired := r.buildStatefulSet(job)
	needsUpdate := false

	// Check replica count
	if *sts.Spec.Replicas != *desired.Spec.Replicas {
		sts.Spec.Replicas = desired.Spec.Replicas
		needsUpdate = true
	}

	// Check container image
	if len(sts.Spec.Template.Spec.Containers) > 0 && len(desired.Spec.Template.Spec.Containers) > 0 {
		if sts.Spec.Template.Spec.Containers[0].Image != desired.Spec.Template.Spec.Containers[0].Image {
			sts.Spec.Template.Spec.Containers[0].Image = desired.Spec.Template.Spec.Containers[0].Image
			needsUpdate = true
		}

		// Check command
		if !stringSlicesEqual(sts.Spec.Template.Spec.Containers[0].Command, desired.Spec.Template.Spec.Containers[0].Command) {
			sts.Spec.Template.Spec.Containers[0].Command = desired.Spec.Template.Spec.Containers[0].Command
			needsUpdate = true
		}

		// Check args
		if !stringSlicesEqual(sts.Spec.Template.Spec.Containers[0].Args, desired.Spec.Template.Spec.Containers[0].Args) {
			sts.Spec.Template.Spec.Containers[0].Args = desired.Spec.Template.Spec.Containers[0].Args
			needsUpdate = true
		}

		// Check env vars
		if !envVarsEqual(sts.Spec.Template.Spec.Containers[0].Env, desired.Spec.Template.Spec.Containers[0].Env) {
			sts.Spec.Template.Spec.Containers[0].Env = desired.Spec.Template.Spec.Containers[0].Env
			needsUpdate = true
		}

		// Check resource limits (GPU count changes)
		desiredLimits := desired.Spec.Template.Spec.Containers[0].Resources.Limits
		currentLimits := sts.Spec.Template.Spec.Containers[0].Resources.Limits
		gpuResource := corev1.ResourceName("nvidia.com/gpu")
		desiredGPU, desiredHas := desiredLimits[gpuResource]
		currentGPU, currentHas := currentLimits[gpuResource]
		if desiredHas != currentHas || (desiredHas && !desiredGPU.Equal(currentGPU)) {
			sts.Spec.Template.Spec.Containers[0].Resources = desired.Spec.Template.Spec.Containers[0].Resources
			needsUpdate = true
		}
	}

	// Check annotations (network mode changes)
	if !mapsEqual(sts.Spec.Template.Annotations, desired.Spec.Template.Annotations) {
		sts.Spec.Template.Annotations = desired.Spec.Template.Annotations
		needsUpdate = true
	}

	// Check node selector
	if !mapsEqual(sts.Spec.Template.Spec.NodeSelector, desired.Spec.Template.Spec.NodeSelector) {
		sts.Spec.Template.Spec.NodeSelector = desired.Spec.Template.Spec.NodeSelector
		needsUpdate = true
	}

	if needsUpdate {
		return r.Update(ctx, sts)
	}

	return nil
}

func (r *FabricAIJobReconciler) buildStatefulSet(job *kubefabricv1.FabricAIJob) *appsv1.StatefulSet {
	labels := map[string]string{
		"kubefabric.ai/job":  job.Name,
		"kubefabric.ai/type": job.Spec.Type,
	}

	replicas := r.getReplicaCount(job)
	gpusPerPod := r.getGPUsPerPod(job)

	// Build pod template
	podTemplate := r.buildPodTemplate(job, labels, gpusPerPod)

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.getStatefulSetName(job),
			Namespace: job.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    &replicas,
			ServiceName: fmt.Sprintf("%s-headless", job.Name),
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: podTemplate,
		},
	}

	return sts
}

func (r *FabricAIJobReconciler) buildPodTemplate(job *kubefabricv1.FabricAIJob, labels map[string]string, gpusPerPod int32) corev1.PodTemplateSpec {
	annotations := make(map[string]string)

	// Add RDMA annotation if network mode is RDMA
	if job.Spec.Network == "rdma" {
		annotations["kubefabric.ai/rdma"] = "true"
		annotations["k8s.v1.cni.cncf.io/networks"] = "rdma-network"
	}

	// Add SR-IOV annotation if network mode is SR-IOV
	if job.Spec.Network == "sriov" {
		annotations["kubefabric.ai/sriov"] = "true"
		annotations["k8s.v1.cni.cncf.io/networks"] = "sriov-network"
	}

	// Build container
	container := corev1.Container{
		Name:            "trainer",
		Image:           job.Spec.Image,
		ImagePullPolicy: job.Spec.ImagePullPolicy,
		Command:         job.Spec.Command,
		Args:            job.Spec.Args,
		Env:             r.buildEnvVars(job),
		WorkingDir:      job.Spec.WorkingDir,
		VolumeMounts:    r.buildVolumeMounts(job),
		Resources:       r.buildResources(job, gpusPerPod),
	}

	// Build pod spec
	podSpec := corev1.PodSpec{
		Containers:   []corev1.Container{container},
		Volumes:      r.buildVolumes(job),
		NodeSelector: r.buildNodeSelector(job),
		Tolerations:  job.Spec.Tolerations,
		Affinity:     job.Spec.Affinity,
		// StatefulSets only support RestartPolicyAlways; use a Job or custom
		// completion detection for run-to-completion semantics.
		RestartPolicy: corev1.RestartPolicyAlways,
	}

	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: podSpec,
	}
}

func (r *FabricAIJobReconciler) buildEnvVars(job *kubefabricv1.FabricAIJob) []corev1.EnvVar {
	// Copy to avoid mutating the spec
	envVars := make([]corev1.EnvVar, len(job.Spec.Env))
	copy(envVars, job.Spec.Env)

	// Add distributed training env vars if enabled
	if job.Spec.Distributed != nil && job.Spec.Distributed.Enabled {
		worldSize := r.getReplicaCount(job) * r.getGPUsPerPod(job)
		envVars = append(envVars,
			corev1.EnvVar{Name: "MASTER_ADDR", Value: fmt.Sprintf("%s-training-0.%s-headless", job.Name, job.Name)},
			corev1.EnvVar{Name: "MASTER_PORT", Value: "29500"},
			corev1.EnvVar{Name: "WORLD_SIZE", Value: fmt.Sprintf("%d", worldSize)},
			corev1.EnvVar{Name: "NCCL_DEBUG", Value: "INFO"},
		)

		if job.Spec.Network == "rdma" {
			envVars = append(envVars,
				corev1.EnvVar{Name: "NCCL_IB_DISABLE", Value: "0"},
				corev1.EnvVar{Name: "NCCL_NET_GDR_LEVEL", Value: "5"},
			)
		}
	}

	return envVars
}

func (r *FabricAIJobReconciler) buildVolumeMounts(job *kubefabricv1.FabricAIJob) []corev1.VolumeMount {
	// Copy to avoid mutating the spec
	volumeMounts := make([]corev1.VolumeMount, len(job.Spec.VolumeMounts))
	copy(volumeMounts, job.Spec.VolumeMounts)

	// Add data volume if storage is specified
	if job.Spec.Storage != "" {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "data",
			MountPath: "/data",
		})
	}

	// Add shared memory for distributed training
	if job.Spec.Distributed != nil && job.Spec.Distributed.Enabled {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "shm",
			MountPath: "/dev/shm",
		})
	}

	return volumeMounts
}

func (r *FabricAIJobReconciler) buildVolumes(job *kubefabricv1.FabricAIJob) []corev1.Volume {
	// Copy to avoid mutating the spec
	volumes := make([]corev1.Volume, len(job.Spec.Volumes))
	copy(volumes, job.Spec.Volumes)

	// Add PVC volume if storage is specified
	if job.Spec.Storage != "" {
		volumes = append(volumes, corev1.Volume{
			Name: "data",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: fmt.Sprintf("%s-data", job.Name),
				},
			},
		})
	}

	// Add shared memory for distributed training
	if job.Spec.Distributed != nil && job.Spec.Distributed.Enabled {
		volumes = append(volumes, corev1.Volume{
			Name: "shm",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					Medium: corev1.StorageMediumMemory,
				},
			},
		})
	}

	return volumes
}

func (r *FabricAIJobReconciler) buildResources(job *kubefabricv1.FabricAIJob, gpusPerPod int32) corev1.ResourceRequirements {
	resources := job.Spec.Resources

	// Add GPU resource limits
	if resources.Limits == nil {
		resources.Limits = corev1.ResourceList{}
	}
	resources.Limits["nvidia.com/gpu"] = *resource.NewQuantity(int64(gpusPerPod), resource.DecimalSI)

	return resources
}

func (r *FabricAIJobReconciler) buildNodeSelector(job *kubefabricv1.FabricAIJob) map[string]string {
	// Copy to avoid mutating the spec
	nodeSelector := make(map[string]string)
	for k, v := range job.Spec.NodeSelector {
		nodeSelector[k] = v
	}

	// Add GPU type selector if specified
	if job.Spec.GpuType != "" && job.Spec.GpuType != "any" {
		nodeSelector["kubefabric.ai/gpu"] = job.Spec.GpuType
	}

	// Add RDMA selector if network mode is RDMA
	if job.Spec.Network == "rdma" {
		nodeSelector["kubefabric.ai/rdma"] = "true"
	}

	return nodeSelector
}

func (r *FabricAIJobReconciler) getStatefulSetName(job *kubefabricv1.FabricAIJob) string {
	return fmt.Sprintf("%s-training", job.Name)
}

func (r *FabricAIJobReconciler) getReplicaCount(job *kubefabricv1.FabricAIJob) int32 {
	if job.Spec.Distributed != nil && job.Spec.Distributed.Enabled {
		if job.Spec.Distributed.Nodes > 0 {
			return job.Spec.Distributed.Nodes
		}
		return 1
	}
	return 1
}

func (r *FabricAIJobReconciler) getGPUsPerPod(job *kubefabricv1.FabricAIJob) int32 {
	if job.Spec.Distributed != nil && job.Spec.Distributed.Enabled {
		if job.Spec.Distributed.GpusPerNode > 0 {
			return job.Spec.Distributed.GpusPerNode
		}
		return 1
	}
	if job.Spec.GPUs > 0 {
		return job.Spec.GPUs
	}
	return 1
}

func (r *FabricAIJobReconciler) updateCondition(job *kubefabricv1.FabricAIJob, condType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: job.Generation,
		LastTransitionTime: metav1.Now(),
	}

	// Only update LastTransitionTime when status actually changes
	for _, cond := range job.Status.Conditions {
		if cond.Type == condType && cond.Status == status {
			condition.LastTransitionTime = cond.LastTransitionTime
			break
		}
	}

	// Find and update existing condition or append new one
	found := false
	for i, cond := range job.Status.Conditions {
		if cond.Type == condType {
			job.Status.Conditions[i] = condition
			found = true
			break
		}
	}
	if !found {
		job.Status.Conditions = append(job.Status.Conditions, condition)
	}
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func envVarsEqual(a, b []corev1.EnvVar) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || a[i].Value != b[i].Value {
			return false
		}
		// Compare ValueFrom references
		if (a[i].ValueFrom == nil) != (b[i].ValueFrom == nil) {
			return false
		}
	}
	return true
}

func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// SetupWithManager sets up the controller with the Manager.
func (r *FabricAIJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubefabricv1.FabricAIJob{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Complete(r)
}
