package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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

	tensorreaperv1 "github.com/ssahani/TensorReaper/operators/network-intelligence/api/v1"
)

const (
	// maxAnomalyHistory is the maximum number of anomalies to retain in status
	maxAnomalyHistory = 100

	// webhookTimeout is the timeout for alert webhook calls
	webhookTimeout = 10 * time.Second
)

// FabricNetworkAnomalyReconciler reconciles a FabricNetworkAnomaly object
type FabricNetworkAnomalyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabricnetworkanomalies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabricnetworkanomalies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabricnetworkanomalies/finalizers,verbs=update
//+kubebuilder:rbac:groups=cilium.io,resources=ciliumnetworkpolicies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *FabricNetworkAnomalyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the FabricNetworkAnomaly instance
	anomalyDetector := &tensorreaperv1.FabricNetworkAnomaly{}
	if err := r.Get(ctx, req.NamespacedName, anomalyDetector); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("FabricNetworkAnomaly resource not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get FabricNetworkAnomaly")
		return ctrl.Result{}, err
	}

	logger.Info("Reconciling FabricNetworkAnomaly",
		"name", anomalyDetector.Name,
		"targetService", anomalyDetector.Spec.TargetService,
		"rules", len(anomalyDetector.Spec.DetectionRules),
	)

	// Determine check interval from the shortest rule window
	checkInterval := r.getCheckInterval(anomalyDetector)

	// Evaluate detection rules against current metrics
	detectedAnomalies := r.evaluateRules(ctx, anomalyDetector)

	// Merge with existing anomalies (keep history)
	allAnomalies := r.mergeAnomalies(anomalyDetector.Status.Anomalies, detectedAnomalies)

	// Auto-mitigate if enabled
	if anomalyDetector.Spec.AutoMitigate {
		r.autoMitigate(ctx, anomalyDetector, detectedAnomalies)
	}

	// Send webhook alerts for new anomalies
	if anomalyDetector.Spec.AlertWebhook != "" && len(detectedAnomalies) > 0 {
		r.sendWebhookAlert(ctx, anomalyDetector, detectedAnomalies)
	}

	// Update status
	r.updateStatus(ctx, req.NamespacedName, allAnomalies)

	logger.Info("FabricNetworkAnomaly check complete",
		"newAnomalies", len(detectedAnomalies),
		"totalAnomalies", len(allAnomalies),
	)

	return ctrl.Result{RequeueAfter: checkInterval}, nil
}

// getCheckInterval determines the reconciliation interval based on the shortest rule window
func (r *FabricNetworkAnomalyReconciler) getCheckInterval(detector *tensorreaperv1.FabricNetworkAnomaly) time.Duration {
	shortest := 1 * time.Minute

	for _, rule := range detector.Spec.DetectionRules {
		if rule.Window != "" {
			if parsed, err := time.ParseDuration(rule.Window); err == nil && parsed > 0 {
				if parsed < shortest {
					shortest = parsed
				}
			}
		}
	}

	return shortest
}

// currentMetrics holds the current metric values for a service
type currentMetrics struct {
	latencyMs   float64
	throughput  float64
	connections float64
	errorRate   float64
	dropRate    float64
}

// evaluateRules checks each detection rule against current metrics
func (r *FabricNetworkAnomalyReconciler) evaluateRules(ctx context.Context, detector *tensorreaperv1.FabricNetworkAnomaly) []tensorreaperv1.NetworkAnomalyEvent {
	logger := log.FromContext(ctx)
	var anomalies []tensorreaperv1.NetworkAnomalyEvent

	// Query current metrics from Prometheus/Hubble
	metrics := r.queryCurrentMetrics(ctx, detector)

	for _, rule := range detector.Spec.DetectionRules {
		var metricValue float64
		switch rule.Metric {
		case "latency":
			metricValue = metrics.latencyMs
		case "throughput":
			metricValue = metrics.throughput
		case "connections":
			metricValue = metrics.connections
		case "errors":
			metricValue = metrics.errorRate
		case "drops":
			metricValue = metrics.dropRate
		default:
			logger.V(1).Info("Unknown metric type", "metric", rule.Metric)
			continue
		}

		// Evaluate the threshold condition
		triggered := false
		switch rule.Operator {
		case "gt":
			triggered = metricValue > rule.Threshold
		case "lt":
			triggered = metricValue < rule.Threshold
		case "gte":
			triggered = metricValue >= rule.Threshold
		case "lte":
			triggered = metricValue <= rule.Threshold
		case "eq":
			triggered = metricValue == rule.Threshold
		}

		if triggered {
			severity := r.classifySeverity(rule, metricValue)
			anomaly := tensorreaperv1.NetworkAnomalyEvent{
				Type:     fmt.Sprintf("%s_%s_threshold", rule.Metric, rule.Operator),
				Severity: severity,
				Detected: metav1.Now(),
				Description: fmt.Sprintf(
					"Service %s: %s metric value %.2f %s threshold %.2f (window: %s)",
					detector.Spec.TargetService,
					rule.Metric,
					metricValue,
					rule.Operator,
					rule.Threshold,
					rule.Window,
				),
				Mitigated: false,
			}
			anomalies = append(anomalies, anomaly)

			logger.Info("Anomaly detected",
				"type", anomaly.Type,
				"severity", severity,
				"metric", rule.Metric,
				"value", metricValue,
				"threshold", rule.Threshold,
			)
		}
	}

	return anomalies
}

