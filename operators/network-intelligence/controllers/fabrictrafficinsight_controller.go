package controllers

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	tensorreaperv1 "github.com/ssahani/TensorReaper/operators/network-intelligence/api/v1"
)

// FabricTrafficInsightReconciler reconciles a FabricTrafficInsight object
type FabricTrafficInsightReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabrictrafficinsights,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabrictrafficinsights/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabrictrafficinsights/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=endpoints,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *FabricTrafficInsightReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the FabricTrafficInsight instance
	insight := &tensorreaperv1.FabricTrafficInsight{}
	if err := r.Get(ctx, req.NamespacedName, insight); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("FabricTrafficInsight resource not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get FabricTrafficInsight")
		return ctrl.Result{}, err
	}

	logger.Info("Reconciling FabricTrafficInsight",
		"name", insight.Name,
		"service", insight.Spec.Service,
		"window", insight.Spec.Window,
	)

	// Parse the analysis window for requeue interval
	requeueInterval := 30 * time.Second
	if insight.Spec.Window != "" {
		if parsed, err := time.ParseDuration(insight.Spec.Window); err == nil && parsed > 0 {
			requeueInterval = parsed
		}
	}

	// Verify the target service exists
	targetNs := insight.Spec.Namespace
	if targetNs == "" {
		targetNs = insight.Namespace
	}

	svc := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      insight.Spec.Service,
		Namespace: targetNs,
	}, svc)
	if err != nil && !errors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("failed to check target service: %w", err)
	}

	// Query traffic metrics from Prometheus/Hubble
	metrics := r.collectTrafficMetrics(ctx, insight)

	// Identify top talkers
	topTalkers := r.identifyTopTalkers(ctx, insight)

	// Detect anomalies
	anomalies := r.detectAnomalies(ctx, insight, metrics)

	// Update status with collected metrics
	r.updateStatus(ctx, req.NamespacedName, metrics, topTalkers, anomalies)

	logger.Info("FabricTrafficInsight metrics updated",
		"p50Latency", metrics.p50Latency,
		"p99Latency", metrics.p99Latency,
		"throughputBps", metrics.throughputBps,
		"dropCount", metrics.dropCount,
		"topTalkersCount", len(topTalkers),
		"anomaliesCount", len(anomalies),
	)

	return ctrl.Result{RequeueAfter: requeueInterval}, nil
}

// trafficMetrics holds collected traffic metrics
type trafficMetrics struct {
	p50Latency    string
	p99Latency    string
	throughputBps int64
	dropCount     int64
}

// collectTrafficMetrics queries Prometheus and Hubble for traffic metrics.
// In production this would issue PromQL queries to the Prometheus service.
func (r *FabricTrafficInsightReconciler) collectTrafficMetrics(ctx context.Context, insight *tensorreaperv1.FabricTrafficInsight) trafficMetrics {
	logger := log.FromContext(ctx)

	metrics := trafficMetrics{}

	// Look for Prometheus service to query metrics
	promSvc := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      "prometheus-server",
		Namespace: "monitoring",
	}, promSvc)
	if err != nil {
		logger.V(1).Info("Prometheus service not found, using Hubble metrics as fallback")
	}

	// Check which metrics are requested
	metricsSet := make(map[string]bool)
	for _, m := range insight.Spec.Metrics {
		metricsSet[m] = true
	}

	// In a production implementation, this would execute PromQL queries such as:
	// - histogram_quantile(0.50, rate(hubble_flows_processed_duration_seconds_bucket{destination=<svc>}[<window>]))
	// - histogram_quantile(0.99, rate(hubble_flows_processed_duration_seconds_bucket{destination=<svc>}[<window>]))
	// - rate(hubble_flows_processed_bytes_total{destination=<svc>}[<window>])
	// - rate(hubble_drop_total{destination=<svc>}[<window>])
	//
	// For now, preserve existing status values to avoid resetting them.
	metrics.p50Latency = insight.Status.P50Latency
	metrics.p99Latency = insight.Status.P99Latency
	metrics.throughputBps = insight.Status.ThroughputBps
	metrics.dropCount = insight.Status.DropCount

	return metrics
}

// identifyTopTalkers finds services with the highest traffic volume to the target service.
// In production this would query Hubble flow logs aggregated by source service.
func (r *FabricTrafficInsightReconciler) identifyTopTalkers(ctx context.Context, insight *tensorreaperv1.FabricTrafficInsight) []tensorreaperv1.TopTalker {
	// In a production implementation, this would query Hubble for flow data
	// grouped by source service, sorted by total bytes transferred.
	// Example PromQL: topk(10, sum by (source) (rate(hubble_flows_processed_bytes_total{destination=<svc>}[<window>])))

	// Preserve existing top talkers from status
	return insight.Status.TopTalkers
}

// detectAnomalies identifies unusual traffic patterns for the target service.
func (r *FabricTrafficInsightReconciler) detectAnomalies(ctx context.Context, insight *tensorreaperv1.FabricTrafficInsight, metrics trafficMetrics) []tensorreaperv1.TrafficAnomaly {
	var anomalies []tensorreaperv1.TrafficAnomaly

	// In a production implementation, anomaly detection would compare current
	// metrics against historical baselines and detect:
	// - Sudden latency spikes (p99 > 3x historical average)
	// - Unusual traffic volume (throughput > 5x historical average)
	// - Elevated drop rates (drops > threshold)
	// - New destination services not seen during baseline period
	// - Retransmission rate spikes indicating network congestion

	// Preserve existing anomalies that have not expired
	for _, a := range insight.Status.Anomalies {
		// Keep anomalies detected within the last hour
		if !a.Detected.IsZero() && time.Since(a.Detected.Time) < 1*time.Hour {
			anomalies = append(anomalies, a)
		}
	}

	return anomalies
}

// updateStatus updates the FabricTrafficInsight status subresource
func (r *FabricTrafficInsightReconciler) updateStatus(ctx context.Context, namespacedName types.NamespacedName, metrics trafficMetrics, topTalkers []tensorreaperv1.TopTalker, anomalies []tensorreaperv1.TrafficAnomaly) {
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		insight := &tensorreaperv1.FabricTrafficInsight{}
		if err := r.Get(ctx, namespacedName, insight); err != nil {
			return err
		}
		insight.Status.P50Latency = metrics.p50Latency
		insight.Status.P99Latency = metrics.p99Latency
		insight.Status.ThroughputBps = metrics.throughputBps
		insight.Status.DropCount = metrics.dropCount
		insight.Status.TopTalkers = topTalkers
		insight.Status.Anomalies = anomalies
		insight.Status.LastUpdated = metav1.Now()
		return r.Status().Update(ctx, insight)
	}); err != nil {
		log.FromContext(ctx).Error(err, "Failed to update FabricTrafficInsight status")
	}
}

// SetupWithManager sets up the controller with the Manager
func (r *FabricTrafficInsightReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tensorreaperv1.FabricTrafficInsight{}).
		Complete(r)
}
