package controllers

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	gryviav1 "github.com/zyvorai/gryvia/operators/network-intelligence/api/v1"
)

// FabricFlowPolicyReconciler reconciles a FabricFlowPolicy object
type FabricFlowPolicyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=gryvia.io,resources=fabricflowpolicies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gryvia.io,resources=fabricflowpolicies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gryvia.io,resources=fabricflowpolicies/finalizers,verbs=update
//+kubebuilder:rbac:groups=cilium.io,resources=ciliumnetworkpolicies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *FabricFlowPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the FabricFlowPolicy instance
	policy := &gryviav1.FabricFlowPolicy{}
	if err := r.Get(ctx, req.NamespacedName, policy); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("FabricFlowPolicy resource not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get FabricFlowPolicy")
		return ctrl.Result{}, err
	}

	logger.Info("Reconciling FabricFlowPolicy",
		"name", policy.Name,
		"action", policy.Spec.Action,
		"intent", policy.Spec.Intent,
	)

	// Translate FabricFlowPolicy to CiliumNetworkPolicy
	ciliumPolicy, err := r.buildCiliumNetworkPolicy(policy)
	if err != nil {
		r.updateStatus(ctx, req.NamespacedName, "Failed", "", false, 0)
		return ctrl.Result{}, fmt.Errorf("failed to build CiliumNetworkPolicy: %w", err)
	}

	// Apply the CiliumNetworkPolicy
	ciliumPolicyName := fmt.Sprintf("ffp-%s", policy.Name)
	existingPolicy := &unstructured.Unstructured{}
	existingPolicy.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "cilium.io",
		Version: "v2",
		Kind:    "CiliumNetworkPolicy",
	})

	err = r.Get(ctx, types.NamespacedName{
		Name:      ciliumPolicyName,
		Namespace: policy.Namespace,
	}, existingPolicy)

	if errors.IsNotFound(err) {
		// Create new CiliumNetworkPolicy
		logger.Info("Creating CiliumNetworkPolicy", "name", ciliumPolicyName)
		if createErr := r.Create(ctx, ciliumPolicy); createErr != nil {
			r.updateStatus(ctx, req.NamespacedName, "Failed", ciliumPolicyName, false, 0)
			return ctrl.Result{}, fmt.Errorf("failed to create CiliumNetworkPolicy: %w", createErr)
		}
	} else if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to check existing CiliumNetworkPolicy: %w", err)
	} else {
		// Update existing CiliumNetworkPolicy
		logger.Info("Updating CiliumNetworkPolicy", "name", ciliumPolicyName)
		ciliumPolicy.SetResourceVersion(existingPolicy.GetResourceVersion())
		if updateErr := r.Update(ctx, ciliumPolicy); updateErr != nil {
			r.updateStatus(ctx, req.NamespacedName, "Failed", ciliumPolicyName, false, 0)
			return ctrl.Result{}, fmt.Errorf("failed to update CiliumNetworkPolicy: %w", updateErr)
		}
	}

	// Query flow count from Hubble (best-effort via service endpoint)
	matchedFlows := r.queryMatchedFlows(ctx, policy)

	// Update status to enforced
	r.updateStatus(ctx, req.NamespacedName, "Enforced", ciliumPolicyName, true, matchedFlows)

	logger.Info("FabricFlowPolicy reconciled successfully",
		"ciliumPolicy", ciliumPolicyName,
		"matchedFlows", matchedFlows,
	)

	return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
}

