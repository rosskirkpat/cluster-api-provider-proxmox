package v1alpha1

import clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

// Conditions and condition Reasons for the ProxmoxCluster object.

const (
	// FailureDomainsAvailableCondition documents the status of the failure domains
	// associated to the ProxmoxCluster.
	FailureDomainsAvailableCondition clusterv1.ConditionType = "FailureDomainsAvailable"

	// FailureDomainsSkippedReason (Severity=Info) documents that some failure domain statuses
	// associated to the ProxmoxCluster are reported as not ready.
	FailureDomainsSkippedReason = "FailureDomainsSkipped"

	// WaitingForFailureDomainStatusReason (Severity=Info) documents that some failure domains
	// associated to the ProxmoxCluster are not reporting the Ready status.
	// Instead of reporting a false ready status, these failure domains are still under the process of reconciling
	// and hence not yet reporting their status.
	WaitingForFailureDomainStatusReason = "WaitingForFailureDomainStatus"
)

// Conditions and condition Reasons for the ProxmoxMachine and the ProxmoxVM object.
//
// NOTE: ProxmoxMachine wraps a ProxmoxVM, some we are using a unique set of conditions and reasons in order
// to ensure a consistent UX; differences between the two objects will be highlighted in the comments.

const (
	// VMProvisionedCondition documents the status of the provisioning of a ProxmoxMachine and its underlying ProxmoxVM.
	VMProvisionedCondition clusterv1.ConditionType = "VMProvisioned"

	// WaitingForClusterInfrastructureReason (Severity=Info) documents a ProxmoxMachine waiting for the cluster
	// infrastructure to be ready before starting the provisioning process.
	//
	// NOTE: This reason does not apply to ProxmoxVM (this state happens before the ProxmoxVM is actually created).
	WaitingForClusterInfrastructureReason = "WaitingForClusterInfrastructure"

	// WaitingForBootstrapDataReason (Severity=Info) documents a ProxmoxMachine waiting for the bootstrap
	// script to be ready before starting the provisioning process.
	//
	// NOTE: This reason does not apply to ProxmoxVM (this state happens before the ProxmoxVM is actually created).
	WaitingForBootstrapDataReason = "WaitingForBootstrapData"

	// WaitingForStaticIPAllocationReason (Severity=Info) documents a ProxmoxVM waiting for the allocation of
	// a static IP address.
	WaitingForStaticIPAllocationReason = "WaitingForStaticIPAllocation"

	// WaitingForIPAllocationReason (Severity=Info) documents a ProxmoxVM waiting for the allocation of
	// an IP address.
	// This is used when the dhcp4 or dhcp6 for a ProxmoxVM is set and the ProxmoxVM is waiting for the
	// relevant IP address  to show up on the VM.
	WaitingForIPAllocationReason = "WaitingForIPAllocation"

	// CloningReason documents (Severity=Info) a ProxmoxMachine/ProxmoxVM currently executing the clone operation.
	CloningReason = "Cloning"

	// CloningFailedReason (Severity=Warning) documents a ProxmoxMachine/ProxmoxVM controller detecting
	// an error while provisioning; those kind of errors are usually transient and failed provisioning
	// are automatically re-tried by the controller.
	CloningFailedReason = "CloningFailed"

	// PoweringOnReason documents (Severity=Info) a ProxmoxMachine/ProxmoxVM currently executing the power on sequence.
	PoweringOnReason = "PoweringOn"

	// PoweringOnFailedReason (Severity=Warning) documents a ProxmoxMachine/ProxmoxVM controller detecting
	// an error while powering on; those kind of errors are usually transient and failed provisioning
	// are automatically re-tried by the controller.
	PoweringOnFailedReason = "PoweringOnFailed"

	// TaskFailure (Severity=Warning) documents a ProxmoxMachine/ProxmoxVM task failure; reconcile loop will automatically
	// retry the operation, but a user intervention might be required to fix the problem.
	TaskFailure = "TaskFailure"

	// WaitingForNetworkAddressesReason (Severity=Info) documents a ProxmoxMachine waiting for the machine network
	// settings to be reported after machine being powered on.
	//
	// NOTE: This reason does not apply to ProxmoxVM (this state happens after the ProxmoxVM is in ready state).
	WaitingForNetworkAddressesReason = "WaitingForNetworkAddresses"

	// TagsAttachmentFailedReason (Severity=Error) documents a ProxmoxMachine/ProxmoxVM tags attachment failure.
	TagsAttachmentFailedReason = "TagsAttachmentFailed"

	// PCIDevicesDetachedCondition documents the status of the attached PCI devices on the ProxmoxVM.
	// It is a negative condition to notify the user that the device(s) is no longer attached to
	// the underlying VM and would require manual intervention to fix the situation.
	//
	// NOTE: This condition does not apply to ProxmoxMachine.
	PCIDevicesDetachedCondition clusterv1.ConditionType = "PCIDevicesDetached"

	// NotFoundReason (Severity=Warning) documents the ProxmoxVM not having the PCI device attached during VM startup.
	// This would indicate that the PCI devices were removed out of band by an external entity.
	NotFoundReason = "NotFound"
)