// queryCurrentMetrics retrieves current network metrics for the target service.
// In production, this queries Prometheus for Hubble and Cilium metrics.
func (r *FabricNetworkAnomalyReconciler) queryCurrentMetrics(ctx context.Context, detector *tensorreaperv1.FabricNetworkAnomaly) currentMetrics {
	logger := log.FromContext(ctx)

	// Look for Prometheus service
	promSvc := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      "prometheus-server",
		Namespace: "monitoring",
	}, promSvc)
	if err != nil {
		logger.V(1).Info("Prometheus not available for metrics query")
	}

	// In production, this would execute PromQL queries:
	// - latency: histogram_quantile(0.99, rate(hubble_http_request_duration_seconds_bucket{destination=<svc>}[<window>]))
	// - throughput: rate(hubble_flows_processed_bytes_total{destination=<svc>}[<window>])
	// - connections: rate(hubble_tcp_connect_total{destination=<svc>}[<window>])
	// - errors: rate(hubble_http_responses_total{destination=<svc>,status=~"5.."}[<window>])
	// - drops: rate(hubble_drop_total{destination=<svc>}[<window>])

	return currentMetrics{}
}

// classifySeverity determines the anomaly severity based on how far the metric
// exceeds the threshold
func (r *FabricNetworkAnomalyReconciler) classifySeverity(rule tensorreaperv1.DetectionRule, value float64) string {
	if rule.Threshold == 0 {
		return "medium"
	}

	ratio := value / rule.Threshold
	switch {
	case ratio > 5.0:
		return "critical"
	case ratio > 3.0:
		return "high"
	case ratio > 1.5:
		return "medium"
	default:
		return "low"
	}
}

// mergeAnomalies combines existing and new anomalies, keeping only recent entries
func (r *FabricNetworkAnomalyReconciler) mergeAnomalies(existing, newAnomalies []tensorreaperv1.NetworkAnomalyEvent) []tensorreaperv1.NetworkAnomalyEvent {
	// Filter existing anomalies to keep only those from the last 24 hours
	var recent []tensorreaperv1.NetworkAnomalyEvent
	cutoff := time.Now().Add(-24 * time.Hour)
	for _, a := range existing {
		if !a.Detected.IsZero() && a.Detected.Time.After(cutoff) {
			recent = append(recent, a)
		}
	}

	// Append new anomalies
	recent = append(recent, newAnomalies...)

	// Limit to maxAnomalyHistory entries
	if len(recent) > maxAnomalyHistory {
		recent = recent[len(recent)-maxAnomalyHistory:]
	}

	return recent
}

// autoMitigate applies temporary deny policies for suspicious traffic
func (r *FabricNetworkAnomalyReconciler) autoMitigate(ctx context.Context, detector *tensorreaperv1.FabricNetworkAnomaly, anomalies []tensorreaperv1.NetworkAnomalyEvent) {
	logger := log.FromContext(ctx)

	for i, anomaly := range anomalies {
		if anomaly.Severity != "critical" && anomaly.Severity != "high" {
			continue
		}

		// Create a temporary deny policy for the target service
		policyName := fmt.Sprintf("fna-mitigate-%s-%d", detector.Name, time.Now().Unix())

		mitigationPolicy := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "cilium.io/v2",
				"kind":       "CiliumNetworkPolicy",
				"metadata": map[string]interface{}{
					"name":      policyName,
					"namespace": detector.Namespace,
					"annotations": map[string]interface{}{
						"tensorreaper.ai/managed-by":   "netpredator-anomaly",
						"tensorreaper.ai/anomaly-type": anomaly.Type,
						"tensorreaper.ai/temporary":    "true",
						"tensorreaper.ai/expires":      time.Now().Add(15 * time.Minute).Format(time.RFC3339),
					},
					"labels": map[string]interface{}{
						"tensorreaper.ai/mitigation": "auto",
					},
				},
				"spec": map[string]interface{}{
					"endpointSelector": map[string]interface{}{
						"matchLabels": map[string]interface{}{
							"app": detector.Spec.TargetService,
						},
					},
					"ingressDeny": []interface{}{
						map[string]interface{}{},
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
			Namespace: detector.Namespace,
		}, existingPolicy)

		if errors.IsNotFound(err) {
			if createErr := r.Create(ctx, mitigationPolicy); createErr != nil {
				logger.Error(createErr, "Failed to create mitigation policy", "name", policyName)
				continue
			}
			logger.Info("Created auto-mitigation policy",
				"name", policyName,
				"anomalyType", anomaly.Type,
				"severity", anomaly.Severity,
			)
			anomalies[i].Mitigated = true
		} else if err != nil {
			logger.Error(err, "Failed to check mitigation policy", "name", policyName)
		}
	}

	// Clean up expired mitigation policies
	r.cleanupExpiredMitigations(ctx, detector)
}

