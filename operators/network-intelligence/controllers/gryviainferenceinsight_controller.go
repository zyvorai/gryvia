package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	gryviav1 "github.com/zyvorai/gryvia/operators/network-intelligence/api/v1"
)

const (
	// inferenceInsightInterval is the default requeue interval for inference analysis
	inferenceInsightInterval = 60 * time.Second
)

// FabricInferenceInsightReconciler reconciles a FabricInferenceInsight object
type FabricInferenceInsightReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=gryvia.io,resources=fabricinferenceinsights,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gryvia.io,resources=fabricinferenceinsights/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gryvia.io,resources=fabricinferenceinsights/finalizers,verbs=update

func (r *FabricInferenceInsightReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the FabricInferenceInsight instance
	insight := &gryviav1.FabricInferenceInsight{}
	if err := r.Get(ctx, req.NamespacedName, insight); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("FabricInferenceInsight resource not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get FabricInferenceInsight")
		return ctrl.Result{}, err
	}

	logger.Info("Reconciling FabricInferenceInsight",
		"name", insight.Name,
		"targetService", insight.Spec.TargetService,
	)

	// Query collector for latency breakdown data
	latencyData := r.queryLatencyBreakdown(ctx, insight)

	// Populate latency breakdown
	breakdown := gryviav1.LatencyBreakdown{
		DNSNs:          latencyData.DNSNs,
		TCPConnectNs:   latencyData.TCPConnectNs,
		TLSHandshakeNs: latencyData.TLSHandshakeNs,
		GPUQueueNs:     latencyData.GPUQueueNs,
		GPUExecNs:      latencyData.GPUExecNs,
		PostprocessNs:  latencyData.PostprocessNs,
		TotalNs:        latencyData.TotalNs,
	}

	// Identify bottleneck phase
	bottleneck := r.identifyLatencyBottleneck(breakdown)

	// Update status
	r.updateInferenceInsightStatus(ctx, req.NamespacedName, "Active", breakdown,
		latencyData.P50TotalNs, latencyData.P95TotalNs, latencyData.P99TotalNs, bottleneck)

	logger.Info("FabricInferenceInsight analysis complete",
		"bottleneck", bottleneck,
		"p50Ns", latencyData.P50TotalNs,
		"p99Ns", latencyData.P99TotalNs,
	)

	return ctrl.Result{RequeueAfter: inferenceInsightInterval}, nil
}

// collectorLatencyResponse represents the response from the collector latency API
type collectorLatencyResponse struct {
	DNSNs          int64 `json:"dnsNs"`
	TCPConnectNs   int64 `json:"tcpConnectNs"`
	TLSHandshakeNs int64 `json:"tlsHandshakeNs"`
	GPUQueueNs     int64 `json:"gpuQueueNs"`
	GPUExecNs      int64 `json:"gpuExecNs"`
	PostprocessNs  int64 `json:"postprocessNs"`
	TotalNs        int64 `json:"totalNs"`
	P50TotalNs     int64 `json:"p50TotalNs"`
	P95TotalNs     int64 `json:"p95TotalNs"`
	P99TotalNs     int64 `json:"p99TotalNs"`
}

// queryLatencyBreakdown fetches latency breakdown data from the collector API
func (r *FabricInferenceInsightReconciler) queryLatencyBreakdown(ctx context.Context, insight *gryviav1.FabricInferenceInsight) collectorLatencyResponse {
	logger := log.FromContext(ctx)

	httpClient := &http.Client{Timeout: 10 * time.Second}
	url := fmt.Sprintf("%s/api/v1/inference/latency?service=%s", collectorBaseURL, insight.Spec.TargetService)
	if insight.Spec.AnalysisWindow != "" {
		url += fmt.Sprintf("&window=%s", insight.Spec.AnalysisWindow)
	}

	resp, err := httpClient.Get(url)
	if err != nil {
		logger.V(1).Info("Failed to query latency breakdown from collector", "error", err)
		return collectorLatencyResponse{}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.V(1).Info("Collector returned non-OK for latency breakdown", "statusCode", resp.StatusCode)
		return collectorLatencyResponse{}
	}

	var latencyData collectorLatencyResponse
	if err := json.NewDecoder(resp.Body).Decode(&latencyData); err != nil {
		logger.Error(err, "Failed to decode latency breakdown response")
		return collectorLatencyResponse{}
	}

	return latencyData
}

// identifyLatencyBottleneck determines which phase contributes the most latency
func (r *FabricInferenceInsightReconciler) identifyLatencyBottleneck(breakdown gryviav1.LatencyBreakdown) string {
	phases := map[string]int64{
		"dns":           breakdown.DNSNs,
		"tcp_connect":   breakdown.TCPConnectNs,
		"tls_handshake": breakdown.TLSHandshakeNs,
		"gpu_queue":     breakdown.GPUQueueNs,
		"gpu_exec":      breakdown.GPUExecNs,
		"postprocess":   breakdown.PostprocessNs,
	}

	maxPhase := "gpu_exec"
	var maxLatency int64
	for phase, latency := range phases {
		if latency > maxLatency {
			maxLatency = latency
			maxPhase = phase
		}
	}

	return maxPhase
}

// updateInferenceInsightStatus updates the FabricInferenceInsight status subresource
func (r *FabricInferenceInsightReconciler) updateInferenceInsightStatus(ctx context.Context, namespacedName types.NamespacedName, phase string, breakdown gryviav1.LatencyBreakdown, p50, p95, p99 int64, bottleneck string) {
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		insight := &gryviav1.FabricInferenceInsight{}
		if err := r.Get(ctx, namespacedName, insight); err != nil {
			return err
		}
		insight.Status.Phase = phase
		insight.Status.LatencyBreakdown = breakdown
		insight.Status.P50TotalNs = p50
		insight.Status.P95TotalNs = p95
		insight.Status.P99TotalNs = p99
		insight.Status.Bottleneck = bottleneck
		insight.Status.LastAnalysis = metav1.Now()
		return r.Status().Update(ctx, insight)
	}); err != nil {
		log.FromContext(ctx).Error(err, "Failed to update FabricInferenceInsight status")
	}
}

// SetupWithManager sets up the controller with the Manager
func (r *FabricInferenceInsightReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gryviav1.FabricInferenceInsight{}).
		Complete(r)
}
