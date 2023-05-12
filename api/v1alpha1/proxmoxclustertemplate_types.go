package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// ProxmoxClusterTemplateSpec defines the desired state of ProxmoxClusterTemplate
type ProxmoxClusterTemplateSpec struct {
	Template ProxmoxClusterTemplateResource `json:"template"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=proxmoxclustertemplates,scope=Namespaced,categories=cluster-api
// +kubebuilder:storageversion

// ProxmoxClusterTemplate is the Schema for the proxmoxclustertemplates API
type ProxmoxClusterTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ProxmoxClusterTemplateSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// ProxmoxClusterTemplateList contains a list of ProxmoxClusterTemplate.
type ProxmoxClusterTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProxmoxClusterTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ProxmoxClusterTemplate{}, &ProxmoxClusterTemplateList{})
}

type ProxmoxClusterTemplateResource struct {
	Spec ProxmoxClusterSpec `json:"spec"`
}
