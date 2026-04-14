package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	tensorreaperv1 "github.com/ssahani/TensorReaper/operators/gpu-operator/api/v1"
)

const (
	metricStateNormal   = "normal"
	metricStateWarning  = "warning"
	metricStateCritical = "critical"

	maxHistoryEntries = 100
)

// FabricMetricReconciler reconciles a FabricMetric object
type FabricMetricReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabricmetrics,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabricmetrics/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabricmetrics/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop
func (r *FabricMetricReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("fabricmetric", req.NamespacedName)

	// Fetch the FabricMetric instance
	metric := &tensorreaperv1.FabricMetric{}
	err := r.Get(ctx, req.NamespacedName, metric)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("FabricMetric resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get FabricMetric")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !metric.ObjectMeta.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	// Initialize state
	if metric.Status.State == "" {
		metric.Status.State = metricStateNormal
		if err := r.Status().Update(ctx, metric); err != nil {
			log.Error(err, "Failed to update initial status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Reconcile metric collection
	result, err := r.reconcileMetric(ctx, metric)
	if err != nil {
		log.Error(err, "Failed to reconcile metric")
		return result, err
	}

	return result, nil
}

func (r *FabricMetricReconciler) reconcileMetric(ctx context.Context, metric *tensorreaperv1.FabricMetric) (ctrl.Result, error) {
	log := r.Log.WithValues("fabricmetric", metric.Name)

	// Collect metric value based on source type
	value, err := r.collectMetricValue(ctx, metric)
	if err != nil {
		log.Error(err, "Failed to collect metric value")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	// Update current value
	now := metav1.Now()
	metric.Status.CurrentValue = value
	metric.Status.LastUpdate = &now

	// Add to history
	entry := tensorreaperv1.MetricHistoryEntry{
		Timestamp: &now,
		Value:     value,
	}
	metric.Status.History = append(metric.Status.History, entry)

	// Trim history to max entries
	if len(metric.Status.History) > maxHistoryEntries {
		metric.Status.History = metric.Status.History[len(metric.Status.History)-maxHistoryEntries:]
	}

	// Evaluate thresholds
	metric.Status.State = r.evaluateThresholds(metric, value)

	// Log threshold breaches
	if metric.Status.State != metricStateNormal {
		log.Info("Metric threshold breached",
			"metric", metric.Spec.Name,
			"value", value,
			"state", metric.Status.State,
		)
	}

	// Update status
	if err := r.Status().Update(ctx, metric); err != nil {
		log.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: r.getCollectionInterval(metric)}, nil
}

func (r *FabricMetricReconciler) collectMetricValue(ctx context.Context, metric *tensorreaperv1.FabricMetric) (float64, error) {
	switch metric.Spec.Source.Type {
	case "prometheus":
		return r.collectFromPrometheus(ctx, metric)
	case "webhook":
		return r.collectFromWebhook(ctx, metric)
	case "job-output":
		return r.collectFromJobOutput(ctx, metric)
	case "script":
		return r.collectFromScript(ctx, metric)
	default:
		return 0, fmt.Errorf("unsupported metric source type: %s", metric.Spec.Source.Type)
	}
}

func (r *FabricMetricReconciler) collectFromPrometheus(ctx context.Context, metric *tensorreaperv1.FabricMetric) (float64, error) {
	if metric.Spec.Source.Prometheus == nil {
		return 0, fmt.Errorf("prometheus source configuration is missing")
	}

	promQuery := metric.Spec.Source.Prometheus.Query
	if promQuery == "" {
		return 0, fmt.Errorf("prometheus query is empty")
	}

	// Query Prometheus API
	// In production, the Prometheus URL would come from configuration
	promURL := "http://prometheus:9090/api/v1/query"
	req, err := http.NewRequestWithContext(ctx, "GET", promURL, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create prometheus request: %w", err)
	}

	q := req.URL.Query()
	q.Add("query", promQuery)
	req.URL.RawQuery = q.Encode()

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("prometheus query failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("prometheus returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read prometheus response: %w", err)
	}

	// Parse Prometheus response
	var promResp struct {
		Status string `json:"status"`
		Data   struct {
			ResultType string `json:"resultType"`
			Result     []struct {
				Value []interface{} `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &promResp); err != nil {
		return 0, fmt.Errorf("failed to parse prometheus response: %w", err)
	}

	if promResp.Status != "success" {
		return 0, fmt.Errorf("prometheus query returned status: %s", promResp.Status)
	}

	if len(promResp.Data.Result) == 0 {
		return 0, fmt.Errorf("prometheus query returned no results")
	}

	// Extract the value (second element in the value array)
	if len(promResp.Data.Result[0].Value) < 2 {
		return 0, fmt.Errorf("unexpected prometheus result format")
	}

	valueStr, ok := promResp.Data.Result[0].Value[1].(string)
	if !ok {
		return 0, fmt.Errorf("unexpected value type in prometheus result")
	}

	var value float64
	if _, err := fmt.Sscanf(valueStr, "%f", &value); err != nil {
		return 0, fmt.Errorf("failed to parse metric value %q: %w", valueStr, err)
	}

	return value, nil
}

func (r *FabricMetricReconciler) collectFromWebhook(ctx context.Context, metric *tensorreaperv1.FabricMetric) (float64, error) {
	if metric.Spec.Source.Webhook == nil {
		return 0, fmt.Errorf("webhook source configuration is missing")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", metric.Spec.Source.Webhook.URL, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create webhook request: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("webhook request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read webhook response: %w", err)
	}

	var result struct {
		Value float64 `json:"value"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("failed to parse webhook response: %w", err)
	}

	return result.Value, nil
}

func (r *FabricMetricReconciler) collectFromJobOutput(_ context.Context, metric *tensorreaperv1.FabricMetric) (float64, error) {
	if metric.Spec.Source.JobOutput == nil {
		return 0, fmt.Errorf("job-output source configuration is missing")
	}

	// In a real implementation, this would:
	// 1. Find the latest completed job with matching labels
	// 2. Read the job's output file or parse its logs
	// 3. Extract the metric value using the configured pattern or JSONPath
	r.Log.Info("Job output metric collection requires runtime log access",
		"metric", metric.Spec.Name,
		"pattern", metric.Spec.Source.JobOutput.Pattern,
		"file", metric.Spec.Source.JobOutput.File,
	)

	return 0, nil
}

func (r *FabricMetricReconciler) collectFromScript(_ context.Context, metric *tensorreaperv1.FabricMetric) (float64, error) {
	if metric.Spec.Source.Script == nil {
		return 0, fmt.Errorf("script source configuration is missing")
	}

	// In a real implementation, this would run the script command
	// and parse its stdout for the metric value
	r.Log.Info("Script metric collection requires node agent",
		"metric", metric.Spec.Name,
		"command", metric.Spec.Source.Script.Command,
	)

	return 0, nil
}

func (r *FabricMetricReconciler) evaluateThresholds(metric *tensorreaperv1.FabricMetric, value float64) string {
	if metric.Spec.Thresholds == nil {
		return metricStateNormal
	}

	// Check critical thresholds first
	if metric.Spec.Thresholds.Critical != nil {
		if metric.Spec.Thresholds.Critical.Min != nil && value < *metric.Spec.Thresholds.Critical.Min {
			return metricStateCritical
		}
		if metric.Spec.Thresholds.Critical.Max != nil && value > *metric.Spec.Thresholds.Critical.Max {
			return metricStateCritical
		}
	}

	// Check warning thresholds
	if metric.Spec.Thresholds.Warning != nil {
		if metric.Spec.Thresholds.Warning.Min != nil && value < *metric.Spec.Thresholds.Warning.Min {
			return metricStateWarning
		}
		if metric.Spec.Thresholds.Warning.Max != nil && value > *metric.Spec.Thresholds.Warning.Max {
			return metricStateWarning
		}
	}

	return metricStateNormal
}

func (r *FabricMetricReconciler) getCollectionInterval(metric *tensorreaperv1.FabricMetric) time.Duration {
	// Default interval is 1 minute
	interval := 1 * time.Minute

	// Try to get interval from the source config
	switch metric.Spec.Source.Type {
	case "prometheus":
		if metric.Spec.Source.Prometheus != nil && metric.Spec.Source.Prometheus.Interval != "" {
			if parsed, err := time.ParseDuration(metric.Spec.Source.Prometheus.Interval); err == nil {
				interval = parsed
			}
		}
	case "webhook":
		if metric.Spec.Source.Webhook != nil && metric.Spec.Source.Webhook.Interval != "" {
			if parsed, err := time.ParseDuration(metric.Spec.Source.Webhook.Interval); err == nil {
				interval = parsed
			}
		}
	case "script":
		if metric.Spec.Source.Script != nil && metric.Spec.Source.Script.Interval != "" {
			if parsed, err := time.ParseDuration(metric.Spec.Source.Script.Interval); err == nil {
				interval = parsed
			}
		}
	}

	return interval
}

// SetupWithManager sets up the controller with the Manager.
func (r *FabricMetricReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tensorreaperv1.FabricMetric{}).
		Complete(r)
}