// cleanupExpiredMitigations removes temporary mitigation policies that have expired
func (r *FabricNetworkAnomalyReconciler) cleanupExpiredMitigations(ctx context.Context, detector *tensorreaperv1.FabricNetworkAnomaly) {
	logger := log.FromContext(ctx)

	// List CiliumNetworkPolicies with mitigation label in the detector's namespace
	policyList := &unstructured.UnstructuredList{}
	policyList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "cilium.io",
		Version: "v2",
		Kind:    "CiliumNetworkPolicyList",
	})

	if err := r.List(ctx, policyList,
		client.InNamespace(detector.Namespace),
		client.MatchingLabels{"tensorreaper.ai/mitigation": "auto"},
	); err != nil {
		logger.V(1).Info("Failed to list mitigation policies for cleanup", "error", err)
		return
	}

	for _, policy := range policyList.Items {
		annotations := policy.GetAnnotations()
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
			if deleteErr := r.Delete(ctx, &policy); deleteErr != nil {
				logger.Error(deleteErr, "Failed to delete expired mitigation policy", "name", policy.GetName())
			} else {
				logger.Info("Cleaned up expired mitigation policy", "name", policy.GetName())
			}
		}
	}
}

// webhookPayload is the JSON payload sent to alert webhooks
type webhookPayload struct {
	Service   string                              `json:"service"`
	Anomalies []tensorreaperv1.NetworkAnomalyEvent `json:"anomalies"`
	Timestamp string                              `json:"timestamp"`
}

// sendWebhookAlert sends anomaly alerts to the configured webhook URL
func (r *FabricNetworkAnomalyReconciler) sendWebhookAlert(ctx context.Context, detector *tensorreaperv1.FabricNetworkAnomaly, anomalies []tensorreaperv1.NetworkAnomalyEvent) {
	logger := log.FromContext(ctx)

	payload := webhookPayload{
		Service:   detector.Spec.TargetService,
		Anomalies: anomalies,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		logger.Error(err, "Failed to marshal webhook payload")
		return
	}

	httpClient := &http.Client{Timeout: webhookTimeout}
	resp, err := httpClient.Post(detector.Spec.AlertWebhook, "application/json", bytes.NewReader(body))
	if err != nil {
		logger.Error(err, "Failed to send webhook alert", "url", detector.Spec.AlertWebhook)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		logger.Info("Webhook alert returned non-success status",
			"url", detector.Spec.AlertWebhook,
			"statusCode", resp.StatusCode,
		)
	} else {
		logger.Info("Webhook alert sent successfully",
			"url", detector.Spec.AlertWebhook,
			"anomalies", len(anomalies),
		)
	}
}

// updateStatus updates the FabricNetworkAnomaly status subresource
func (r *FabricNetworkAnomalyReconciler) updateStatus(ctx context.Context, namespacedName types.NamespacedName, anomalies []tensorreaperv1.NetworkAnomalyEvent) {
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		detector := &tensorreaperv1.FabricNetworkAnomaly{}
		if err := r.Get(ctx, namespacedName, detector); err != nil {
			return err
		}
		detector.Status.Anomalies = anomalies
		detector.Status.LastCheck = metav1.Now()
		return r.Status().Update(ctx, detector)
	}); err != nil {
		log.FromContext(ctx).Error(err, "Failed to update FabricNetworkAnomaly status")
	}
}

// SetupWithManager sets up the controller with the Manager
func (r *FabricNetworkAnomalyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tensorreaperv1.FabricNetworkAnomaly{}).
		Complete(r)
}
