package multus

import (
	"context"
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"

	kubefabricv1 "github.com/ssahani/kube-fabric/operators/network-operator/api/v1"
)

// CreateNetworkAttachment creates a Multus NetworkAttachmentDefinition
func CreateNetworkAttachment(ctx context.Context, k8sClient client.Client, network *kubefabricv1.FabricNetwork) error {
	config, err := generateNetworkConfig(network)
	if err != nil {
		return fmt.Errorf("failed to generate network config: %w", err)
	}

	// Use the network's target namespace, or default to "default"
	namespace := network.Spec.TargetNamespace
	if namespace == "" {
		namespace = "default"
	}

	nad := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "k8s.cni.cncf.io/v1",
			"kind":       "NetworkAttachmentDefinition",
			"metadata": map[string]interface{}{
				"name":      network.Name,
				"namespace": namespace,
				"labels": map[string]interface{}{
					"kubefabric.ai/network": network.Name,
					"kubefabric.ai/type":    network.Spec.NetworkType,
				},
			},
			"spec": map[string]interface{}{
				"config": config,
			},
		},
	}

	nad.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "k8s.cni.cncf.io",
		Version: "v1",
		Kind:    "NetworkAttachmentDefinition",
	})

	existing := &unstructured.Unstructured{}
	existing.SetGroupVersionKind(nad.GroupVersionKind())

	err = k8sClient.Get(ctx, types.NamespacedName{Name: network.Name, Namespace: namespace}, existing)
	if err != nil {
		if errors.IsNotFound(err) {
			return k8sClient.Create(ctx, nad)
		}
		return err
	}

	// Update existing
	nad.SetResourceVersion(existing.GetResourceVersion())
	return k8sClient.Update(ctx, nad)
}

func generateNetworkConfig(network *kubefabricv1.FabricNetwork) (string, error) {
	switch network.Spec.NetworkType {
	case "rdma":
		return generateRDMAConfig(network)
	case "sriov":
		return generateSRIOVConfig(network)
	case "standard":
		return generateStandardConfig(network)
	default:
		return "", fmt.Errorf("unknown network type: %s", network.Spec.NetworkType)
	}
}

func generateRDMAConfig(network *kubefabricv1.FabricNetwork) (string, error) {
	if network.Spec.RDMA == nil {
		return "", fmt.Errorf("RDMA configuration required")
	}

	mtu := network.Spec.MTU
	if mtu == 0 {
		mtu = 9000 // Default MTU for RDMA
	}

	configObj := map[string]interface{}{
		"cniVersion": "0.3.1",
		"name":       network.Name,
		"type":       "macvlan",
		"master":     "ib0",
		"mode":       "bridge",
		"mtu":        mtu,
		"ipam": map[string]interface{}{
			"type":    "whereabouts",
			"range":   network.Spec.RDMA.Subnet,
			"gateway": network.Spec.RDMA.Gateway,
		},
	}
	configBytes, err := json.Marshal(configObj)
	if err != nil {
		return "", fmt.Errorf("failed to marshal RDMA config: %w", err)
	}

	return string(configBytes), nil
}

func generateSRIOVConfig(network *kubefabricv1.FabricNetwork) (string, error) {
	if network.Spec.SRIOV == nil {
		return "", fmt.Errorf("SR-IOV configuration required")
	}

	mtu := network.Spec.MTU
	if mtu == 0 {
		mtu = 9000
	}

	// Use SRIOV spec values for IPAM if available, otherwise use defaults
	subnet := "10.56.0.0/16"
	gateway := "10.56.217.1"
	if network.Spec.SRIOV != nil {
		if network.Spec.SRIOV.Subnet != "" {
			subnet = network.Spec.SRIOV.Subnet
		}
		if network.Spec.SRIOV.Gateway != "" {
			gateway = network.Spec.SRIOV.Gateway
		}
	}

	configObj := map[string]interface{}{
		"cniVersion": "0.3.1",
		"name":       network.Name,
		"type":       "sriov",
		"vlan":       0,
		"mtu":        mtu,
		"ipam": map[string]interface{}{
			"type":   "host-local",
			"subnet": subnet,
			"routes": []map[string]string{{"dst": "0.0.0.0/0"}},
			"gateway": gateway,
		},
	}
	configBytes, err := json.Marshal(configObj)
	if err != nil {
		return "", fmt.Errorf("failed to marshal SR-IOV config: %w", err)
	}

	return string(configBytes), nil
}

func generateStandardConfig(network *kubefabricv1.FabricNetwork) (string, error) {
	mtu := network.Spec.MTU
	if mtu == 0 {
		mtu = 1500
	}

	configObj := map[string]interface{}{
		"cniVersion": "0.3.1",
		"name":       network.Name,
		"type":       "bridge",
		"bridge":     "kubefabric0",
		"mtu":        mtu,
		"ipam": map[string]interface{}{
			"type":   "host-local",
			"subnet": "10.244.0.0/16",
		},
	}
	configBytes, err := json.Marshal(configObj)
	if err != nil {
		return "", fmt.Errorf("failed to marshal standard config: %w", err)
	}

	return string(configBytes), nil
}

// GetNetworkAttachment retrieves a NetworkAttachmentDefinition
func GetNetworkAttachment(ctx context.Context, k8sClient client.Client, name, namespace string) (*unstructured.Unstructured, error) {
	nad := &unstructured.Unstructured{}
	nad.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "k8s.cni.cncf.io",
		Version: "v1",
		Kind:    "NetworkAttachmentDefinition",
	})

	err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, nad)
	return nad, err
}
