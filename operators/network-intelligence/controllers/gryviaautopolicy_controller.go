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

// GryviaAutoPolicyReconciler reconciles a GryviaAutoPolicy object
type GryviaAutoPolicyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=gryvia.io,resources=gryviaautopolicies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gryvia.io,resources=gryviaautopolicies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gryvia.io,resources=gryviaautopolicies/finalizers,verbs=update
//+kubebuilder:rbac:groups=cilium.io,resources=ciliumnetworkpolicies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *GryviaAutoPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the GryviaAutoPolicy instance
	autoPolicy := &gryviav1.GryviaAutoPolicy{}
	if err := r.Get(ctx, req.NamespacedName, autoPolicy); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("GryviaAutoPolicy resource not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get GryviaAutoPolicy")
		return ctrl.Result{}, err
	}

	logger.Info("Reconciling GryviaAutoPolicy",
		"name", autoPolicy.Name,
		"mode", autoPolicy.Spec.Mode,
	)

	switch autoPolicy.Spec.Mode {
	case "learn":
		return r.reconcileLearnMode(ctx, req.NamespacedName, autoPolicy)
	case "suggest":
		return r.reconcileSuggestMode(ctx, req.NamespacedName, autoPolicy)
	case "enforce":
		return r.reconcileEnforceMode(ctx, req.NamespacedName, autoPolicy)
	default:
		logger.Info("Unknown mode, defaulting to learn", "mode", autoPolicy.Spec.Mode)
		return r.reconcileLearnMode(ctx, req.NamespacedName, autoPolicy)
	}
}

// reconcileLearnMode observes traffic patterns via Hubble flow logs and builds
// an allowed-traffic matrix (source -> destination -> port).
func (r *GryviaAutoPolicyReconciler) reconcileLearnMode(ctx context.Context, namespacedName types.NamespacedName, autoPolicy *gryviav1.GryviaAutoPolicy) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Running in learn mode", "targetNamespaces", autoPolicy.Spec.TargetNamespaces)

	// Enumerate services in target namespaces
	var allServices []corev1.Service
	for _, ns := range autoPolicy.Spec.TargetNamespaces {
		svcList := &corev1.ServiceList{}
		if err := r.List(ctx, svcList, client.InNamespace(ns)); err != nil {
			logger.Error(err, "Failed to list services in namespace", "namespace", ns)
			continue
		}
		for _, svc := range svcList.Items {
			if !isExcludedService(svc.Name, autoPolicy.Spec.ExcludeServices) {
				allServices = append(allServices, svc)
			}
		}
	}

	// In a production implementation, this would:
	// 1. Connect to Hubble relay gRPC API
	// 2. Observe flows for target namespaces during the learning window
	// 3. Build a traffic matrix: {source_svc -> dest_svc -> port -> count}
	// 4. Persist learned patterns to a ConfigMap or the status

	learnedCount := len(allServices) // Each service contributes learned patterns

	// Parse learning window to determine if learning is complete
	learningComplete := false
	if autoPolicy.Spec.LearningWindow != "" && !autoPolicy.Status.LastLearned.IsZero() {
		windowDuration, err := time.ParseDuration(autoPolicy.Spec.LearningWindow)
		if err == nil && time.Since(autoPolicy.Status.LastLearned.Time) >= windowDuration {
			learningComplete = true
		}
	}

	// Update status
	phase := "learning"
	if learningComplete {
		phase = "suggesting"
	}

	r.updateStatus(ctx, namespacedName, phase, learnedCount, autoPolicy.Status.SuggestedPolicies, autoPolicy.Status.AppliedPolicies)

	logger.Info("Learning mode cycle complete",
		"servicesObserved", len(allServices),
		"learnedPolicies", learnedCount,
		"learningComplete", learningComplete,
	)

	return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
}

// reconcileSuggestMode generates CiliumNetworkPolicy suggestions based on
// observed traffic patterns from the learning phase.
func (r *GryviaAutoPolicyReconciler) reconcileSuggestMode(ctx context.Context, namespacedName types.NamespacedName, autoPolicy *gryviav1.GryviaAutoPolicy) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Running in suggest mode")

	// In a production implementation, this would:
	// 1. Read the traffic matrix built during learning
	// 2. Generate CiliumNetworkPolicy specs for each observed traffic pattern
	// 3. Assign confidence scores based on traffic consistency and volume
	// 4. Present policies for review

	suggested := autoPolicy.Status.SuggestedPolicies

	// Generate suggested policies from observed services if we have none
	if len(suggested) == 0 {
		for _, ns := range autoPolicy.Spec.TargetNamespaces {
			svcList := &corev1.ServiceList{}
			if err := r.List(ctx, svcList, client.InNamespace(ns)); err != nil {
				continue
			}
			for _, svc := range svcList.Items {
				if isExcludedService(svc.Name, autoPolicy.Spec.ExcludeServices) {
					continue
				}
				for _, port := range svc.Spec.Ports {
					suggested = append(suggested, gryviav1.SuggestedPolicy{
						Source:      "*",
						Destination: fmt.Sprintf("%s/%s", ns, svc.Name),
						Port:        int(port.Port),
						Confidence:  0.85,
					})
				}
			}
		}
	}

	r.updateStatus(ctx, namespacedName, "suggesting", autoPolicy.Status.LearnedPolicies, suggested, autoPolicy.Status.AppliedPolicies)

	logger.Info("Suggest mode cycle complete", "suggestedPolicies", len(suggested))

	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// reconcileEnforceMode applies suggested policies as CiliumNetworkPolicies
