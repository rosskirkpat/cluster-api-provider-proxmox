package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/errors"
)

const (
	// VMFinalizer allows the reconciler to clean up resources associated
	// with a ProxmoxVM before removing it from the API Server.
	VMFinalizer = "proxmoxvm.infrastructure.cluster.x-k8s.io"

	// IPAddressClaimFinalizer allows the reconciler to prevent deletion of an
	// IPAddressClaim that is in use.
	IPAddressClaimFinalizer = "proxmoxvm.infrastructure.cluster.x-k8s.io/ip-claim-protection"
)

// ProxmoxVMSpec defines the desired state of ProxmoxVM.
type ProxmoxVMSpec struct {
	VirtualMachineCloneSpec `json:",inline"`

	// BootstrapRef is a reference to a bootstrap provider-specific resource
	// that holds configuration details.
	// This field is optional in case no bootstrap data is required to create
	// a VM.
	// +optional
	BootstrapRef *corev1.ObjectReference `json:"bootstrapRef,omitempty"`

	// VMID is the VM's unique ID that is assigned at runtime after
	// the VM has been created.
	// This field is required at runtime for other controllers that read
	// this CRD as unstructured data.
	// +optional
	VMID string `json:"vmId,omitempty"`

	BiosUUID string `json:"biosUUID"`
}

// ProxmoxVMStatus defines the observed state of ProxmoxVM
type ProxmoxVMStatus struct {
	// Host describes the hostname or IP address of the infrastructure host
	// that the ProxmoxVM is residing on.
	// +optional
	Host string `json:"host,omitempty"`

	// Ready is true when the provider resource is ready.
	// This field is required at runtime for other controllers that read
	// this CRD as unstructured data.
	// +optional
	Ready bool `json:"ready,omitempty"`

	// Addresses is a list of the VM's IP addresses.
	// This field is required at runtime for other controllers that read
	// this CRD as unstructured data.
	// +optional
	Addresses []string `json:"addresses,omitempty"`

	// CloneMode is the type of clone operation used to clone this VM. Since
	// LinkedMode is the default but fails gracefully if the source of the
	// clone has no snapshots, this field may be used to determine the actual
	// type of clone operation used to create this VM.
	// +optional
	CloneMode CloneMode `json:"cloneMode,omitempty"`

	// Snapshot is the name of the snapshot from which the VM was cloned if
	// LinkedMode is enabled.
	// +optional
	Snapshot string `json:"snapshot,omitempty"`

	// RetryAfter tracks the time we can retry queueing a task
	// +optional
	RetryAfter metav1.Time `json:"retryAfter,omitempty"`

	// TaskRef is a proxmox reference to a Task related to the machine.
	// This value is set automatically at runtime and should not be set or
	// modified by users.
	// +optional
	TaskRef string `json:"taskRef,omitempty"`

	// VmIdRef is a proxmox reference to a VM ID related to the machine.
	// This value is set automatically at runtime and should not be set or
	// modified by users.
	// +optional
	VmIdRef int `json:"vmIdRef,omitempty"`

	// Network returns the network status for each of the machine's configured
	// network interfaces.
	// +optional
	Network []NetworkStatus `json:"network,omitempty"`

	// FailureReason will be set in the event that there is a terminal problem
	// reconciling the proxmoxvm and will contain a succinct value suitable
	// for vm interpretation.
	//
	// This field should not be set for transitive errors that a controller
	// faces that are expected to be fixed automatically over
	// time (like service outages), but instead indicate that something is
	// fundamentally wrong with the vm.
	//
	// Any transient errors that occur during the reconciliation of proxmoxvms
	// can be added as events to the proxmoxvm object and/or logged in the
	// controller's output.
	// +optional
	FailureReason *errors.MachineStatusError `json:"failureReason,omitempty"`

	// FailureMessage will be set in the event that there is a terminal problem
	// reconciling the proxmoxvm and will contain a more verbose string suitable
	// for logging and human consumption.
	//
	// This field should not be set for transitive errors that a controller
	// faces that are expected to be fixed automatically over
	// time (like service outages), but instead indicate that something is
	// fundamentally wrong with the vm.
	//
	// Any transient errors that occur during the reconciliation of proxmoxvms
	// can be added as events to the proxmoxvm object and/or logged in the
	// controller's output.
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// Conditions defines current service state of the ProxmoxVM.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`

	// ModuleUUID is the unique identifier for the Proxmox cluster module construct
	// which is used to configure anti-affinity. Objects with the same ModuleUUID
	// will be anti-affined, meaning that Proxmox will best effort schedule
	// the VMs on separate hosts.
	// +optional
	ModuleUUID *string `json:"moduleUUID,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=proxmoxvms,scope=Namespaced,categories=cluster-api
// +kubebuilder:storageversion
// +kubebuilder:subresource:status

// ProxmoxVM is the Schema for the proxmoxvms API
type ProxmoxVM struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProxmoxVMSpec   `json:"spec,omitempty"`
	Status ProxmoxVMStatus `json:"status,omitempty"`
}

func (r *ProxmoxVM) GetConditions() clusterv1.Conditions {
	return r.Status.Conditions
}

func (r *ProxmoxVM) SetConditions(conditions clusterv1.Conditions) {
	r.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// ProxmoxVMList contains a list of ProxmoxVM
type ProxmoxVMList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProxmoxVM `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ProxmoxVM{}, &ProxmoxVMList{})
}
