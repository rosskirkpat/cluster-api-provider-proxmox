package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type FailureDomainType string

const (
	HostGroupFailureDomain      FailureDomainType = "HostGroup"
	ComputeClusterFailureDomain FailureDomainType = "ComputeCluster"
	DatacenterFailureDomain     FailureDomainType = "Datacenter"
)

// ProxmoxFailureDomainSpec defines the desired state of ProxmoxFailureDomain
type ProxmoxFailureDomainSpec struct {

	// Region defines the name and type for a region
	Region FailureDomain `json:"region"`

	// Zone defines the name and type for a zone
	Zone FailureDomain `json:"zone"`

	// Topology describes a given failure domain using Proxmox constructs
	Topology Topology `json:"topology"`
}

type FailureDomain struct {
	// Name is the name of the pool that represents this failure domain
	Name string `json:"name"`

	// Type is the type of failure domain, the current values are "Datacenter", "ComputeCluster" and "HostGroup"
	// +kubebuilder:validation:Enum=Datacenter;ComputeCluster;HostGroup
	Type FailureDomainType `json:"type"`

	// PoolCategory is the category used for the pool
	PoolCategory string `json:"poolCategory"`

	// AutoConfigure tags the Type which is specified in the Topology
	AutoConfigure *bool `json:"autoConfigure,omitempty"`
}

type Topology struct {
	// Datacenter as the failure domain.
	// +kubebuilder:validation:Required
	Datacenter string `json:"datacenter"`

	// ComputeCluster as the failure domain
	// +optional
	ComputeCluster *string `json:"computeCluster,omitempty"`

	// Hosts has information required for placement of machines on Proxmox hosts.
	// +optional
	Hosts *FailureDomainHosts `json:"hosts,omitempty"`

	// Networks is the list of networks within this failure domain
	// +optional
	Networks []string `json:"networks,omitempty"`

	// Datastore is the name or inventory path of the datastore in which the
	// virtual machine is created/located.
	// +optional
	Datastore string `json:"datastore,omitempty"`
}

type FailureDomainHosts struct {
	// ClusterVMGroupName is the name of the Cluster VM group (a Proxmox node)
	ClusterVMGroupName string `json:"clusterCMGroupName"`

	// HAGroupName is the name of the HA group (HA-configured Proxmox Cluster)
	HAGroupName string `json:"haGroupName"`

	// Pool is the name of the Proxmox Cluster pool
	Pool string `json:"pool,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:resource:path=proxmoxfailuredomains,scope=Cluster,categories=cluster-api

// ProxmoxFailureDomain is the Schema for the proxmoxfailuredomains API
type ProxmoxFailureDomain struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ProxmoxFailureDomainSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// ProxmoxFailureDomainList contains a list of ProxmoxFailureDomain
type ProxmoxFailureDomainList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProxmoxFailureDomain `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ProxmoxFailureDomain{}, &ProxmoxFailureDomainList{})
}