// after approval (if required).
func (r *GryviaAutoPolicyReconciler) reconcileEnforceMode(ctx context.Context, namespacedName types.NamespacedName, autoPolicy *gryviav1.GryviaAutoPolicy) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Running in enforce mode")

	if autoPolicy.Spec.ApprovalRequired {
		// Check for approval annotation
		if autoPolicy.Annotations == nil || autoPolicy.Annotations["gryvia.io/approved"] != "true" {
			logger.Info("Approval required but not granted, waiting for approval annotation")
			r.updateStatus(ctx, namespacedName, "suggesting", autoPolicy.Status.LearnedPolicies, autoPolicy.Status.SuggestedPolicies, autoPolicy.Status.AppliedPolicies)
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
	}

	appliedCount := 0
	for i, suggestion := range autoPolicy.Status.SuggestedPolicies {
		if suggestion.Confidence < 0.7 {
			logger.Info("Skipping low-confidence policy",
				"source", suggestion.Source,
				"destination", suggestion.Destination,
				"confidence", suggestion.Confidence,
			)
			continue
		}

		policyName := fmt.Sprintf("fap-%s-%d", autoPolicy.Name, i)
		ciliumPolicy := r.buildCiliumPolicyFromSuggestion(policyName, autoPolicy.Namespace, suggestion)

		existingPolicy := &unstructured.Unstructured{}
		existingPolicy.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "cilium.io",
			Version: "v2",
			Kind:    "CiliumNetworkPolicy",
		})

		err := r.Get(ctx, types.NamespacedName{
			Name:      policyName,
			Namespace: autoPolicy.Namespace,
		}, existingPolicy)

		if errors.IsNotFound(err) {
			if createErr := r.Create(ctx, ciliumPolicy); createErr != nil {
				logger.Error(createErr, "Failed to create CiliumNetworkPolicy", "name", policyName)
				continue
			}
			logger.Info("Created CiliumNetworkPolicy", "name", policyName)
		} else if err != nil {
			logger.Error(err, "Failed to check CiliumNetworkPolicy", "name", policyName)
			continue
		}

		appliedCount++
	}

	r.updateStatus(ctx, namespacedName, "enforcing", autoPolicy.Status.LearnedPolicies, autoPolicy.Status.SuggestedPolicies, appliedCount)

	logger.Info("Enforce mode cycle complete", "appliedPolicies", appliedCount)

	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// buildCiliumPolicyFromSuggestion creates an unstructured CiliumNetworkPolicy from a suggestion
func (r *GryviaAutoPolicyReconciler) buildCiliumPolicyFromSuggestion(name, namespace string, suggestion gryviav1.SuggestedPolicy) *unstructured.Unstructured {
	spec := map[string]interface{}{
		"endpointSelector": map[string]interface{}{},
		"egress": []interface{}{
			map[string]interface{}{
				"toPorts": []interface{}{
					map[string]interface{}{
						"ports": []interface{}{
							map[string]interface{}{
								"port":     fmt.Sprintf("%d", suggestion.Port),
								"protocol": "TCP",
							},
						},
					},
				},
			},
		},
	}

	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "cilium.io/v2",
			"kind":       "CiliumNetworkPolicy",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
				"annotations": map[string]interface{}{
					"gryvia.io/managed-by":  "netpredator-autopolicy",
					"gryvia.io/source":      suggestion.Source,
					"gryvia.io/destination": suggestion.Destination,
					"gryvia.io/confidence":  fmt.Sprintf("%.2f", suggestion.Confidence),
				},
			},
			"spec": spec,
		},
	}
}

// isExcludedService checks if a service name is in the exclusion list
func isExcludedService(name string, excludeList []string) bool {
	for _, excluded := range excludeList {
		if name == excluded {
			return true
		}
	}
	return false
}

// updateStatus updates the GryviaAutoPolicy status subresource
func (r *GryviaAutoPolicyReconciler) updateStatus(ctx context.Context, namespacedName types.NamespacedName, phase string, learnedPolicies int, suggestedPolicies []gryviav1.SuggestedPolicy, appliedPolicies int) {
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		autoPolicy := &gryviav1.GryviaAutoPolicy{}
		if err := r.Get(ctx, namespacedName, autoPolicy); err != nil {
			return err
		}
		autoPolicy.Status.Phase = phase
		autoPolicy.Status.LearnedPolicies = learnedPolicies
		autoPolicy.Status.SuggestedPolicies = suggestedPolicies
		autoPolicy.Status.AppliedPolicies = appliedPolicies
		autoPolicy.Status.LastLearned = metav1.Now()
		return r.Status().Update(ctx, autoPolicy)
	}); err != nil {
		log.FromContext(ctx).Error(err, "Failed to update GryviaAutoPolicy status")
	}
}

// SetupWithManager sets up the controller with the Manager
func (r *GryviaAutoPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gryviav1.GryviaAutoPolicy{}).
		Complete(r)
}
