package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kubefabricv1 "github.com/ssahani/kube-fabric/operators/network-operator/api/v1"
	"github.com/ssahani/kube-fabric/operators/network-operator/pkg/multus"
	"github.com/ssahani/kube-fabric/operators/network-operator/pkg/rdma"
	"github.com/ssahani/kube-fabric/operators/network-operator/pkg/sriov"
)

const (
	fabricNetworkFinalizer = "kubefabric.ai/network-finalizer"
)

// FabricNetworkReconciler reconciles a FabricNetwork object
type FabricNetworkReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=kubefabric.ai,resources=fabricnetworks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kubefabric.ai,resources=fabricnetworks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kubefabric.ai,resources=fabricnetworks/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=k8s.cni.cncf.io,resources=network-attachment-definitions,verbs=get;list;watch;create;update;patch;delete

func (r *FabricNetworkReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the FabricNetwork instance
	network := &kubefabricv1.FabricNetwork{}
	err := r.Get(ctx, req.NamespacedName, network)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("FabricNetwork resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get FabricNetwork")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !network.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, network)
	}

	// Add finalizer if it doesn't exist
	if !controllerutil.ContainsFinalizer(network, fabricNetworkFinalizer) {
		controllerutil.AddFinalizer(network, fabricNetworkFinalizer)
		if err := r.Update(ctx, network); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	logger.Info("Reconciling FabricNetwork", "name", network.Name, "type", network.Spec.NetworkType)

	// Reconcile the network configuration
	result, err := r.reconcileNetwork(ctx, network)
	if err != nil {
		logger.Error(err, "Failed to reconcile network")
		r.updateStatus(ctx, network, "Failed", err.Error())
		return result, err
	}

	// Update status to Ready
	r.updateStatus(ctx, network, "Ready", "Network configured successfully")

	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

func (r *FabricNetworkReconciler) reconcileNetwork(ctx context.Context, network *kubefabricv1.FabricNetwork) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Get matching nodes
	nodes, err := r.getMatchingNodes(ctx, network)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get matching nodes: %w", err)
	}

	if len(nodes) == 0 {
		logger.Info("No matching nodes found", "nodeSelector", network.Spec.NodeSelector)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	logger.Info("Found matching nodes", "count", len(nodes))

	// Update total nodes in status
	network.Status.TotalNodes = len(nodes)

	// Configure based on network type
	switch network.Spec.NetworkType {
	case "rdma":
		if err := r.configureRDMA(ctx, network, nodes); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to configure RDMA: %w", err)
		}
	case "sriov":
		if err := r.configureSRIOV(ctx, network, nodes); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to configure SR-IOV: %w", err)
		}
	case "standard":
		logger.Info("Standard network type - no special configuration needed")
	default:
		return ctrl.Result{}, fmt.Errorf("unknown network type: %s", network.Spec.NetworkType)
	}

	// Ensure Multus NetworkAttachmentDefinition
	if err := r.ensureNetworkAttachment(ctx, network); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create network attachment: %w", err)
	}

	network.Status.ConfiguredNodes = len(nodes)

	return ctrl.Result{}, nil
}

func (r *FabricNetworkReconciler) getMatchingNodes(ctx context.Context, network *kubefabricv1.FabricNetwork) ([]corev1.Node, error) {
	nodeList := &corev1.NodeList{}

	listOpts := []client.ListOption{}
	if len(network.Spec.NodeSelector) > 0 {
		listOpts = append(listOpts, client.MatchingLabels(network.Spec.NodeSelector))
	}

	if err := r.List(ctx, nodeList, listOpts...); err != nil {
		return nil, err
	}

	return nodeList.Items, nil
}

func (r *FabricNetworkReconciler) configureRDMA(ctx context.Context, network *kubefabricv1.FabricNetwork, nodes []corev1.Node) error {
	logger := log.FromContext(ctx)

	if network.Spec.RDMA == nil {
		return fmt.Errorf("RDMA configuration is required for rdma network type")
	}

	logger.Info("Configuring RDMA network", "mode", network.Spec.RDMA.Mode)

	// Install RDMA device plugin
	if err := rdma.InstallDevicePlugin(ctx, r.Client, network, r.Scheme); err != nil {
		return fmt.Errorf("failed to install RDMA device plugin: %w", err)
	}

	// Configure RDMA on each node, tracking failures
	failedNodes := 0
	for _, node := range nodes {
		if err := rdma.ConfigureNode(ctx, r.Client, &node, network.Spec.RDMA); err != nil {
			logger.Error(err, "Failed to configure RDMA on node", "node", node.Name)
			failedNodes++
			continue
		}
		logger.Info("Configured RDMA on node", "node", node.Name)
	}

	if failedNodes == len(nodes) {
		return fmt.Errorf("failed to configure RDMA on all %d nodes", failedNodes)
	}

	return nil
}

