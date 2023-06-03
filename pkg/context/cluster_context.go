package context

import (
	"fmt"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
	infrav1 "github.com/rosskirkpat/cluster-api-provider-proxmox/api/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
)

// ClusterContext is a Go context used with a ProxmoxCluster.
type ClusterContext struct {
	*ControllerContext
	Client         client.Client
	Cluster        *clusterv1.Cluster
	ProxmoxCluster *infrav1.ProxmoxCluster
	PatchHelper    *patch.Helper
	Logger         logr.Logger
}

// String returns ProxmoxClusterGroupVersionKind ProxmoxClusterNamespace/ProxmoxClusterName.
func (c *ClusterContext) String() string {
	return fmt.Sprintf("%s %s/%s", c.ProxmoxCluster.GroupVersionKind(), c.ProxmoxCluster.Namespace, c.ProxmoxCluster.Name)
}

// Patch updates the object and its status on the API server.
func (c *ClusterContext) Patch() error {
	// always update the readyCondition.
	conditions.SetSummary(c.ProxmoxCluster,
		conditions.WithConditions(
			infrav1.ProxmoxAvailableCondition,
		),
	)

	return c.PatchHelper.Patch(c, c.ProxmoxCluster)
}
