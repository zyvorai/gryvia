package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gryviav1 "github.com/zyvorai/gryvia/operators/gpu-operator/api/v1"
)

const (
	defaultSharingReconcileInterval = 30 * time.Second
)

// FabricGPUSharingPolicyReconciler reconciles a FabricGPUSharingPolicy object
type FabricGPUSharingPolicyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

//+kubebuilder:rbac:groups=gryvia.io,resources=fabricgpusharingpolicies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gryvia.io,resources=fabricgpusharingpolicies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gryvia.io,resources=fabricgpusharingpolicies/finalizers,verbs=update
//+kubebuilder:rbac:groups=gryvia.io,resources=fabricgpunodes,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete

func (r *FabricGPUSharingPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("fabricgpusharingpolicy", req.NamespacedName)

	// Fetch the FabricGPUSharingPolicy instance
	policy := &gryviav1.FabricGPUSharingPolicy{}
	err := r.Get(ctx, req.NamespacedName, policy)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("FabricGPUSharingPolicy resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get FabricGPUSharingPolicy")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !policy.ObjectMeta.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	log.Info("Reconciling FabricGPUSharingPolicy", "strategy", policy.Spec.Strategy)

	// Reconcile the GPU sharing policy
	result, err := r.reconcileGPUSharing(ctx, policy)
	if err != nil {
		log.Error(err, "Failed to reconcile GPU sharing policy")
		return result, err
	}

	return result, nil
}

