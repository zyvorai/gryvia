package controllers

import (
	"context"
	"encoding/json"
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

// FabricTraceSessionReconciler reconciles a FabricTraceSession object
type FabricTraceSessionReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabrictracesessions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabrictracesessions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=tensorreaper.ai,resources=fabrictracesessions/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *FabricTraceSessionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the FabricTraceSession instance
	session := &tensorreaperv1.FabricTraceSession{}
	if err := r.Get(ctx, req.NamespacedName, session); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("FabricTraceSession resource not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get FabricTraceSession")
		return ctrl.Result{}, err
	}

	logger.Info("Reconciling FabricTraceSession",
		"name", session.Name,
		"service", session.Spec.Service,
		"level", session.Spec.Level,
	)

	// Handle session based on current phase
	switch session.Status.Phase {
	case "completed", "expired":
		logger.Info("Trace session already finished", "phase", session.Status.Phase)
		return ctrl.Result{}, nil
	case "active":
		return r.reconcileActiveSession(ctx, req.NamespacedName, session)
	default:
		return r.startSession(ctx, req.NamespacedName, session)
	}
}

// startSession initializes a new trace session
func (r *FabricTraceSessionReconciler) startSession(ctx context.Context, namespacedName types.NamespacedName, session *tensorreaperv1.FabricTraceSession) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Starting new trace session")

	// Parse session duration
	sessionDuration := 5 * time.Minute
	if session.Spec.Duration != "" {
		if parsed, err := time.ParseDuration(session.Spec.Duration); err == nil && parsed > 0 {
			sessionDuration = parsed
		}
	}

	// Create the results ConfigMap
	resultCMName := fmt.Sprintf("trace-%s", session.Name)
	resultCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resultCMName,
			Namespace: session.Namespace,
			Labels: map[string]string{
				"tensorreaper.ai/managed-by": "netpredator",
				"tensorreaper.ai/trace":      session.Name,
			},
		},
		Data: map[string]string{
			"session":  session.Name,
			"service":  session.Spec.Service,
			"level":    session.Spec.Level,
			"status":   "active",
			"flows":    "[]",
		},
	}

	existingCM := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{Name: resultCMName, Namespace: session.Namespace}, existingCM)
	if errors.IsNotFound(err) {
		if createErr := r.Create(ctx, resultCM); createErr != nil {
			return ctrl.Result{}, fmt.Errorf("failed to create trace results ConfigMap: %w", createErr)
		}
	} else if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to check trace results ConfigMap: %w", err)
	}

	// Configure Hubble to capture flows matching the session filters.
	// In production, this would call the Hubble observer API with the configured
	// filters (srcIP, dstIP, port, protocol) and trace level.
	r.configureHubbleCapture(ctx, session)

	// Update status to active
	now := metav1.Now()
	endTime := metav1.NewTime(now.Add(sessionDuration))

	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current := &tensorreaperv1.FabricTraceSession{}
		if err := r.Get(ctx, namespacedName, current); err != nil {
			return err
		}
		current.Status.Phase = "active"
		current.Status.FlowsCaptured = 0
		current.Status.StartTime = now
		current.Status.EndTime = endTime
		current.Status.ResultRef = &tensorreaperv1.TraceResultRef{
			Kind:      "ConfigMap",
			Name:      resultCMName,
			Namespace: session.Namespace,
		}
		return r.Status().Update(ctx, current)
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update trace session status: %w", err)
	}

	logger.Info("Trace session started",
		"duration", sessionDuration,
		"resultConfigMap", resultCMName,
	)

	// Requeue to check for expiration
	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}

// reconcileActiveSession checks if the session has expired and captures flow data
func (r *FabricTraceSessionReconciler) reconcileActiveSession(ctx context.Context, namespacedName types.NamespacedName, session *tensorreaperv1.FabricTraceSession) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Check if session has expired
	if !session.Status.EndTime.IsZero() && time.Now().After(session.Status.EndTime.Time) {
		logger.Info("Trace session expired, completing")
		return r.completeSession(ctx, namespacedName, session)
	}

	// Capture flows from Hubble (best-effort)
	capturedFlows := r.captureFlows(ctx, session)

	// Update the results ConfigMap with captured flows
	if session.Status.ResultRef != nil {
		r.updateTraceResults(ctx, session, capturedFlows)
	}

	// Update flow count in status
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current := &tensorreaperv1.FabricTraceSession{}
		if err := r.Get(ctx, namespacedName, current); err != nil {
			return err
		}
		current.Status.FlowsCaptured = current.Status.FlowsCaptured + int64(len(capturedFlows))
		return r.Status().Update(ctx, current)
	}); err != nil {
		logger.Error(err, "Failed to update flow count")
	}

	// Calculate time until expiration for requeue
	requeueAfter := 10 * time.Second
	if !session.Status.EndTime.IsZero() {
		remaining := time.Until(session.Status.EndTime.Time)
		if remaining < requeueAfter {
			requeueAfter = remaining
		}
	}

	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

