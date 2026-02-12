package sriov

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
	SRIOVDevicePluginNamespace = "kube-system"
	SRIOVDevicePluginName      = "sriov-device-plugin"
	SRIOVCNIName               = "sriov-cni"
)

// InstallDevicePlugin installs the SR-IOV device plugin
func InstallDevicePlugin(ctx context.Context, k8sClient client.Client, sriovConfig *kubefabricv1.SRIOVConfig) error {
	// Install SR-IOV CNI first
	if err := installSRIOVCNI(ctx, k8sClient); err != nil {
		return fmt.Errorf("failed to install SR-IOV CNI: %w", err)
	}

	// Create ConfigMap for SR-IOV device plugin
	if err := createSRIOVConfigMap(ctx, k8sClient, sriovConfig); err != nil {
		return fmt.Errorf("failed to create SR-IOV ConfigMap: %w", err)
	}

	// Install device plugin DaemonSet
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      SRIOVDevicePluginName,
			Namespace: SRIOVDevicePluginNamespace,
			Labels: map[string]string{
				"app": SRIOVDevicePluginName,
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": SRIOVDevicePluginName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": SRIOVDevicePluginName,
					},
				},
				Spec: corev1.PodSpec{
					HostNetwork: true,
					Containers: []corev1.Container{
						{
							Name:  "sriov-device-plugin",
							Image: "ghcr.io/k8snetworkplumbingwg/sriov-network-device-plugin:latest",
							Args: []string{
								"--log-level=10",
							},
							SecurityContext: &corev1.SecurityContext{
								Privileged: func() *bool { b := true; return &b }(),
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "devicesock",
									MountPath: "/var/lib/kubelet/device-plugins",
								},
								{
									Name:      "config",
									MountPath: "/etc/pcidp/config.json",
									SubPath:   "config.json",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "devicesock",
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
										Name: "sriov-config",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	existing := &appsv1.DaemonSet{}
	err := k8sClient.Get(ctx, types.NamespacedName{Name: ds.Name, Namespace: ds.Namespace}, existing)
	if err != nil {
		if errors.IsNotFound(err) {
			return k8sClient.Create(ctx, ds)
		}
		return err
	}

	existing.Spec = ds.Spec
	return k8sClient.Update(ctx, existing)
}

func installSRIOVCNI(ctx context.Context, k8sClient client.Client) error {
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      SRIOVCNIName,
			Namespace: SRIOVDevicePluginNamespace,
			Labels: map[string]string{
				"app": SRIOVCNIName,
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": SRIOVCNIName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": SRIOVCNIName,
					},
				},
				Spec: corev1.PodSpec{
					HostNetwork: true,
					Containers: []corev1.Container{
						{
							Name:  "sriov-cni",
							Image: "ghcr.io/k8snetworkplumbingwg/sriov-cni:latest",
							SecurityContext: &corev1.SecurityContext{
								Privileged: func() *bool { b := true; return &b }(),
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "cnibin",
									MountPath: "/host/opt/cni/bin",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "cnibin",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/opt/cni/bin",
								},
							},
						},
					},
				},
			},
		},
	}

	existing := &appsv1.DaemonSet{}
	err := k8sClient.Get(ctx, types.NamespacedName{Name: ds.Name, Namespace: ds.Namespace}, existing)
	if err != nil {
		if errors.IsNotFound(err) {
			return k8sClient.Create(ctx, ds)
		}
		return err
	}

	return nil
}

func createSRIOVConfigMap(ctx context.Context, k8sClient client.Client, sriovConfig *kubefabricv1.SRIOVConfig) error {
	resourceName := sriovConfig.ResourceName
	if resourceName == "" {
		resourceName = "intel_sriov_netdevice"
	}

	config := fmt.Sprintf(`{
  "resourceList": [
    {
      "resourceName": "%s",
      "selectors": {
        "pfNames": ["%s"]
      }
    }
  ]
}`, resourceName, sriovConfig.PhysicalInterface)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sriov-config",
			Namespace: SRIOVDevicePluginNamespace,
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

// ConfigureNode configures SR-IOV on a specific node
func ConfigureNode(ctx context.Context, k8sClient client.Client, node *corev1.Node, sriovConfig *kubefabricv1.SRIOVConfig) error {
	if node.Labels == nil {
		node.Labels = make(map[string]string)
	}

	node.Labels["kubefabric.io/sriov"] = "enabled"
	node.Labels[fmt.Sprintf("kubefabric.io/sriov-%s", sriovConfig.ResourceName)] = "true"

	if node.Annotations == nil {
		node.Annotations = make(map[string]string)
	}

	node.Annotations["kubefabric.io/sriov-interface"] = sriovConfig.PhysicalInterface
	node.Annotations["kubefabric.io/sriov-numvfs"] = fmt.Sprintf("%d", sriovConfig.NumVFs)

	return k8sClient.Update(ctx, node)
}

// EnableVFs enables Virtual Functions on the physical interface
// This would typically be done via a node configuration DaemonSet
func EnableVFs(physicalInterface string, numVFs int) error {
	// This is a placeholder - actual implementation would use:
	// echo <numVFs> > /sys/class/net/<physicalInterface>/device/sriov_numvfs
	return fmt.Errorf("VF enablement requires node-level access - deploy SR-IOV network operator or configure manually")
}