func (r *FabricGPUSharingPolicyReconciler) reconcileGPUSharing(ctx context.Context, policy *gryviav1.FabricGPUSharingPolicy) (ctrl.Result, error) {
	log := r.Log.WithValues("policy", policy.Name)

	// Find matching GPU nodes
	matchingNodes, err := r.findMatchingNodes(ctx, policy)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to find matching nodes: %w", err)
	}

	// Update affected nodes in status
	var affectedNodeNames []string
	for _, node := range matchingNodes {
		affectedNodeNames = append(affectedNodeNames, node.Spec.NodeName)
	}
	policy.Status.AffectedNodes = affectedNodeNames

	// Calculate total physical GPUs
	totalGPUs := 0
	for _, node := range matchingNodes {
		totalGPUs += node.Spec.GpuCount
	}
	policy.Status.TotalGPUs = totalGPUs

	// Calculate effective GPUs based on sharing strategy
	policy.Status.EffectiveGPUs = r.calculateEffectiveGPUs(policy, totalGPUs)

	// Configure sharing on matching nodes based on strategy
	switch policy.Spec.Strategy {
	case "time-slicing":
		if err := r.configureTimeSlicing(ctx, policy, matchingNodes); err != nil {
			log.Error(err, "Failed to configure time-slicing")
		}
	case "mig":
		if err := r.configureMIG(ctx, policy, matchingNodes); err != nil {
			log.Error(err, "Failed to configure MIG")
		}
	case "fractional":
		if err := r.configureFractional(ctx, policy, matchingNodes); err != nil {
			log.Error(err, "Failed to configure fractional GPU")
		}
	}

	// Track per-pod GPU utilization
	r.trackPerPodUtilization(ctx, policy, matchingNodes)

	// Calculate current allocations
	policy.Status.CurrentAllocations = r.calculateAllocations(ctx, policy, matchingNodes)

	// Enforce fair sharing with utilization limits
	r.enforceFairSharing(ctx, policy, matchingNodes)

	// Update condition
	r.updateSharingCondition(policy, "Ready", metav1.ConditionTrue, "SharingActive",
		fmt.Sprintf("GPU sharing policy applied: %d physical GPUs -> %d effective GPUs",
			policy.Status.TotalGPUs, policy.Status.EffectiveGPUs))

	// Update status
	if err := r.Status().Update(ctx, policy); err != nil {
		log.Error(err, "Failed to update GPU sharing policy status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: defaultSharingReconcileInterval}, nil
}

func (r *FabricGPUSharingPolicyReconciler) findMatchingNodes(ctx context.Context, policy *gryviav1.FabricGPUSharingPolicy) ([]gryviav1.FabricGpuNode, error) {
	gpuNodeList := &gryviav1.FabricGpuNodeList{}
	if err := r.List(ctx, gpuNodeList); err != nil {
		return nil, fmt.Errorf("failed to list FabricGpuNodes: %w", err)
	}

	var matching []gryviav1.FabricGpuNode
	for _, node := range gpuNodeList.Items {
		if r.nodeMatchesSelector(node, policy.Spec.NodeSelector) {
			matching = append(matching, node)
		}
	}

	return matching, nil
}

func (r *FabricGPUSharingPolicyReconciler) nodeMatchesSelector(node gryviav1.FabricGpuNode, selector map[string]string) bool {
	if len(selector) == 0 {
		return true
	}

	nodeLabels := node.Labels
	if nodeLabels == nil {
		nodeLabels = make(map[string]string)
	}

	// Also check against node spec fields
	effectiveLabels := make(map[string]string)
	for k, v := range nodeLabels {
		effectiveLabels[k] = v
	}
	effectiveLabels["gryvia.io/gpu-type"] = node.Spec.GpuType

	for key, value := range selector {
		if effectiveLabels[key] != value {
			return false
		}
	}

	return true
}

func (r *FabricGPUSharingPolicyReconciler) calculateEffectiveGPUs(policy *gryviav1.FabricGPUSharingPolicy, totalPhysical int) int {
	switch policy.Spec.Strategy {
	case "time-slicing":
		if policy.Spec.TimeSlicing != nil && policy.Spec.TimeSlicing.MaxPodsPerGPU > 0 {
			return totalPhysical * policy.Spec.TimeSlicing.MaxPodsPerGPU
		}
		return totalPhysical * 4 // Default 4x

	case "fractional":
		multiplier := 4 // Default 1/4
		if policy.Spec.FractionalGPU != nil {
			switch policy.Spec.FractionalGPU.Granularity {
			case "1/2":
				multiplier = 2
			case "1/4":
				multiplier = 4
			case "1/8":
				multiplier = 8
			case "1/16":
				multiplier = 16
			}
		}
		return totalPhysical * multiplier

	case "mig":
		if policy.Spec.MIG != nil {
			totalProfiles := 0
			for _, profile := range policy.Spec.MIG.Profiles {
				totalProfiles += profile.Count
			}
			if totalProfiles > 0 {
				return totalPhysical * totalProfiles
			}
		}
		return totalPhysical * 7 // Default A100 MIG profiles

	default:
		return totalPhysical
	}
}

func (r *FabricGPUSharingPolicyReconciler) configureTimeSlicing(ctx context.Context, policy *gryviav1.FabricGPUSharingPolicy, nodes []gryviav1.FabricGpuNode) error {
	log := r.Log.WithValues("policy", policy.Name, "strategy", "time-slicing")

	if policy.Spec.TimeSlicing == nil || !policy.Spec.TimeSlicing.Enabled {
		return nil
	}

	for _, node := range nodes {
		// Label the node with time-slicing configuration
		k8sNode := &corev1.Node{}
		if err := r.Get(ctx, client.ObjectKey{Name: node.Spec.NodeName}, k8sNode); err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			log.Error(err, "Failed to get node", "node", node.Spec.NodeName)
			continue
		}

		if k8sNode.Labels == nil {
			k8sNode.Labels = make(map[string]string)
		}

		// Apply time-slicing labels
		k8sNode.Labels["gryvia.io/gpu-sharing"] = "time-slicing"
		k8sNode.Labels["gryvia.io/max-pods-per-gpu"] = fmt.Sprintf("%d", policy.Spec.TimeSlicing.MaxPodsPerGPU)

		if err := r.Update(ctx, k8sNode); err != nil {
			log.Error(err, "Failed to label node for time-slicing", "node", node.Spec.NodeName)
		}
	}

	return nil
}

