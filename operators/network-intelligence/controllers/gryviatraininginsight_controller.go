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
	// trainingInsightInterval is the default requeue interval while a job is running
	trainingInsightInterval = 60 * time.Second
)

// GryviaTrainingInsightReconciler reconciles a GryviaTrainingInsight object
type GryviaTrainingInsightReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=gryvia.io,resources=gryviatraininginsights,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gryvia.io,resources=gryviatraininginsights/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gryvia.io,resources=gryviatraininginsights/finalizers,verbs=update
//+kubebuilder:rbac:groups=gryvia.io,resources=gryviaaijobs,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch

func (r *GryviaTrainingInsightReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the GryviaTrainingInsight instance
	insight := &gryviav1.GryviaTrainingInsight{}
	if err := r.Get(ctx, req.NamespacedName, insight); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("GryviaTrainingInsight resource not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get GryviaTrainingInsight")
		return ctrl.Result{}, err
	}

	logger.Info("Reconciling GryviaTrainingInsight",
		"name", insight.Name,
		"targetJob", insight.Spec.TargetJob,
		"metrics", insight.Spec.Metrics,
	)

	// Query collector for NCCL training stats
	trainingData := r.queryTrainingStats(ctx, insight)

	// Populate rank stats
	var rankStats []gryviav1.RankStat
	for _, rd := range trainingData.RankStats {
		rankStats = append(rankStats, gryviav1.RankStat{
			Rank:         rd.Rank,
			AvgLatencyNs: int64(rd.AvgLatencyNs),
			TotalBytes:   rd.TotalBytes,
			IsStraggler:  rd.IsStraggler,
		})
	}

	// Detect stragglers
	stragglers := r.detectStragglers(rankStats)

	// Detect communication pattern
	commPattern := trainingData.CommPattern
	if commPattern == "" {
		commPattern = "unknown"
	}

	// Calculate communication-to-compute ratio
	commComputeRatio := trainingData.CommComputeRatio

	// Identify bottleneck
	bottleneck := r.identifyBottleneck(commComputeRatio)
	phase := "Active"
	if len(rankStats) == 0 {
		phase = "AwaitingData"
		bottleneck = "unknown"
	}

	// Update status
	r.updateTrainingInsightStatus(ctx, req.NamespacedName, phase, rankStats, commPattern, commComputeRatio, stragglers, bottleneck)

	logger.Info("GryviaTrainingInsight analysis complete",
		"ranks", len(rankStats),
		"stragglers", len(stragglers),
		"bottleneck", bottleneck,
		"commPattern", commPattern,
	)

	return ctrl.Result{RequeueAfter: trainingInsightInterval}, nil
}

// collectorTrainingResponse represents the response from the collector training API
type collectorTrainingResponse struct {
	RankStats map[string]struct {
		Rank         int     `json:"rank"`
		AvgLatencyNs float64 `json:"avg_latency_ns"`
		TotalBytes   int64   `json:"total_bytes"`
		IsStraggler  bool    `json:"is_straggler"`
	} `json:"rank_stats"`
	CommPattern      string  `json:"pattern"`
	CommComputeRatio float64 `json:"comm_compute_ratio"`
}

// queryTrainingStats fetches NCCL training stats from the collector API
func (r *GryviaTrainingInsightReconciler) queryTrainingStats(ctx context.Context, insight *gryviav1.GryviaTrainingInsight) collectorTrainingResponse {
	logger := log.FromContext(ctx)

	httpClient := &http.Client{Timeout: 10 * time.Second}
	url := fmt.Sprintf("%s/api/v1/ai/training?job=%s", collectorBaseURL, insight.Spec.TargetJob)
	if insight.Spec.AnalysisWindow != "" {
		url += fmt.Sprintf("&window=%s", insight.Spec.AnalysisWindow)
	}

	resp, err := httpClient.Get(url)
	if err != nil {
		logger.V(1).Info("Failed to query training stats from collector", "error", err)
		return collectorTrainingResponse{}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.V(1).Info("Collector returned non-OK for training stats", "statusCode", resp.StatusCode)
		return collectorTrainingResponse{}
	}

	var trainingData collectorTrainingResponse
	if err := json.NewDecoder(resp.Body).Decode(&trainingData); err != nil {
		logger.Error(err, "Failed to decode training stats response")
		return collectorTrainingResponse{}
	}

	return trainingData
}

// detectStragglers analyzes rank stats to identify straggler ranks
func (r *GryviaTrainingInsightReconciler) detectStragglers(rankStats []gryviav1.RankStat) []gryviav1.StragglerInfo {
	if len(rankStats) < 2 {
		return nil
	}

	// Calculate mean latency across ranks.
	var totalLatency int64
	for _, rs := range rankStats {
		totalLatency += rs.AvgLatencyNs
	}
	meanLatency := totalLatency / int64(len(rankStats))

	if meanLatency == 0 {
		return nil
	}

	// Identify ranks that are significantly slower than the median
	var stragglers []gryviav1.StragglerInfo
	for _, rs := range rankStats {
		slowdownFactor := float64(rs.AvgLatencyNs) / float64(meanLatency)
		if slowdownFactor > 1.5 {
			stragglers = append(stragglers, gryviav1.StragglerInfo{
				Rank:           rs.Rank,
				SlowdownFactor: slowdownFactor,
				Reason:         "high_communication_latency",
			})
		}
	}

	return stragglers
}

// identifyBottleneck determines the primary bottleneck based on the comm-compute ratio
func (r *GryviaTrainingInsightReconciler) identifyBottleneck(commComputeRatio float64) string {
	switch {
	case commComputeRatio > 0.5:
		return "communication"
	case commComputeRatio < 0.1:
		return "data_loading"
	default:
		return "compute"
	}
}

// updateTrainingInsightStatus updates the GryviaTrainingInsight status subresource
func (r *GryviaTrainingInsightReconciler) updateTrainingInsightStatus(ctx context.Context, namespacedName types.NamespacedName, phase string, rankStats []gryviav1.RankStat, commPattern string, commComputeRatio float64, stragglers []gryviav1.StragglerInfo, bottleneck string) {
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		insight := &gryviav1.GryviaTrainingInsight{}
		if err := r.Get(ctx, namespacedName, insight); err != nil {
			return err
		}
		insight.Status.Phase = phase
		insight.Status.RankStats = rankStats
		insight.Status.CommPattern = commPattern
		insight.Status.CommComputeRatio = commComputeRatio
		insight.Status.Stragglers = stragglers
		insight.Status.Bottleneck = bottleneck
		insight.Status.LastAnalysis = metav1.Now()
		return r.Status().Update(ctx, insight)
	}); err != nil {
		log.FromContext(ctx).Error(err, "Failed to update GryviaTrainingInsight status")
	}
}

// SetupWithManager sets up the controller with the Manager
func (r *GryviaTrainingInsightReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gryviav1.GryviaTrainingInsight{}).
		Complete(r)
}
