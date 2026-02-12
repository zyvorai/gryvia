package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FabricNetworkSpec defines the desired state of FabricNetwork
type FabricNetworkSpec struct {
	// NetworkType is the type of network (rdma, sriov, standard)
	NetworkType string `json:"networkType"`

	// RDMA configuration for InfiniBand/RoCE
	RDMA *RDMAConfig `json:"rdma,omitempty"`

	// SRIOV configuration
	SRIOV *SRIOVConfig `json:"sriov,omitempty"`

	// NodeSelector for network configuration
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// MTU for network interfaces
	MTU int `json:"mtu,omitempty"`
}

// RDMAConfig defines RDMA network configuration
type RDMAConfig struct {
	// Mode is the RDMA mode (infiniband, roce)
	Mode string `json:"mode"`

	// Devices is the list of RDMA devices
	Devices []string `json:"devices,omitempty"`

	// Subnet for IP assignment
	Subnet string `json:"subnet,omitempty"`

	// Gateway for the network
	Gateway string `json:"gateway,omitempty"`
}

// SRIOVConfig defines SR-IOV configuration
type SRIOVConfig struct {
	// PhysicalInterface is the physical NIC
	PhysicalInterface string `json:"physicalInterface"`

	// NumVFs is the number of virtual functions
	NumVFs int `json:"numVfs"`

	// VFDriver is the driver for VFs
	VFDriver string `json:"vfDriver,omitempty"`

	// ResourceName is the Kubernetes resource name
	ResourceName string `json:"resourceName"`
}

// FabricNetworkStatus defines the observed state of FabricNetwork
type FabricNetworkStatus struct {
	// Phase is the current phase (Pending, Ready, Failed)
	Phase string `json:"phase,omitempty"`

	// Conditions represent the latest available observations
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ConfiguredNodes is the number of nodes configured
	ConfiguredNodes int `json:"configuredNodes,omitempty"`

	// TotalNodes is the total number of matching nodes
	TotalNodes int `json:"totalNodes,omitempty"`

	// LastUpdated is the timestamp of last status update
	LastUpdated metav1.Time `json:"lastUpdated,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster

// FabricNetwork is the Schema for the fabricnetworks API
type FabricNetwork struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FabricNetworkSpec   `json:"spec,omitempty"`
	Status FabricNetworkStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// FabricNetworkList contains a list of FabricNetwork
type FabricNetworkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FabricNetwork `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FabricNetwork{}, &FabricNetworkList{})
}
