/*
Copyright 2023 Ross Kirkpatrick.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	// ClusterFinalizer allows ReconcileProxmoxCluster to clean up Proxmox
	// resources associated with ProxmoxCluster before removing it from the
	// API server.
	ClusterFinalizer = "proxmoxcluster.infrastructure.cluster.x-k8s.io"
)

// ProxmoxVersion conveys the API version of the Proxmox instance.
type ProxmoxVersion string

func NewProxmoxVersion(version string) ProxmoxVersion {
	return ProxmoxVersion(version)
}

// ProxmoxClusterSpec defines the desired state of ProxmoxCluster
type ProxmoxClusterSpec struct {
	// Server is the address of the Proxmox endpoint.
	Server string `json:"server,omitempty"`

	// Thumbprint is the colon-separated SHA-1 checksum of the given Proxmox server's host certificate
	// +optional
	Thumbprint string `json:"thumbprint,omitempty"`

	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	// +optional
	ControlPlaneEndpoint APIEndpoint `json:"controlPlaneEndpoint"`

	// IdentityRef is a reference to either a Secret or ProxmoxClusterIdentity that contains
	// the identity to use when reconciling the cluster.
	// +optional
	IdentityRef *ProxmoxIdentityReference `json:"identityRef,omitempty"`

	// FailureDomainSelector is the label selector to use for failure domain selection
	// for the control plane nodes of the cluster.
	// An empty value for the selector includes all the related failure domains.
	// +optional
	FailureDomainSelector *metav1.LabelSelector `json:"failureDomainSelector,omitempty"`
}

// ProxmoxClusterStatus defines the observed state of ProxmoxCluster
type ProxmoxClusterStatus struct {
	// Ready denotes that the proxmox cluster (infrastructure) is ready.
	// +optional
	Ready bool `json:"ready"`

	// Conditions defines current service state of the ProxmoxCluster.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`

	// FailureDomains is a list of failure domain objects synced from the infrastructure provider.
	FailureDomains clusterv1.FailureDomains `json:"failureDomains,omitempty"`

	// ProxmoxVersion defines the version of the Proxmox server defined in the spec.
	ProxmoxVersion ProxmoxVersion `json:"proxmoxVersion,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:resource:path=proxmoxclusters,scope=Namespaced,categories=cluster-api
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="Cluster infrastructure is ready for ProxmoxMachine"
// +kubebuilder:printcolumn:name="Server",type="string",JSONPath=".spec.server",description="Server is the address of the Proxmox endpoint."
// +kubebuilder:printcolumn:name="ControlPlaneEndpoint",type="string",JSONPath=".spec.controlPlaneEndpoint[0]",description="API Endpoint",priority=1
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of Machine"

// ProxmoxCluster is the Schema for the proxmoxclusters API
type ProxmoxCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProxmoxClusterSpec   `json:"spec,omitempty"`
	Status ProxmoxClusterStatus `json:"status,omitempty"`
}

func (c *ProxmoxCluster) GetConditions() clusterv1.Conditions {
	return c.Status.Conditions
}

func (c *ProxmoxCluster) SetConditions(conditions clusterv1.Conditions) {
	c.Status.Conditions = conditions
}

//+kubebuilder:object:root=true

// ProxmoxClusterList contains a list of ProxmoxCluster
type ProxmoxClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProxmoxCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ProxmoxCluster{}, &ProxmoxClusterList{})
}
