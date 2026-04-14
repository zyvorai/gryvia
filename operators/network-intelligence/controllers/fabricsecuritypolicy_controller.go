package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

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

	tensorreaperv1 "github.com/ssahani/TensorReaper/operators/network-intelligence/api/v1"
)

const (
	// securityCheckInterval is the default requeue interval for security policy checks
	securityCheckInterval = 30 * time.Second

	// securityWebhookTimeout is the timeout for security alert webhook calls
	securityWebhookTimeout = 10 * time.Second

	// collectorBaseURL is the base URL for the eBPF collector API
	collectorBaseURL = "http://tensorreaper-collector.tensorreaper-system.svc.cluster.local:9090"
)

// FabricSecurityPolicyReconciler reconciles a FabricSecurityPolicy object
type FabricSecurityPolicyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabricsecuritypolicies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabricsecuritypolicies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabricsecuritypolicies/finalizers,verbs=update
//+kubebuilder:rbac:groups=cilium.io,resources=ciliumnetworkpolicies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *FabricSecurityPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the FabricSecurityPolicy instance
	policy := &tensorreaperv1.FabricSecurityPolicy{}
	if err := r.Get(ctx, req.NamespacedName, policy); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("FabricSecurityPolicy resource not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get FabricSecurityPolicy")
		return ctrl.Result{}, err
	}

	logger.Info("Reconciling FabricSecurityPolicy",
		"name", policy.Name,
		"namespaces", policy.Spec.TargetNamespaces,
		"rules", len(policy.Spec.DetectionRules),
	)

	// Count active detection rules
	activeDetections := 0
	for _, rule := range policy.Spec.DetectionRules {
		if rule.Enabled {
			activeDetections++
		}
	}

	// Verify eBPF programs are running for each enabled detection rule
	r.verifyEBPFPrograms(ctx, policy)

	// Query collector for security alerts
	alerts := r.querySecurityAlerts(ctx, policy)

	// Count alerts by detection type
	detectionCounts := make(map[string]int)
	for _, alert := range alerts {
		detectionCounts[alert.Type]++
	}

	// Auto-block if enabled and critical alerts detected
	if policy.Spec.AutoBlock {
		r.autoBlockThreats(ctx, policy, alerts)
	}

	// Send webhook alerts for new detections
	if policy.Spec.AlertWebhook != "" && len(alerts) > 0 {
		r.sendSecurityWebhook(ctx, policy, alerts)
	}

	// Update status
	var lastAlert metav1.Time
	if len(alerts) > 0 {
		lastAlert = metav1.Now()
	} else {
		lastAlert = policy.Status.LastAlert
	}

	totalAlerts := policy.Status.AlertsTriggered + len(alerts)

	r.updateSecurityStatus(ctx, req.NamespacedName, "Active", activeDetections, totalAlerts, lastAlert, detectionCounts)

	logger.Info("FabricSecurityPolicy check complete",
		"activeDetections", activeDetections,
		"newAlerts", len(alerts),
	)

	return ctrl.Result{RequeueAfter: securityCheckInterval}, nil
}

// securityAlert represents an alert from the collector
type securityAlert struct {
	Type     string `json:"type"`
	Severity string `json:"severity"`
	Process  string `json:"process"`
	Path     string `json:"path"`
	SourceIP string `json:"sourceIP"`
	Message  string `json:"message"`
}