func (r *FabricNetworkReconciler) configureSRIOV(ctx context.Context, network *kubefabricv1.FabricNetwork, nodes []corev1.Node) error {
	logger := log.FromContext(ctx)

	if network.Spec.SRIOV == nil {
		return fmt.Errorf("SR-IOV configuration is required for sriov network type")
	}

	logger.Info("Configuring SR-IOV network", "interface", network.Spec.SRIOV.PhysicalInterface)

	// Install SR-IOV device plugin
	if err := sriov.InstallDevicePlugin(ctx, r.Client, network.Spec.SRIOV); err != nil {
		return fmt.Errorf("failed to install SR-IOV device plugin: %w", err)
	}

	// Configure SR-IOV on each node, tracking failures
	failedNodes := 0
	for _, node := range nodes {
		if err := sriov.ConfigureNode(ctx, r.Client, &node, network.Spec.SRIOV); err != nil {
			logger.Error(err, "Failed to configure SR-IOV on node", "node", node.Name)
			failedNodes++
			continue
		}
		logger.Info("Configured SR-IOV on node", "node", node.Name)
	}

	if failedNodes == len(nodes) {
		return fmt.Errorf("failed to configure SR-IOV on all %d nodes", failedNodes)
	}

	return nil
}

func (r *FabricNetworkReconciler) ensureNetworkAttachment(ctx context.Context, network *kubefabricv1.FabricNetwork) error {
	logger := log.FromContext(ctx)

	logger.Info("Creating NetworkAttachmentDefinition", "name", network.Name)

	if err := multus.CreateNetworkAttachment(ctx, r.Client, network); err != nil {
		return err
	}

	return nil
}

func (r *FabricNetworkReconciler) handleDeletion(ctx context.Context, network *kubefabricv1.FabricNetwork) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if controllerutil.ContainsFinalizer(network, fabricNetworkFinalizer) {
		logger.Info("Running cleanup for FabricNetwork", "name", network.Name)

		// Explicitly clean up DaemonSets and ConfigMaps since cross-namespace owner
		// references don't work (resources are in kube-system, owner is cluster-scoped).
		for _, name := range []string{"rdma-device-plugin", "sriov-device-plugin", "sriov-cni"} {
			ds := &appsv1.DaemonSet{}
			if err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: "kube-system"}, ds); err == nil {
				if delErr := r.Delete(ctx, ds); delErr != nil {
					logger.Error(delErr, "Failed to delete DaemonSet during cleanup", "daemonset", name)
				} else {
					logger.Info("Deleted DaemonSet during cleanup", "daemonset", name)
				}
			}
		}
		for _, name := range []string{"rdma-devices", "sriov-config"} {
			cm := &corev1.ConfigMap{}
			if err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: "kube-system"}, cm); err == nil {
				if delErr := r.Delete(ctx, cm); delErr != nil {
					logger.Error(delErr, "Failed to delete ConfigMap during cleanup", "configmap", name)
				}
			}
		}

		// Remove node labels applied by this network.
		nodes, err := r.getMatchingNodes(ctx, network)
		if err != nil {
			logger.Error(err, "Failed to list nodes during cleanup")
		} else {
			for _, node := range nodes {
				// Clean up RDMA labels/annotations
				delete(node.Labels, "kubefabric.ai/rdma")
				delete(node.Labels, "kubefabric.ai/rdma-mode")
				delete(node.Annotations, "kubefabric.ai/rdma-devices")
				// Clean up SR-IOV labels/annotations
				delete(node.Labels, "kubefabric.ai/sriov")
				delete(node.Annotations, "kubefabric.ai/sriov-interface")
				delete(node.Annotations, "kubefabric.ai/sriov-numvfs")
				// Remove any kubefabric.ai/sriov-* labels
				for k := range node.Labels {
					if strings.HasPrefix(k, "kubefabric.ai/sriov") {
						delete(node.Labels, k)
					}
				}
				if updateErr := r.Update(ctx, &node); updateErr != nil {
					logger.Error(updateErr, "Failed to remove labels from node", "node", node.Name)
				}
			}
		}

		// Remove finalizer
		controllerutil.RemoveFinalizer(network, fabricNetworkFinalizer)
		if err := r.Update(ctx, network); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *FabricNetworkReconciler) updateStatus(ctx context.Context, network *kubefabricv1.FabricNetwork, phase, message string) {
	network.Status.Phase = phase
	network.Status.LastUpdated = metav1.Now()

	condition := metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             phase,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}

	if phase == "Failed" {
		condition.Status = metav1.ConditionFalse
	}

	meta.SetStatusCondition(&network.Status.Conditions, condition)

	if err := r.Status().Update(ctx, network); err != nil {
		log.FromContext(ctx).Error(err, "Failed to update FabricNetwork status")
	}
}

// SetupWithManager sets up the controller with the Manager
func (r *FabricNetworkReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// NOTE: Owns() for DaemonSet and ConfigMap is not used here because those
	// resources live in kube-system while FabricNetwork is cluster-scoped.
	// Cross-namespace owner references cannot be set, so cleanup is handled
	// via the finalizer in handleDeletion instead.
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubefabricv1.FabricNetwork{}).
		Complete(r)
}
