package context

import (
	"fmt"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
	infrav1 "github.com/rosskirkpat/cluster-api-provider-proxmox/api/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
)

type BaseMachineContext struct {
	*ControllerContext
	Logger      logr.Logger
	Cluster     *clusterv1.Cluster
	Machine     *clusterv1.Machine
	PatchHelper *patch.Helper
	Client      client.Client
}

func (c *BaseMachineContext) GetCluster() *clusterv1.Cluster {
	return c.Cluster
}

func (c *BaseMachineContext) GetMachine() *clusterv1.Machine {
	return c.Machine
}

// GetLogger returns this context's logger.
func (c *BaseMachineContext) GetLogger() logr.Logger {
	return c.Logger
}

// PIMMachineContext is a Go context used with a ProxmoxMachine.
type PIMMachineContext struct {
	*BaseMachineContext
	Client         client.Client
	ProxmoxCluster *infrav1.ProxmoxCluster
	ProxmoxMachine *infrav1.ProxmoxMachine
}

// String returns ProxmoxMachineGroupVersionKind ProxmoxMachineNamespace/ProxmoxMachineName.
func (c *PIMMachineContext) String() string {
	return fmt.Sprintf("%s %s/%s", c.ProxmoxMachine.GroupVersionKind(), c.ProxmoxMachine.Namespace, c.ProxmoxMachine.Name)
}

// Patch updates the object and its status on the API server.
func (c *PIMMachineContext) Patch() error {
	return c.PatchHelper.Patch(c, c.ProxmoxMachine)
}

func (c *PIMMachineContext) GetProxmoxMachine() ProxmoxMachine {
	return c.ProxmoxMachine
}

func (c *PIMMachineContext) GetObjectMeta() v1.ObjectMeta {
	return c.ProxmoxMachine.ObjectMeta
}

func (c *PIMMachineContext) SetBaseMachineContext(base *BaseMachineContext) {
	c.BaseMachineContext = base
}

func (c *PIMMachineContext) GetClient() client.Client {
	return c.ControllerContext.Client
}