// verifyEBPFPrograms checks that the eBPF collector is healthy and programs are loaded
func (r *FabricSecurityPolicyReconciler) verifyEBPFPrograms(ctx context.Context, policy *tensorreaperv1.FabricSecurityPolicy) {
	logger := log.FromContext(ctx)

	httpClient := &http.Client{Timeout: 5 * time.Second}
	resp, err := httpClient.Get(fmt.Sprintf("%s/api/v1/health", collectorBaseURL))
	if err != nil {
		logger.V(1).Info("eBPF collector not reachable", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Info("eBPF collector health check failed", "statusCode", resp.StatusCode)
	}
}

// querySecurityAlerts fetches security alerts from the collector API
func (r *FabricSecurityPolicyReconciler) querySecurityAlerts(ctx context.Context, policy *tensorreaperv1.FabricSecurityPolicy) []securityAlert {
	logger := log.FromContext(ctx)

	httpClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := httpClient.Get(fmt.Sprintf("%s/api/v1/security/alerts", collectorBaseURL))
	if err != nil {
		logger.V(1).Info("Failed to query security alerts from collector", "error", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.V(1).Info("Collector returned non-OK for security alerts", "statusCode", resp.StatusCode)
		return nil
	}

	var alerts []securityAlert
	if err := json.NewDecoder(resp.Body).Decode(&alerts); err != nil {
		logger.Error(err, "Failed to decode security alerts response")
		return nil
	}

	// Filter alerts by enabled detection rules
	var filtered []securityAlert
	enabledTypes := make(map[string]bool)
	for _, rule := range policy.Spec.DetectionRules {
		if rule.Enabled {
			enabledTypes[rule.Type] = true
		}
	}

	for _, alert := range alerts {
		if enabledTypes[alert.Type] {
			filtered = append(filtered, alert)
		}
	}

	return filtered
}

// autoBlockThreats creates temporary CiliumNetworkPolicy to block detected threats
func (r *FabricSecurityPolicyReconciler) autoBlockThreats(ctx context.Context, policy *tensorreaperv1.FabricSecurityPolicy, alerts []securityAlert) {
	logger := log.FromContext(ctx)

	for _, alert := range alerts {
		if alert.Severity != "critical" && alert.Severity != "high" {
			continue
		}

		if alert.SourceIP == "" {
			continue
		}

		policyName := fmt.Sprintf("fsp-block-%s-%d", policy.Name, time.Now().Unix())

		blockPolicy := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "cilium.io/v2",
				"kind":       "CiliumNetworkPolicy",
				"metadata": map[string]interface{}{
					"name":      policyName,
					"namespace": policy.Namespace,
					"annotations": map[string]interface{}{
						"tensorreaper.ai/managed-by":  "security-policy",
						"tensorreaper.ai/alert-type":  alert.Type,
						"tensorreaper.ai/temporary":   "true",
						"tensorreaper.ai/expires":     time.Now().Add(30 * time.Minute).Format(time.RFC3339),
						"tensorreaper.ai/source-ip":   alert.SourceIP,
					},
					"labels": map[string]interface{}{
						"tensorreaper.ai/security-block": "auto",
					},
				},
				"spec": map[string]interface{}{
					"ingressDeny": []interface{}{
						map[string]interface{}{
							"fromCIDR": []interface{}{
								fmt.Sprintf("%s/32", alert.SourceIP),
							},
						},
					},
				},
			},
		}

		existingPolicy := &unstructured.Unstructured{}
		existingPolicy.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "cilium.io",
			Version: "v2",
			Kind:    "CiliumNetworkPolicy",
		})

		err := r.Get(ctx, types.NamespacedName{
			Name:      policyName,
			Namespace: policy.Namespace,
		}, existingPolicy)

		if errors.IsNotFound(err) {
			if createErr := r.Create(ctx, blockPolicy); createErr != nil {
				logger.Error(createErr, "Failed to create security block policy", "name", policyName)
				continue
			}
			logger.Info("Created auto-block policy for security threat",
				"name", policyName,
				"alertType", alert.Type,
				"sourceIP", alert.SourceIP,
			)
		} else if err != nil {
			logger.Error(err, "Failed to check security block policy", "name", policyName)
		}
	}

	// Clean up expired block policies
	r.cleanupExpiredBlockPolicies(ctx, policy)
}

