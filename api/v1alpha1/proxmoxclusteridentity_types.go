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
	SecretIdentitySetFinalizer = "proxmoxcluster/infrastructure.cluster.x-k8s.io"
)

type ProxmoxClusterIdentitySpec struct {
	// SecretName references a Secret inside the controller namespace with the credentials to use
	// +kubebuilder:validation:MinLength=1
	SecretName string `json:"secretName,omitempty"`

	// AllowedNamespaces is used to identify which namespaces are allowed to use this account.
	// Namespaces can be selected with a label selector.
	// If this object is nil, no namespaces will be allowed
	// +optional
	AllowedNamespaces *AllowedNamespaces `json:"allowedNamespaces,omitempty"`
}

type ProxmoxClusterIdentityStatus struct {
	// +optional
	Ready bool `json:"ready,omitempty"`

	// Conditions defines current service state of the ProxmoxCluster.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

type AllowedNamespaces struct {
	// Selector is a standard Kubernetes LabelSelector. A label query over a set of resources.
	// +optional
	Selector metav1.LabelSelector `json:"selector"`
}

type ProxmoxIdentityKind string

var (
	ProxmoxClusterIdentityKind = ProxmoxIdentityKind("ProxmoxClusterIdentity")
	SecretKind                 = ProxmoxIdentityKind("Secret")
)

type ProxmoxIdentityReference struct {
	// Kind of the identity. Can either be ProxmoxClusterIdentity or Secret
	// +kubebuilder:validation:Enum=ProxmoxClusterIdentity;Secret
	Kind ProxmoxIdentityKind `json:"kind"`

	// Name of the identity.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

func (c *ProxmoxClusterIdentity) GetConditions() clusterv1.Conditions {
	return c.Status.Conditions
}

func (c *ProxmoxClusterIdentity) SetConditions(conditions clusterv1.Conditions) {
	c.Status.Conditions = conditions
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=proxmoxclusteridentities,scope=Cluster,categories=cluster-api
// +kubebuilder:storageversion
// +kubebuilder:subresource:status

// ProxmoxClusterIdentity defines the account to be used for reconciling clusters
type ProxmoxClusterIdentity struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProxmoxClusterIdentitySpec   `json:"spec,omitempty"`
	Status ProxmoxClusterIdentityStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ProxmoxClusterIdentityList contains a list of ProxmoxClusterIdentity
type ProxmoxClusterIdentityList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProxmoxClusterIdentity `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ProxmoxClusterIdentity{}, &ProxmoxClusterIdentityList{})
}