// buildCiliumNetworkPolicy translates a FabricFlowPolicy into an unstructured CiliumNetworkPolicy
func (r *FabricFlowPolicyReconciler) buildCiliumNetworkPolicy(policy *gryviav1.FabricFlowPolicy) (*unstructured.Unstructured, error) {
	ciliumPolicyName := fmt.Sprintf("ffp-%s", policy.Name)

	// Build endpoint selector from source labels
	endpointSelector := map[string]interface{}{}
	if policy.Spec.Source != nil && len(policy.Spec.Source.Labels) > 0 {
		matchLabels := make(map[string]interface{})
		for k, v := range policy.Spec.Source.Labels {
			matchLabels[k] = v
		}
		endpointSelector["matchLabels"] = matchLabels
	}

	// Build ingress/egress rules based on action
	spec := map[string]interface{}{
		"endpointSelector": endpointSelector,
	}

	// Build port rules
	var toPorts []interface{}
	if policy.Spec.Destination != nil && policy.Spec.Destination.Port > 0 {
		portRule := map[string]interface{}{
			"ports": []interface{}{
				map[string]interface{}{
					"port":     fmt.Sprintf("%d", policy.Spec.Destination.Port),
					"protocol": mapProtocol(policy.Spec.Protocol),
				},
			},
		}
		toPorts = append(toPorts, portRule)
	}

	// Build destination selector
	destSelector := map[string]interface{}{}
	if policy.Spec.Destination != nil {
		if len(policy.Spec.Destination.Labels) > 0 {
			matchLabels := make(map[string]interface{})
			for k, v := range policy.Spec.Destination.Labels {
				matchLabels[k] = v
			}
			destSelector["matchLabels"] = matchLabels
		}
	}

	switch policy.Spec.Action {
	case "allow":
		egressRule := map[string]interface{}{}
		if len(destSelector) > 0 {
			egressRule["toEndpoints"] = []interface{}{destSelector}
		}
		if len(toPorts) > 0 {
			egressRule["toPorts"] = toPorts
		}
		spec["egress"] = []interface{}{egressRule}
	case "deny":
		egressDenyRule := map[string]interface{}{}
		if len(destSelector) > 0 {
			egressDenyRule["toEndpoints"] = []interface{}{destSelector}
		}
		if len(toPorts) > 0 {
			egressDenyRule["toPorts"] = toPorts
		}
		spec["egressDeny"] = []interface{}{egressDenyRule}
	case "log":
		// For log action, create an allow rule with visibility annotations
		egressRule := map[string]interface{}{}
		if len(destSelector) > 0 {
			egressRule["toEndpoints"] = []interface{}{destSelector}
		}
		if len(toPorts) > 0 {
			egressRule["toPorts"] = toPorts
		}
		spec["egress"] = []interface{}{egressRule}
	}

	// Build annotations based on intent
	annotations := map[string]interface{}{
		"gryvia.io/managed-by": "netpredator",
		"gryvia.io/intent":     policy.Spec.Intent,
	}
	annotations = r.applyIntentAnnotations(annotations, policy.Spec.Intent)

	ciliumObj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "cilium.io/v2",
			"kind":       "CiliumNetworkPolicy",
			"metadata": map[string]interface{}{
				"name":        ciliumPolicyName,
				"namespace":   policy.Namespace,
				"annotations": annotations,
			},
			"spec": spec,
		},
	}

	return ciliumObj, nil
}

// applyIntentAnnotations maps intent types to Cilium-specific annotations
func (r *FabricFlowPolicyReconciler) applyIntentAnnotations(annotations map[string]interface{}, intent string) map[string]interface{} {
	switch intent {
	case "low-latency":
		annotations["cilium.io/priority"] = "high"
		annotations["gryvia.io/qos-class"] = "low-latency"
		annotations["gryvia.io/dscp"] = "46" // EF (Expedited Forwarding)
	case "high-throughput":
		annotations["cilium.io/priority"] = "normal"
		annotations["gryvia.io/qos-class"] = "high-throughput"
		annotations["gryvia.io/dscp"] = "34" // AF41
	case "secure":
		annotations["cilium.io/priority"] = "high"
		annotations["gryvia.io/qos-class"] = "secure"
		annotations["gryvia.io/encryption"] = "wireguard"
	default:
		annotations["gryvia.io/qos-class"] = "default"
	}
	return annotations
}

// mapProtocol converts the policy protocol to Cilium protocol format
func mapProtocol(protocol string) string {
	switch protocol {
	case "tcp":
		return "TCP"
	case "udp":
		return "UDP"
	case "icmp":
		return "ICMP"
	default:
		return "TCP"
	}
}

// queryMatchedFlows queries Hubble for the number of flows matching this policy.
// This is a best-effort operation; if Hubble is unavailable, returns 0.
func (r *FabricFlowPolicyReconciler) queryMatchedFlows(ctx context.Context, policy *gryviav1.FabricFlowPolicy) int64 {
	logger := log.FromContext(ctx)

	// Look for the Hubble relay service to query flow counts
	hubbleSvc := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      "hubble-relay",
		Namespace: "kube-system",
	}, hubbleSvc)
	if err != nil {
		logger.V(1).Info("Hubble relay service not found, skipping flow count query")
		return 0
	}

	// In a production implementation, this would connect to the Hubble gRPC API
	// and query for flows matching the policy's source/destination selectors.
	// For now, return the existing count from status to avoid resetting it.
	return policy.Status.MatchedFlows
}

// updateStatus updates the FabricFlowPolicy status subresource
func (r *FabricFlowPolicyReconciler) updateStatus(ctx context.Context, namespacedName types.NamespacedName, phase, ciliumPolicyRef string, enforced bool, matchedFlows int64) {
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		policy := &gryviav1.FabricFlowPolicy{}
		if err := r.Get(ctx, namespacedName, policy); err != nil {
			return err
		}
		policy.Status.Phase = phase
		policy.Status.CiliumPolicyRef = ciliumPolicyRef
		policy.Status.Enforced = enforced
		policy.Status.LastApplied = metav1.Now()
		policy.Status.MatchedFlows = matchedFlows
		return r.Status().Update(ctx, policy)
	}); err != nil {
		log.FromContext(ctx).Error(err, "Failed to update FabricFlowPolicy status")
	}
}

// SetupWithManager sets up the controller with the Manager
func (r *FabricFlowPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gryviav1.FabricFlowPolicy{}).
		Complete(r)
}
