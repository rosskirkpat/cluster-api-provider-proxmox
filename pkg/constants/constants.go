package constants

import (
	"time"

	infrav1 "github.com/rosskirkpat/cluster-api-provider-proxmox/api/v1alpha1"
)

const (
	// CloudProviderSecretName is the name of the Secret that stores the
	// cloud provider credentials.
	CloudProviderSecretName = "cloud-provider-proxmox-credentials"

	// CloudProviderSecretNamespace is the namespace in which the cloud provider
	// credentials secret is located.
	CloudProviderSecretNamespace = "kube-system"

	// DefaultBindPort is the default API port used to generate the kubeadm
	// configurations.
	DefaultBindPort = 6443

	// ProxmoxCredentialSecretUserKey is the key used to store/retrieve the
	// Proxmox username from a Kubernetes secret.
	ProxmoxCredentialSecretUserKey = "username"

	// ProxmoxCredentialSecretPassKey is the key used to store/retrieve the
	// Proxmox password from a Kubernetes secret.
	ProxmoxCredentialSecretPassKey = "password"

	// MachineReadyAnnotationLabel is the annotation used to indicate that a
	// machine is ready.
	MachineReadyAnnotationLabel = "capv." + infrav1.GroupName + "/machine-ready"

	// MaintenanceAnnotationLabel is the annotation used to indicate a machine and/or
	// cluster are in maintenance mode.
	MaintenanceAnnotationLabel = "capv." + infrav1.GroupName + "/maintenance"

	// DefaultEnableKeepAlive is true by default.
	DefaultEnableKeepAlive = true

	// DefaultKeepAliveDuration unit minutes.
	DefaultKeepAliveDuration = time.Minute * 5

	NodeLabelPrefix = "node.cluster.x-k8s.io"

	ProxmoxHostInfoLabel = NodeLabelPrefix + "/proxmox-host"
)
