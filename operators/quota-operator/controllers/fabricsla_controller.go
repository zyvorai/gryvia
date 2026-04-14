package controllers

import (
	"context"
	"fmt"
	"sort"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	tensorreaperv1 "github.com/ssahani/TensorReaper/operators/quota-operator/api/v1"
)

const (
	maxSLABreaches = 50
)

// FabricSLAReconciler reconciles a FabricSLA object
type FabricSLAReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabricslas,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabricslas/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabricslas/finalizers,verbs=update
//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabricaijobs,verbs=get;list;watch
//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabricquotas,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *FabricSLAReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the FabricSLA instance
	sla := &tensorreaperv1.FabricSLA{}
	err := r.Get(ctx, req.NamespacedName, sla)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("FabricSLA resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get FabricSLA")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !sla.ObjectMeta.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	logger.Info("Reconciling FabricSLA", "tier", sla.Spec.Tier, "scope", sla.Spec.Scope.Name)

	// Reconcile the SLA
	result, err := r.reconcileSLA(ctx, sla)
	if err != nil {
		logger.Error(err, "Failed to reconcile SLA")
		r.updateCondition(sla, "Ready", metav1.ConditionFalse, "ReconcileFailed", err.Error())
		if statusErr := r.Status().Update(ctx, sla); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after reconcile failure")
		}
		return result, err
	}

	// Update status
	if err := r.Status().Update(ctx, sla); err != nil {
		logger.Error(err, "Failed to update FabricSLA status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

func (r *FabricSLAReconciler) reconcileSLA(ctx context.Context, sla *tensorreaperv1.FabricSLA) (ctrl.Result, error) {
	// Get jobs matching the SLA scope
	jobs, err := r.getScopeJobs(ctx, sla)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get scope jobs: %w", err)
	}

	// Calculate SLA metrics
	metrics := r.calculateMetrics(sla, jobs)
	sla.Status.Metrics = metrics

	// Check for SLA breaches
	r.checkBreaches(sla, jobs)

	// Calculate compliance
	sla.Status.Compliance = r.calculateCompliance(sla)

	r.updateCondition(sla, "Ready", metav1.ConditionTrue, "SLAActive",
		fmt.Sprintf("SLA %s: compliance=%.2f%%", sla.Spec.Tier, sla.Status.Compliance.Current))

	return ctrl.Result{}, nil
}

func (r *FabricSLAReconciler) getScopeJobs(ctx context.Context, sla *tensorreaperv1.FabricSLA) ([]tensorreaperv1.FabricAIJob, error) {
	var allJobs []tensorreaperv1.FabricAIJob

	switch sla.Spec.Scope.Type {
	case "namespace":
		jobList := &tensorreaperv1.FabricAIJobList{}
		if err := r.List(ctx, jobList, client.InNamespace(sla.Spec.Scope.Name)); err != nil {
			return nil, err
		}
		allJobs = jobList.Items

	case "team":
		quotaList := &tensorreaperv1.FabricQuotaList{}
		if err := r.List(ctx, quotaList); err != nil {
			return nil, err
		}
		for _, quota := range quotaList.Items {
			if quota.Spec.Team == sla.Spec.Scope.Name {
				for _, ns := range quota.Spec.Namespaces {
					jobList := &tensorreaperv1.FabricAIJobList{}
					if err := r.List(ctx, jobList, client.InNamespace(ns)); err != nil {
						continue
					}
					allJobs = append(allJobs, jobList.Items...)
				}
			}
		}

	default:
		jobList := &tensorreaperv1.FabricAIJobList{}
		if err := r.List(ctx, jobList); err != nil {
			return nil, err
		}
		allJobs = jobList.Items
	}

	return allJobs, nil
}

func (r *FabricSLAReconciler) calculateMetrics(sla *tensorreaperv1.FabricSLA, jobs []tensorreaperv1.FabricAIJob) *tensorreaperv1.SLAMetricsStatus {
	metrics := &tensorreaperv1.SLAMetricsStatus{}

	// Calculate queue times
	var queueTimes []time.Duration
	for _, job := range jobs {
		if job.Status.StartTime == nil || job.Status.StartTime.IsZero() {
			// Job is still in queue
			if job.Status.Phase == "Pending" || job.Status.Phase == "Queued" {
				queueTime := time.Since(job.CreationTimestamp.Time)
				queueTimes = append(queueTimes, queueTime)
			}
			continue
		}

		// Job has started, calculate actual queue time
		queueTime := job.Status.StartTime.Time.Sub(job.CreationTimestamp.Time)
		if queueTime > 0 {
			queueTimes = append(queueTimes, queueTime)
		}
	}

	if len(queueTimes) > 0 {
		// Sort for percentile calculations
		sort.Slice(queueTimes, func(i, j int) bool {
			return queueTimes[i] < queueTimes[j]
		})

		// Average queue time
		totalQueueTime := time.Duration(0)
		for _, qt := range queueTimes {
			totalQueueTime += qt
		}
		avgQueueTime := totalQueueTime / time.Duration(len(queueTimes))
		metrics.AvgQueueTime = avgQueueTime.Round(time.Second).String()

		// P95
		p95Index := int(float64(len(queueTimes)) * 0.95)
		if p95Index >= len(queueTimes) {
			p95Index = len(queueTimes) - 1
		}
		metrics.P95QueueTime = queueTimes[p95Index].Round(time.Second).String()

		// P99
		p99Index := int(float64(len(queueTimes)) * 0.99)
		if p99Index >= len(queueTimes) {
			p99Index = len(queueTimes) - 1
		}
		metrics.P99QueueTime = queueTimes[p99Index].Round(time.Second).String()
	}

	// Calculate uptime (percentage of jobs that ran without failure)
	totalJobs := 0
	successfulJobs := 0
	for _, job := range jobs {
		if job.Status.Phase == "Running" || job.Status.Phase == "Succeeded" || job.Status.Phase == "Failed" {
			totalJobs++
			if job.Status.Phase != "Failed" {
				successfulJobs++
			}
		}
	}
	if totalJobs > 0 {
		metrics.Uptime = (float64(successfulJobs) / float64(totalJobs)) * 100
	} else {
		metrics.Uptime = 100.0
	}

	// Count breaches
	metrics.Breaches = len(sla.Status.Breaches)

	return metrics
}

func (r *FabricSLAReconciler) checkBreaches(sla *tensorreaperv1.FabricSLA, jobs []tensorreaperv1.FabricAIJob) {
	// Check queue time SLA
	if sla.Spec.Performance != nil && sla.Spec.Performance.MaxQueueTime != "" {
		maxQueueTime, err := time.ParseDuration(sla.Spec.Performance.MaxQueueTime)
		if err == nil {
			for _, job := range jobs {
				var queueTime time.Duration
				if job.Status.StartTime != nil && !job.Status.StartTime.IsZero() {
					queueTime = job.Status.StartTime.Time.Sub(job.CreationTimestamp.Time)
				} else if job.Status.Phase == "Pending" || job.Status.Phase == "Queued" {
					queueTime = time.Since(job.CreationTimestamp.Time)
				}

				if queueTime > maxQueueTime {
					breach := tensorreaperv1.SLABreach{
						Timestamp:   metav1.Now(),
						Type:        "queue-time-exceeded",
						Severity:    r.breachSeverity(sla),
						Duration:    (queueTime - maxQueueTime).Round(time.Second).String(),
						Compensated: false,
					}
					r.addBreach(sla, breach)
				}
			}
		}
	}

	// Check availability SLA
	if sla.Spec.Availability != nil && sla.Spec.Availability.Uptime > 0 {
		if sla.Status.Metrics != nil && sla.Status.Metrics.Uptime < sla.Spec.Availability.Uptime {
			breach := tensorreaperv1.SLABreach{
				Timestamp:   metav1.Now(),
				Type:        "availability-breach",
				Severity:    r.breachSeverity(sla),
				Duration:    "ongoing",
				Compensated: false,
			}
			r.addBreach(sla, breach)
		}
	}
}

func (r *FabricSLAReconciler) addBreach(sla *tensorreaperv1.FabricSLA, breach tensorreaperv1.SLABreach) {
	// Avoid duplicate breaches of the same type within 5 minutes
	for _, existing := range sla.Status.Breaches {
		if existing.Type == breach.Type {
			timeDiff := breach.Timestamp.Time.Sub(existing.Timestamp.Time)
			if timeDiff < 5*time.Minute {
				return
			}
		}
	}

	sla.Status.Breaches = append(sla.Status.Breaches, breach)

	// Maintain rolling buffer
	if len(sla.Status.Breaches) > maxSLABreaches {
		sla.Status.Breaches = sla.Status.Breaches[len(sla.Status.Breaches)-maxSLABreaches:]
	}
}

func (r *FabricSLAReconciler) breachSeverity(sla *tensorreaperv1.FabricSLA) string {
	switch sla.Spec.Tier {
	case "platinum":
		return "critical"
	case "gold":
		return "high"
	case "silver":
		return "medium"
	default:
		return "low"
	}
}

func (r *FabricSLAReconciler) calculateCompliance(sla *tensorreaperv1.FabricSLA) *tensorreaperv1.SLAComplianceStatus {
	compliance := &tensorreaperv1.SLAComplianceStatus{}

	// Calculate compliance based on breaches in the current month
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	breachesThisMonth := 0
	for _, breach := range sla.Status.Breaches {
		if breach.Timestamp.Time.After(startOfMonth) {
			breachesThisMonth++
		}
	}

	// Base compliance on uptime metrics
	if sla.Status.Metrics != nil {
		compliance.Current = sla.Status.Metrics.Uptime
	} else {
		compliance.Current = 100.0
	}

	// Determine trend
	if sla.Status.Compliance != nil {
		if compliance.Current > sla.Status.Compliance.Current {
			compliance.Trend = "improving"
		} else if compliance.Current < sla.Status.Compliance.Current {
			compliance.Trend = "degrading"
		} else {
			compliance.Trend = "stable"
		}
		compliance.LastMonth = sla.Status.Compliance.Current
	} else {
		compliance.Trend = "stable"
		compliance.LastMonth = compliance.Current
	}

	return compliance
}

func (r *FabricSLAReconciler) updateCondition(sla *tensorreaperv1.FabricSLA, condType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}
	meta.SetStatusCondition(&sla.Status.Conditions, condition)
}

// SetupWithManager sets up the controller with the Manager.
func (r *FabricSLAReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tensorreaperv1.FabricSLA{}).
		Complete(r)
}
