package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ProxmoxMachineTemplateSpec defines the desired state of ProxmoxMachineTemplate
type ProxmoxMachineTemplateSpec struct {
	Template ProxmoxMachineTemplateResource `json:"template"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=proxmoxmachinetemplates,scope=Namespaced,categories=cluster-api
// +kubebuilder:storageversion

// ProxmoxMachineTemplate is the Schema for the proxmoxmachinetemplates API
type ProxmoxMachineTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ProxmoxMachineTemplateSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// ProxmoxMachineTemplateList contains a list of ProxmoxMachineTemplate
type ProxmoxMachineTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProxmoxMachineTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ProxmoxMachineTemplate{}, &ProxmoxMachineTemplateList{})
}
