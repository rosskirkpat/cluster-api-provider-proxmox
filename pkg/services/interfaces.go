package services

import (
	infrav1 "github.com/rosskirkpat/cluster-api-provider-proxmox/api/v1alpha1"
	"github.com/rosskirkpat/cluster-api-provider-proxmox/pkg/context"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ProxmoxMachineService is used for proxmox VM lifecycle and syncing with ProxmoxMachine types.
type ProxmoxMachineService interface {
	FetchProxmoxMachine(client client.Client, name types.NamespacedName) (context.MachineContext, error)
	FetchProxmoxCluster(client client.Client, cluster *clusterv1.Cluster, machineContext context.MachineContext) (context.MachineContext, error)
	ReconcileDelete(ctx context.MachineContext) error
	SyncFailureReason(ctx context.MachineContext) (bool, error)
	ReconcileNormal(ctx context.MachineContext) (bool, error)
	GetHostInfo(ctx context.MachineContext) (string, error)
}

// VirtualMachineService is a service for creating/updating/deleting virtual
// machines on Proxmox.
type VirtualMachineService interface {
	// ReconcileVM reconciles a VM with the intended state.
	ReconcileVM(ctx *context.VMContext) (infrav1.VirtualMachine, error)

	// DestroyVM powers off and removes a VM from the inventory.
	DestroyVM(ctx *context.VMContext) (infrav1.VirtualMachine, error)
}

// ControlPlaneEndpointService is a service for reconciling load balanced control plane endpoints.
type ControlPlaneEndpointService interface {
	// ReconcileControlPlaneEndpointService manages the lifecycle of a
	// control plane endpoint managed by a VirtualMachineService
	ReconcileControlPlaneEndpointService(ctx *context.ClusterContext, netProvider NetworkProvider) (*clusterv1.APIEndpoint, error)
}

// ResourcePolicyService is a service for reconciling a VirtualMachineSetResourcePolicy for a cluster.
type ResourcePolicyService interface {
	// ReconcileResourcePolicy ensures that a VirtualMachineSetResourcePolicy exists for the cluster
	// Returns the name of a policy if it exists, otherwise returns an error
	ReconcileResourcePolicy(ctx *context.ClusterContext) (string, error)
}

// NetworkProvider provision network resources and configures VM based on network type.
type NetworkProvider interface {
	// HasLoadBalancer indicates whether this provider has a load balancer for Services.
	HasLoadBalancer() bool

	// ProvisionClusterNetwork creates network resource for a given cluster
	// This operation should be idempotent
	ProvisionClusterNetwork(ctx *context.ClusterContext) error

	// GetClusterNetworkName returns the name of a valid cluster network if one exists
	// Returns an empty string if the operation is not supported
	GetClusterNetworkName(ctx *context.ClusterContext) (string, error)

	// GetVMServiceAnnotations returns the annotations, if any, to place on a VM Service.
	GetVMServiceAnnotations(ctx *context.ClusterContext) (map[string]string, error)

	// ConfigureVirtualMachine configures a VM for the particular network
	ConfigureVirtualMachine(ctx *context.ClusterContext, vm *infrav1.VirtualMachine) error

	// VerifyNetworkStatus verifies the status of the network after virtual network creation
	VerifyNetworkStatus(ctx *context.ClusterContext, obj runtime.Object) error
}