// Conditions and Reasons related to utilizing a ProxmoxClusterIdentity to make connections to a Proxmox server.
// Can currently be used by ProxmoxCluster and ProxmoxVM.
const (
	// ProxmoxAvailableCondition documents the connectivity with a proxmox server
	// for a given resource.
	ProxmoxAvailableCondition clusterv1.ConditionType = "ProxmoxAvailable"

	// ProxmoxUnreachableReason (Severity=Error) documents a controller detecting
	// issues with Proxmox server reachability.
	ProxmoxUnreachableReason = "ProxmoxUnreachable"
)

const (
	// ClusterFeaturesAvailableCondition documents the availability of cluster features for the ProxmoxCluster object.
	ClusterFeaturesAvailableCondition clusterv1.ConditionType = "ClusterFeaturesAvailable"

	// MissingProxmoxVersionReason (Severity=Warning) documents a controller detecting
	//  the scenario in which the Proxmox server version is not set in the status of the ProxmoxCluster object.
	MissingProxmoxVersionReason = "MissingProxmoxVersion"

	// ProxmoxVersionIncompatibleReason (Severity=Info) documents the case where the proxmox server version of the
	// ProxmoxCluster object does not support cluster modules.
	ProxmoxVersionIncompatibleReason = "ProxmoxVersionIncompatible"

	// ClusterFeatureSetupFailedReason (Severity=Warning) documents a controller detecting
	// issues when setting up anti-affinity constraints via cluster features for objects
	// belonging to the cluster.
	ClusterFeatureSetupFailedReason = "ClusterFeatureSetupFailed"
)

const (
	// CredentialsAvailableCondition is used by ProxmoxClusterIdentity when a credential
	// secret is available and unused by other ProxmoxClusterIdentities.
	CredentialsAvailableCondition clusterv1.ConditionType = "CredentialsAvailable"

	// SecretNotAvailableReason is used when the secret referenced by the ProxmoxClusterIdentity cannot be found.
	SecretNotAvailableReason = "SecretNotAvailable"

	// SecretOwnerReferenceFailedReason is used for errors while updating the owner reference of the secret.
	SecretOwnerReferenceFailedReason = "SecretOwnerReferenceFailed"

	// SecretAlreadyInUseReason is used when another ProxmoxClusterIdentity is using the secret.
	SecretAlreadyInUseReason = "SecretInUse"
)

const (
	// PlacementConstraintMetCondition documents whether the placement constraint is configured correctly or not.
	PlacementConstraintMetCondition clusterv1.ConditionType = "PlacementConstraintMet"

	// ResourcePoolNotFoundReason (Severity=Error) documents that the resource pool in the placement constraint
	// associated to the ProxmoxDeploymentZone is misconfigured.
	ResourcePoolNotFoundReason = "ResourcePoolNotFound"

	// FolderNotFoundReason (Severity=Error) documents that the folder in the placement constraint
	// associated to the ProxmoxDeploymentZone is misconfigured.
	FolderNotFoundReason = "FolderNotFound"
)

const (
	// IPAddressClaimedCondition documents the status of claiming an IP address
	// from an IPAM provider.
	IPAddressClaimedCondition clusterv1.ConditionType = "IPAddressClaimed"

	// IPAddressClaimsBeingCreatedReason (Severity=Info) documents that claims for the
	// IP addresses required by the ProxmoxVM are being created.
	IPAddressClaimsBeingCreatedReason = "IPAddressClaimsBeingCreated"

	// WaitingForIPAddressReason (Severity=Info) documents that the ProxmoxVM is
	// currently waiting for an IP address to be provisioned.
	WaitingForIPAddressReason = "WaitingForIPAddress"

	// IPAddressInvalidReason (Severity=Error) documents that the IP address
	// provided by the IPAM provider is not valid.
	IPAddressInvalidReason = "IPAddressInvalid"

	// IPAddressClaimNotFoundReason (Severity=Error) documents that the IPAddressClaim
	// cannot be found.
	IPAddressClaimNotFoundReason = "IPAddressClaimNotFound"
)
