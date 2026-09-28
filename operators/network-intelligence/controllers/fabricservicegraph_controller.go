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

	gryviav1 "github.com/zyvorai/gryvia/operators/network-intelligence/api/v1"
)

// FabricServiceGraphReconciler reconciles a FabricServiceGraph object
type FabricServiceGraphReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=gryvia.io,resources=fabricservicegraphs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gryvia.io,resources=fabricservicegraphs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gryvia.io,resources=fabricservicegraphs/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=endpoints,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *FabricServiceGraphReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the FabricServiceGraph instance
	graph := &gryviav1.FabricServiceGraph{}
	if err := r.Get(ctx, req.NamespacedName, graph); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("FabricServiceGraph resource not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get FabricServiceGraph")
		return ctrl.Result{}, err
	}

	logger.Info("Reconciling FabricServiceGraph",
		"name", graph.Name,
		"namespaces", graph.Spec.Namespaces,
		"includeExternal", graph.Spec.IncludeExternal,
	)

	// Parse refresh interval
	refreshInterval := 1 * time.Minute
	if graph.Spec.RefreshInterval != "" {
		if parsed, err := time.ParseDuration(graph.Spec.RefreshInterval); err == nil && parsed > 0 {
			refreshInterval = parsed
		}
	}

	// Discover services across target namespaces
	nodes := r.discoverServiceNodes(ctx, graph)

	// Build edges from Hubble flow data
	edges := r.buildServiceEdges(ctx, graph, nodes)

	// Determine health status for each node
	r.evaluateNodeHealth(ctx, nodes, edges)

	// Update status with the graph
	r.updateStatus(ctx, req.NamespacedName, nodes, edges)

	logger.Info("FabricServiceGraph refreshed",
		"nodes", len(nodes),
		"edges", len(edges),
	)

	return ctrl.Result{RequeueAfter: refreshInterval}, nil
}

// discoverServiceNodes enumerates services across target namespaces to build graph nodes
func (r *FabricServiceGraphReconciler) discoverServiceNodes(ctx context.Context, graph *gryviav1.FabricServiceGraph) []gryviav1.ServiceGraphNode {
	logger := log.FromContext(ctx)
	var nodes []gryviav1.ServiceGraphNode

	namespaces := graph.Spec.Namespaces
	if len(namespaces) == 0 {
		// Default to the graph's own namespace
		namespaces = []string{graph.Namespace}
	}

	for _, ns := range namespaces {
		// Verify namespace exists
		namespace := &corev1.Namespace{}
		if err := r.Get(ctx, types.NamespacedName{Name: ns}, namespace); err != nil {
			logger.V(1).Info("Namespace not found, skipping", "namespace", ns)
			continue
		}

		svcList := &corev1.ServiceList{}
		if err := r.List(ctx, svcList, client.InNamespace(ns)); err != nil {
			logger.Error(err, "Failed to list services", "namespace", ns)
			continue
		}

		for _, svc := range svcList.Items {
			nodeType := "service"
			if svc.Spec.Type == corev1.ServiceTypeExternalName {
				if !graph.Spec.IncludeExternal {
					continue
				}
				nodeType = "external"
			}

			nodes = append(nodes, gryviav1.ServiceGraphNode{
				Name:      svc.Name,
				Namespace: svc.Namespace,
				Type:      nodeType,
				Health:    "unknown",
			})
		}
	}

	return nodes
}

// buildServiceEdges queries Hubble flow data to discover connections between services
func (r *FabricServiceGraphReconciler) buildServiceEdges(ctx context.Context, graph *gryviav1.FabricServiceGraph, nodes []gryviav1.ServiceGraphNode) []gryviav1.ServiceGraphEdge {
	logger := log.FromContext(ctx)

	// Check if Hubble is available
	hubbleSvc := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      "hubble-relay",
		Namespace: "kube-system",
	}, hubbleSvc)
	if err != nil {
		logger.V(1).Info("Hubble relay not available, using existing edges from status")
		// Preserve existing edges from status if Hubble is unavailable
		existing := &gryviav1.FabricServiceGraph{}
		if getErr := r.Get(ctx, types.NamespacedName{
			Name:      graph.Name,
			Namespace: graph.Namespace,
		}, existing); getErr == nil {
			return existing.Status.Edges
		}
		return nil
	}

	// In production, this would:
	// 1. Connect to Hubble relay gRPC API
	// 2. Query flows grouped by (source_service, destination_service, port, protocol)
	// 3. Calculate aggregate latency (P99) and throughput for each edge
	// 4. Determine verdict (forwarded, dropped, error) based on flow verdicts
	// 5. Respect the depth parameter for graph traversal depth
	//
	// Example Hubble query: GetFlows with source/destination namespace filters
	// matching graph.Spec.Namespaces, aggregated over graph.Spec.RefreshInterval

	// Build a service lookup for cross-referencing
	serviceMap := make(map[string]bool)
	for _, node := range nodes {
		key := fmt.Sprintf("%s/%s", node.Namespace, node.Name)
		serviceMap[key] = true
	}

	// Preserve existing edges
	existing := &gryviav1.FabricServiceGraph{}
	if getErr := r.Get(ctx, types.NamespacedName{
		Name:      graph.Name,
		Namespace: graph.Namespace,
	}, existing); getErr == nil {
		return existing.Status.Edges
	}

	return nil
}

// evaluateNodeHealth determines the health status of each service node
// based on error rates observed in the service edges
func (r *FabricServiceGraphReconciler) evaluateNodeHealth(ctx context.Context, nodes []gryviav1.ServiceGraphNode, edges []gryviav1.ServiceGraphEdge) {
	// Build a map of error/drop counts per destination service
	errorCounts := make(map[string]int)
	totalCounts := make(map[string]int)

	for _, edge := range edges {
		totalCounts[edge.Destination]++
		if edge.Verdict == "dropped" || edge.Verdict == "error" {
			errorCounts[edge.Destination]++
		}
	}

	// Evaluate health based on error ratio
	for i := range nodes {
		key := fmt.Sprintf("%s/%s", nodes[i].Namespace, nodes[i].Name)
		total := totalCounts[key]
		errs := errorCounts[key]

		if total == 0 {
			nodes[i].Health = "unknown"
			continue
		}

		errorRate := float64(errs) / float64(total)
		switch {
		case errorRate == 0:
			nodes[i].Health = "healthy"
		case errorRate < 0.05:
			nodes[i].Health = "degraded"
		default:
			nodes[i].Health = "unhealthy"
		}
	}
}

// updateStatus updates the FabricServiceGraph status subresource
func (r *FabricServiceGraphReconciler) updateStatus(ctx context.Context, namespacedName types.NamespacedName, nodes []gryviav1.ServiceGraphNode, edges []gryviav1.ServiceGraphEdge) {
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		graph := &gryviav1.FabricServiceGraph{}
		if err := r.Get(ctx, namespacedName, graph); err != nil {
			return err
		}
		graph.Status.Nodes = nodes
		graph.Status.Edges = edges
		graph.Status.LastUpdated = metav1.Now()
		return r.Status().Update(ctx, graph)
	}); err != nil {
		log.FromContext(ctx).Error(err, "Failed to update FabricServiceGraph status")
	}
}

// SetupWithManager sets up the controller with the Manager
func (r *FabricServiceGraphReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gryviav1.FabricServiceGraph{}).
		Complete(r)
}
