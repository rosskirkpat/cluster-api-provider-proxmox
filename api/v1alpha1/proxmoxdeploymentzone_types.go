package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	// DeploymentZoneFinalizer allows ReconcileProxmoxDeploymentZone to
	// check for dependents associated with ProxmoxDeploymentZone
	// before removing it from the API Server.
	DeploymentZoneFinalizer = "proxmoxdeploymentzone.infrastructure.cluster.x-k8s.io"
)

// ProxmoxDeploymentZoneSpec defines the desired state of ProxmoxDeploymentZone
type ProxmoxDeploymentZoneSpec struct {

	// Server is the address of the Proxmox server endpoint.
	Server string `json:"server,omitempty"`

	// FailureDomain is the name of the ProxmoxFailureDomain used for this ProxmoxDeploymentZone
	FailureDomain string `json:"failureDomain,omitempty"`

	// ControlPlane determines if this failure domain is suitable for use by control plane machines.
	// +optional
	ControlPlane *bool `json:"controlPlane,omitempty"`

	// PlacementConstraint encapsulates the placement constraints
	// used within this deployment zone.
	PlacementConstraint PlacementConstraint `json:"placementConstraint"`
}

// PlacementConstraint is the context information for VM placements within a failure domain
type PlacementConstraint struct {
	// ResourcePool is the name or inventory path of the resource pool in which
	// the virtual machine is created/located.
	// +optional
	ResourcePool string `json:"resourcePool,omitempty"`

	// Folder is the name or inventory path of the folder in which the
	// virtual machine is created/located.
	// +optional
	Folder string `json:"folder,omitempty"`
}

type Network struct {
	// Name is the network name for this machine's VM.
	Name string `json:"name,omitempty"`

	// DHCP4 is a flag that indicates whether to use DHCP for IPv4
	// +optional
	DHCP4 *bool `json:"dhcp4,omitempty"`

	// DHCP6 indicates whether to use DHCP for IPv6
	// +optional
	DHCP6 *bool `json:"dhcp6,omitempty"`
}

type ProxmoxDeploymentZoneStatus struct {
	// Ready is true when the ProxmoxDeploymentZone resource is ready.
	// If set to false, it will be ignored by ProxmoxClusters
	// +optional
	Ready *bool `json:"ready,omitempty"`

	// Conditions defines current service state of the ProxmoxMachine.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:resource:path=proxmoxdeploymentzones,scope=Cluster,categories=cluster-api
// +kubebuilder:subresource:status

// ProxmoxDeploymentZone is the Schema for the proxmoxdeploymentzones API
type ProxmoxDeploymentZone struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProxmoxDeploymentZoneSpec   `json:"spec,omitempty"`
	Status ProxmoxDeploymentZoneStatus `json:"status,omitempty"`
}

func (z *ProxmoxDeploymentZone) GetConditions() clusterv1.Conditions {
	return z.Status.Conditions
}

func (z *ProxmoxDeploymentZone) SetConditions(conditions clusterv1.Conditions) {
	z.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// ProxmoxDeploymentZoneList contains a list of ProxmoxDeploymentZone
type ProxmoxDeploymentZoneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProxmoxDeploymentZone `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ProxmoxDeploymentZone{}, &ProxmoxDeploymentZoneList{})
}