func (r *FabricGPUSharingPolicyReconciler) configureMIG(ctx context.Context, policy *gryviav1.FabricGPUSharingPolicy, nodes []gryviav1.FabricGpuNode) error {
	log := r.Log.WithValues("policy", policy.Name, "strategy", "mig")

	if policy.Spec.MIG == nil || !policy.Spec.MIG.Enabled {
		return nil
	}

	for _, node := range nodes {
		k8sNode := &corev1.Node{}
		if err := r.Get(ctx, client.ObjectKey{Name: node.Spec.NodeName}, k8sNode); err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			log.Error(err, "Failed to get node", "node", node.Spec.NodeName)
			continue
		}

		if k8sNode.Labels == nil {
			k8sNode.Labels = make(map[string]string)
		}

		// Apply MIG labels
		k8sNode.Labels["gryvia.io/gpu-sharing"] = "mig"
		k8sNode.Labels["gryvia.io/mig-enabled"] = "true"

		// Encode MIG profiles in labels
		for _, profile := range policy.Spec.MIG.Profiles {
			labelKey := fmt.Sprintf("gryvia.io/mig-%s", profile.Name)
			k8sNode.Labels[labelKey] = fmt.Sprintf("%d", profile.Count)
		}

		if err := r.Update(ctx, k8sNode); err != nil {
			log.Error(err, "Failed to label node for MIG", "node", node.Spec.NodeName)
		}
	}

	return nil
}

func (r *FabricGPUSharingPolicyReconciler) configureFractional(ctx context.Context, policy *gryviav1.FabricGPUSharingPolicy, nodes []gryviav1.FabricGpuNode) error {
	log := r.Log.WithValues("policy", policy.Name, "strategy", "fractional")

	if policy.Spec.FractionalGPU == nil || !policy.Spec.FractionalGPU.Enabled {
		return nil
	}

	for _, node := range nodes {
		k8sNode := &corev1.Node{}
		if err := r.Get(ctx, client.ObjectKey{Name: node.Spec.NodeName}, k8sNode); err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			log.Error(err, "Failed to get node", "node", node.Spec.NodeName)
			continue
		}

		if k8sNode.Labels == nil {
			k8sNode.Labels = make(map[string]string)
		}

		// Apply fractional GPU labels
		k8sNode.Labels["gryvia.io/gpu-sharing"] = "fractional"
		k8sNode.Labels["gryvia.io/gpu-granularity"] = policy.Spec.FractionalGPU.Granularity

		if policy.Spec.FractionalGPU.Oversubscription != nil && policy.Spec.FractionalGPU.Oversubscription.Enabled {
			k8sNode.Labels["gryvia.io/gpu-oversubscription"] = "true"
			k8sNode.Labels["gryvia.io/gpu-oversubscription-ratio"] = fmt.Sprintf("%.1f", policy.Spec.FractionalGPU.Oversubscription.MaxRatio)
		}

		if err := r.Update(ctx, k8sNode); err != nil {
			log.Error(err, "Failed to label node for fractional GPU", "node", node.Spec.NodeName)
		}
	}

	return nil
}

