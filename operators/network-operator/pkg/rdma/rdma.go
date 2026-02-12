package rdma

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubefabricv1 "github.com/yourusername/kubefabric/operators/network-operator/api/v1"
)

const (
	RDMADevicePluginNamespace = "kube-system"
	RDMADevicePluginName      = "rdma-device-plugin"
)

// InstallDevicePlugin installs the RDMA device plugin DaemonSet
func InstallDevicePlugin(ctx context.Context, k8sClient client.Client) error {
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RDMADevicePluginName,
			Namespace: RDMADevicePluginNamespace,
			Labels: map[string]string{
				"app": RDMADevicePluginName,
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": RDMADevicePluginName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": RDMADevicePluginName,
					},
				},
				Spec: corev1.PodSpec{
					HostNetwork: true,
					Containers: []corev1.Container{
						{
							Name:  "rdma-device-plugin",
							Image: "ghcr.io/mellanox/k8s-rdma-shared-dev-plugin:latest",
							SecurityContext: &corev1.SecurityContext{
								Privileged: func() *bool { b := true; return &b }(),
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "device-plugin",
									MountPath: "/var/lib/kubelet/device-plugins",
								},
								{
									Name:      "config",
									MountPath: "/k8s-rdma-shared-dev-plugin",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "device-plugin",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/var/lib/kubelet/device-plugins",
								},
							},
						},
						{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "rdma-devices",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// Check if DaemonSet already exists
	existing := &appsv1.DaemonSet{}
	err := k8sClient.Get(ctx, types.NamespacedName{Name: ds.Name, Namespace: ds.Namespace}, existing)
	if err != nil {
		if errors.IsNotFound(err) {
			return k8sClient.Create(ctx, ds)
		}
		return err
	}

	// Update if exists
	existing.Spec = ds.Spec
	return k8sClient.Update(ctx, existing)
}

// ConfigureNode configures RDMA on a specific node
func ConfigureNode(ctx context.Context, k8sClient client.Client, node *corev1.Node, rdmaConfig *kubefabricv1.RDMAConfig) error {
	// Label node with RDMA capability
	if node.Labels == nil {
		node.Labels = make(map[string]string)
	}

	node.Labels["kubefabric.io/rdma"] = "enabled"
	node.Labels["kubefabric.io/rdma-mode"] = rdmaConfig.Mode

	// Add device annotations
	if node.Annotations == nil {
		node.Annotations = make(map[string]string)
	}

	if len(rdmaConfig.Devices) > 0 {
		node.Annotations["kubefabric.io/rdma-devices"] = fmt.Sprintf("%v", rdmaConfig.Devices)
	}

	return k8sClient.Update(ctx, node)
}

// CreateRDMAConfigMap creates a ConfigMap for RDMA device configuration
func CreateRDMAConfigMap(ctx context.Context, k8sClient client.Client, rdmaConfig *kubefabricv1.RDMAConfig) error {
	// RDMA device plugin configuration
	config := `{
  "configList": [
    {
      "resourceName": "rdma_shared_device_a",
      "rdmaHcaMax": 1000,
      "selectors": {
        "vendors": ["15b3"],
        "deviceIDs": ["1017", "1019", "101b"]
      }
    }
  ]
}`

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rdma-devices",
			Namespace: RDMADevicePluginNamespace,
		},
		Data: map[string]string{
			"config.json": config,
		},
	}

	existing := &corev1.ConfigMap{}
	err := k8sClient.Get(ctx, types.NamespacedName{Name: cm.Name, Namespace: cm.Namespace}, existing)
	if err != nil {
		if errors.IsNotFound(err) {
			return k8sClient.Create(ctx, cm)
		}
		return err
	}

	existing.Data = cm.Data
	return k8sClient.Update(ctx, existing)
}

// ValidateRDMASupport checks if node has RDMA capable hardware
func ValidateRDMASupport(node *corev1.Node) bool {
	// Check for Mellanox/NVIDIA network adapters
	if val, exists := node.Labels["feature.node.kubernetes.io/pci-15b3.present"]; exists && val == "true" {
		return true
	}
	return false
}
