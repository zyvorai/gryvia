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

	tensorreaperv1 "github.com/ssahani/TensorReaper/operators/network-intelligence/api/v1"
)

const (
	// trainingInsightInterval is the default requeue interval while a job is running
	trainingInsightInterval = 60 * time.Second
)

// FabricTrainingInsightReconciler reconciles a FabricTrainingInsight object
type FabricTrainingInsightReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabrictraininginsights,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabrictraininginsights/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabrictraininginsights/finalizers,verbs=update
//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabricaijobs,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch

func (r *FabricTrainingInsightReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the FabricTrainingInsight instance
	insight := &tensorreaperv1.FabricTrainingInsight{}
	if err := r.Get(ctx, req.NamespacedName, insight); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("FabricTrainingInsight resource not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get FabricTrainingInsight")
		return ctrl.Result{}, err
	}

	logger.Info("Reconciling FabricTrainingInsight",
		"name", insight.Name,
		"targetJob", insight.Spec.TargetJob,
		"metrics", insight.Spec.Metrics,
	)

	// Query collector for NCCL training stats
	trainingData := r.queryTrainingStats(ctx, insight)

	// Populate rank stats
	var rankStats []tensorreaperv1.RankStat
	for _, rd := range trainingData.RankStats {
		rankStats = append(rankStats, tensorreaperv1.RankStat{
			Rank:         rd.Rank,
			AvgLatencyNs: rd.AvgLatencyNs,
			TotalBytes:   rd.TotalBytes,
			IsStraggler:  rd.IsStraggler,
		})
	}

	// Detect stragglers
	stragglers := r.detectStragglers(rankStats)

	// Detect communication pattern
	commPattern := trainingData.CommPattern
	if commPattern == "" {
		commPattern = "ring_allreduce"
	}

	// Calculate communication-to-compute ratio
	commComputeRatio := trainingData.CommComputeRatio

	// Identify bottleneck
	bottleneck := r.identifyBottleneck(commComputeRatio)

	// Update status
	r.updateTrainingInsightStatus(ctx, req.NamespacedName, "Active", rankStats, commPattern, commComputeRatio, stragglers, bottleneck)

	logger.Info("FabricTrainingInsight analysis complete",
		"ranks", len(rankStats),
		"stragglers", len(stragglers),
		"bottleneck", bottleneck,
		"commPattern", commPattern,
	)

	return ctrl.Result{RequeueAfter: trainingInsightInterval}, nil
}

// collectorTrainingResponse represents the response from the collector training API
type collectorTrainingResponse struct {
	RankStats []struct {
		Rank         int   `json:"rank"`
		AvgLatencyNs int64 `json:"avgLatencyNs"`
		TotalBytes   int64 `json:"totalBytes"`
		IsStraggler  bool  `json:"isStraggler"`
	} `json:"rankStats"`
	CommPattern      string  `json:"commPattern"`
	CommComputeRatio float64 `json:"commComputeRatio"`
}

// queryTrainingStats fetches NCCL training stats from the collector API
func (r *FabricTrainingInsightReconciler) queryTrainingStats(ctx context.Context, insight *tensorreaperv1.FabricTrainingInsight) collectorTrainingResponse {
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
func (r *FabricTrainingInsightReconciler) detectStragglers(rankStats []tensorreaperv1.RankStat) []tensorreaperv1.StragglerInfo {
	if len(rankStats) < 2 {
		return nil
	}

	// Calculate median latency
	var totalLatency int64
	for _, rs := range rankStats {
		totalLatency += rs.AvgLatencyNs
	}
	medianLatency := totalLatency / int64(len(rankStats))

	if medianLatency == 0 {
		return nil
	}

	// Identify ranks that are significantly slower than the median
	var stragglers []tensorreaperv1.StragglerInfo
	for _, rs := range rankStats {
		slowdownFactor := float64(rs.AvgLatencyNs) / float64(medianLatency)
		if slowdownFactor > 1.5 {
			reason := "high_communication_latency"
			if rs.TotalBytes > 0 {
				avgBytesPerRank := totalLatency / int64(len(rankStats))
				if float64(rs.TotalBytes) > float64(avgBytesPerRank)*2.0 {
					reason = "excessive_data_transfer"
				}
			}
			stragglers = append(stragglers, tensorreaperv1.StragglerInfo{
				Rank:           rs.Rank,
				SlowdownFactor: slowdownFactor,
				Reason:         reason,
			})
		}
	}

	return stragglers
}

// identifyBottleneck determines the primary bottleneck based on the comm-compute ratio
func (r *FabricTrainingInsightReconciler) identifyBottleneck(commComputeRatio float64) string {
	switch {
	case commComputeRatio > 0.5:
		return "communication"
	case commComputeRatio < 0.1:
		return "data_loading"
	default:
		return "compute"
	}
}

// updateTrainingInsightStatus updates the FabricTrainingInsight status subresource
func (r *FabricTrainingInsightReconciler) updateTrainingInsightStatus(ctx context.Context, namespacedName types.NamespacedName, phase string, rankStats []tensorreaperv1.RankStat, commPattern string, commComputeRatio float64, stragglers []tensorreaperv1.StragglerInfo, bottleneck string) {
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		insight := &tensorreaperv1.FabricTrainingInsight{}
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
		log.FromContext(ctx).Error(err, "Failed to update FabricTrainingInsight status")
	}
}

// SetupWithManager sets up the controller with the Manager
func (r *FabricTrainingInsightReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tensorreaperv1.FabricTrainingInsight{}).
		Complete(r)
}