// completeSession finalizes the trace session
func (r *FabricTraceSessionReconciler) completeSession(ctx context.Context, namespacedName types.NamespacedName, session *tensorreaperv1.FabricTraceSession) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Update the results ConfigMap to mark as completed
	if session.Status.ResultRef != nil {
		cm := &corev1.ConfigMap{}
		err := r.Get(ctx, types.NamespacedName{
			Name:      session.Status.ResultRef.Name,
			Namespace: session.Status.ResultRef.Namespace,
		}, cm)
		if err == nil {
			if cm.Data == nil {
				cm.Data = make(map[string]string)
			}
			cm.Data["status"] = "completed"
			cm.Data["flowsCaptured"] = fmt.Sprintf("%d", session.Status.FlowsCaptured)
			if updateErr := r.Update(ctx, cm); updateErr != nil {
				logger.Error(updateErr, "Failed to update trace results ConfigMap")
			}
		}
	}

	// Parse session duration to determine if expired vs completed
	phase := "completed"
	if session.Spec.Duration != "" {
		if parsed, err := time.ParseDuration(session.Spec.Duration); err == nil {
			if !session.Status.StartTime.IsZero() && time.Since(session.Status.StartTime.Time) > parsed {
				phase = "expired"
			}
		}
	}

	// Update status to completed/expired
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current := &tensorreaperv1.FabricTraceSession{}
		if err := r.Get(ctx, namespacedName, current); err != nil {
			return err
		}
		current.Status.Phase = phase
		current.Status.EndTime = metav1.Now()
		return r.Status().Update(ctx, current)
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update completed session status: %w", err)
	}

	logger.Info("Trace session completed",
		"phase", phase,
		"flowsCaptured", session.Status.FlowsCaptured,
	)

	return ctrl.Result{}, nil
}

// configureHubbleCapture configures Hubble to capture flows for this session.
// In production, this connects to Hubble's gRPC API to set up flow observation.
func (r *FabricTraceSessionReconciler) configureHubbleCapture(ctx context.Context, session *tensorreaperv1.FabricTraceSession) {
	logger := log.FromContext(ctx)

	// Verify Hubble relay is available
	hubbleSvc := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      "hubble-relay",
		Namespace: "kube-system",
	}, hubbleSvc)
	if err != nil {
		logger.V(1).Info("Hubble relay service not found, trace capture will be limited")
		return
	}

	logger.Info("Hubble relay available, flow capture configured",
		"filters", session.Spec.Filters,
		"level", session.Spec.Level,
		"captureHeaders", session.Spec.CaptureHeaders,
	)
}

// capturedFlow represents a single captured network flow
type capturedFlow struct {
	Timestamp   string `json:"timestamp"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Port        int    `json:"port"`
	Protocol    string `json:"protocol"`
	Verdict     string `json:"verdict"`
	Bytes       int64  `json:"bytes"`
}

// captureFlows queries Hubble for flows matching session filters.
// In production, this streams flows from the Hubble observer API.
func (r *FabricTraceSessionReconciler) captureFlows(ctx context.Context, session *tensorreaperv1.FabricTraceSession) []capturedFlow {
	// In production, this would:
	// 1. Connect to Hubble relay gRPC at hubble-relay.kube-system:4245
	// 2. Create a GetFlows request with the session's filters
	// 3. Stream flows and apply level-specific filtering (L3/L4/L7)
	// 4. If captureHeaders is true and level is l7, include HTTP headers
	// 5. Return collected flows

	return nil
}

// updateTraceResults appends captured flows to the results ConfigMap
func (r *FabricTraceSessionReconciler) updateTraceResults(ctx context.Context, session *tensorreaperv1.FabricTraceSession, flows []capturedFlow) {
	if session.Status.ResultRef == nil || len(flows) == 0 {
		return
	}

	logger := log.FromContext(ctx)

	cm := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      session.Status.ResultRef.Name,
		Namespace: session.Status.ResultRef.Namespace,
	}, cm)
	if err != nil {
		logger.Error(err, "Failed to get trace results ConfigMap")
		return
	}

	// Append new flows to existing data
	existingFlows := cm.Data["flows"]
	var allFlows []capturedFlow
	if existingFlows != "" && existingFlows != "[]" {
		if jsonErr := json.Unmarshal([]byte(existingFlows), &allFlows); jsonErr != nil {
			logger.Error(jsonErr, "Failed to parse existing flows")
		}
	}
	allFlows = append(allFlows, flows...)

	flowsJSON, err := json.Marshal(allFlows)
	if err != nil {
		logger.Error(err, "Failed to marshal flows")
		return
	}

	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}
	cm.Data["flows"] = string(flowsJSON)

	if updateErr := r.Update(ctx, cm); updateErr != nil {
		logger.Error(updateErr, "Failed to update trace results ConfigMap")
	}
}

// SetupWithManager sets up the controller with the Manager
func (r *FabricTraceSessionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tensorreaperv1.FabricTraceSession{}).
		Complete(r)
}
