package util

import (
	"context"

	"github.com/pkg/errors"
	infrav1 "github.com/rosskirkpat/cluster-api-provider-proxmox/api/v1alpha1"
	apitypes "k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetProxmoxClusterFromProxmoxMachine gets the proxmox.infrastructure.cluster.x-k8s.io.ProxmoxCluster resource for the given ProxmoxMachine.
func GetProxmoxClusterFromProxmoxMachine(ctx context.Context, c client.Client, machine *infrav1.ProxmoxMachine) (*infrav1.ProxmoxCluster, error) {
	clusterName := machine.Labels[clusterv1.ClusterNameLabel]
	if clusterName == "" {
		return nil, errors.Errorf("error getting ProxmoxCluster name from ProxmoxMachine %s/%s",
			machine.Namespace, machine.Name)
	}
	namespacedName := apitypes.NamespacedName{
		Namespace: machine.Namespace,
		Name:      clusterName,
	}
	cluster := &clusterv1.Cluster{}
	if err := c.Get(ctx, namespacedName, cluster); err != nil {
		return nil, err
	}

	proxmoxClusterKey := apitypes.NamespacedName{
		Namespace: machine.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	proxmoxCluster := &infrav1.ProxmoxCluster{}
	err := c.Get(ctx, proxmoxClusterKey, proxmoxCluster)
	return proxmoxCluster, err
}