func (r *FabricGPUSharingPolicyReconciler) trackPerPodUtilization(ctx context.Context, policy *gryviav1.FabricGPUSharingPolicy, nodes []gryviav1.FabricGpuNode) {
	var podUtils []gryviav1.GPUPodUtilization

	for _, node := range nodes {
		// List pods running on this node
		podList := &corev1.PodList{}
		if err := r.List(ctx, podList, client.MatchingFields{"spec.nodeName": node.Spec.NodeName}); err != nil {
			// If field selectors are not indexed, fall back to listing all pods
			// and filtering manually
			allPods := &corev1.PodList{}
			if err := r.List(ctx, allPods); err != nil {
				continue
			}
			for _, pod := range allPods.Items {
				if pod.Spec.NodeName == node.Spec.NodeName && pod.Status.Phase == corev1.PodRunning {
					// Check if this pod is using GPU resources
					for _, container := range pod.Spec.Containers {
						if gpuReq, ok := container.Resources.Requests["nvidia.com/gpu"]; ok {
							if gpuReq.Value() > 0 {
								podUtils = append(podUtils, gryviav1.GPUPodUtilization{
									PodName:   pod.Name,
									Namespace: pod.Namespace,
									NodeName:  node.Spec.NodeName,
								})
							}
						}
					}
				}
			}
			continue
		}

		for _, pod := range podList.Items {
			if pod.Status.Phase != corev1.PodRunning {
				continue
			}
			for _, container := range pod.Spec.Containers {
				if gpuReq, ok := container.Resources.Requests["nvidia.com/gpu"]; ok {
					if gpuReq.Value() > 0 {
						podUtils = append(podUtils, gryviav1.GPUPodUtilization{
							PodName:   pod.Name,
							Namespace: pod.Namespace,
							NodeName:  node.Spec.NodeName,
						})
					}
				}
			}
		}
	}

	// Only keep the most recent 100 entries
	if len(podUtils) > 100 {
		podUtils = podUtils[:100]
	}

	policy.Status.PerPodUtilization = podUtils
}

func (r *FabricGPUSharingPolicyReconciler) calculateAllocations(ctx context.Context, policy *gryviav1.FabricGPUSharingPolicy, nodes []gryviav1.FabricGpuNode) *gryviav1.GPUSharingAllocations {
	allocs := &gryviav1.GPUSharingAllocations{
		Physical: policy.Status.TotalGPUs,
	}

	// Count fractional allocations from per-pod tracking
	allocs.Fractional = len(policy.Status.PerPodUtilization)

	// Calculate utilization from GPU node status
	totalUtil := 0.0
	gpuCount := 0
	for _, node := range nodes {
		for _, gpuStatus := range node.Status.GpuStatus {
			totalUtil += float64(gpuStatus.Utilization) / 100.0
			gpuCount++
		}
	}
	if gpuCount > 0 {
		allocs.Utilization = totalUtil / float64(gpuCount)
	}

	return allocs
}

func (r *FabricGPUSharingPolicyReconciler) enforceFairSharing(ctx context.Context, policy *gryviav1.FabricGPUSharingPolicy, nodes []gryviav1.FabricGpuNode) {
	log := r.Log.WithValues("policy", policy.Name)

	// Check tenant quotas
	if len(policy.Spec.TenantQuotas) == 0 {
		return
	}

	// Count current allocations per tenant from pod labels
	tenantAllocations := make(map[string]int)
	for _, pu := range policy.Status.PerPodUtilization {
		// Look up the pod to find tenant label
		pod := &corev1.Pod{}
		if err := r.Get(ctx, client.ObjectKey{Name: pu.PodName, Namespace: pu.Namespace}, pod); err != nil {
			continue
		}
		tenant := pod.Labels["gryvia.io/tenant"]
		if tenant == "" {
			tenant = pod.Labels["gryvia.io/team"]
		}
		if tenant != "" {
			tenantAllocations[tenant]++
		}
	}

	// Log warnings for tenants exceeding their quotas
	for _, tq := range policy.Spec.TenantQuotas {
		current := tenantAllocations[tq.Tenant]
		if tq.MaxFractionalGPUs > 0 && current > tq.MaxFractionalGPUs {
			log.Info("Tenant exceeding GPU sharing quota",
				"tenant", tq.Tenant,
				"currentAllocations", current,
				"maxFractionalGPUs", tq.MaxFractionalGPUs,
			)
		}
	}
}

func (r *FabricGPUSharingPolicyReconciler) updateSharingCondition(policy *gryviav1.FabricGPUSharingPolicy, condType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: policy.Generation,
		LastTransitionTime: metav1.Now(),
	}

	found := false
	for i, cond := range policy.Status.Conditions {
		if cond.Type == condType {
			policy.Status.Conditions[i] = condition
			found = true
			break
		}
	}
	if !found {
		policy.Status.Conditions = append(policy.Status.Conditions, condition)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *FabricGPUSharingPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gryviav1.FabricGPUSharingPolicy{}).
		Complete(r)
}