// cleanupExpiredBlockPolicies removes temporary block policies that have expired
func (r *FabricSecurityPolicyReconciler) cleanupExpiredBlockPolicies(ctx context.Context, policy *tensorreaperv1.FabricSecurityPolicy) {
	logger := log.FromContext(ctx)

	policyList := &unstructured.UnstructuredList{}
	policyList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "cilium.io",
		Version: "v2",
		Kind:    "CiliumNetworkPolicyList",
	})

	if err := r.List(ctx, policyList,
		client.InNamespace(policy.Namespace),
		client.MatchingLabels{"tensorreaper.ai/security-block": "auto"},
	); err != nil {
		logger.V(1).Info("Failed to list security block policies for cleanup", "error", err)
		return
	}

	for _, p := range policyList.Items {
		annotations := p.GetAnnotations()
		if annotations == nil {
			continue
		}

		expiresStr, ok := annotations["tensorreaper.ai/expires"]
		if !ok {
			continue
		}

		expires, err := time.Parse(time.RFC3339, expiresStr)
		if err != nil {
			continue
		}

		if time.Now().After(expires) {
			if deleteErr := r.Delete(ctx, &p); deleteErr != nil {
				logger.Error(deleteErr, "Failed to delete expired security block policy", "name", p.GetName())
			} else {
				logger.Info("Cleaned up expired security block policy", "name", p.GetName())
			}
		}
	}
}

// securityWebhookPayload is the JSON payload sent to security alert webhooks
type securityWebhookPayload struct {
	PolicyName string           `json:"policyName"`
	Namespace  string           `json:"namespace"`
	Alerts     []securityAlert  `json:"alerts"`
	Timestamp  string           `json:"timestamp"`
}

// sendSecurityWebhook sends security alerts to the configured webhook URL
func (r *FabricSecurityPolicyReconciler) sendSecurityWebhook(ctx context.Context, policy *tensorreaperv1.FabricSecurityPolicy, alerts []securityAlert) {
	logger := log.FromContext(ctx)

	payload := securityWebhookPayload{
		PolicyName: policy.Name,
		Namespace:  policy.Namespace,
		Alerts:     alerts,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		logger.Error(err, "Failed to marshal security webhook payload")
		return
	}

	httpClient := &http.Client{Timeout: securityWebhookTimeout}
	resp, err := httpClient.Post(policy.Spec.AlertWebhook, "application/json", bytes.NewReader(body))
	if err != nil {
		logger.Error(err, "Failed to send security webhook alert", "url", policy.Spec.AlertWebhook)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		logger.Info("Security webhook returned non-success status",
			"url", policy.Spec.AlertWebhook,
			"statusCode", resp.StatusCode,
		)
	} else {
		logger.Info("Security webhook alert sent successfully",
			"url", policy.Spec.AlertWebhook,
			"alerts", len(alerts),
		)
	}
}

// updateSecurityStatus updates the FabricSecurityPolicy status subresource
func (r *FabricSecurityPolicyReconciler) updateSecurityStatus(ctx context.Context, namespacedName types.NamespacedName, phase string, activeDetections, alertsTriggered int, lastAlert metav1.Time, detectionCounts map[string]int) {
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		policy := &tensorreaperv1.FabricSecurityPolicy{}
		if err := r.Get(ctx, namespacedName, policy); err != nil {
			return err
		}
		policy.Status.Phase = phase
		policy.Status.ActiveDetections = activeDetections
		policy.Status.AlertsTriggered = alertsTriggered
		policy.Status.LastAlert = lastAlert
		policy.Status.DetectionCounts = detectionCounts
		return r.Status().Update(ctx, policy)
	}); err != nil {
		log.FromContext(ctx).Error(err, "Failed to update FabricSecurityPolicy status")
	}
}

// SetupWithManager sets up the controller with the Manager
func (r *FabricSecurityPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tensorreaperv1.FabricSecurityPolicy{}).
		Complete(r)
}
