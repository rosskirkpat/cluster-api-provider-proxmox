package context

import (
	"fmt"
	"github.com/go-logr/logr"
	infrav1 "github.com/rosskirkpat/cluster-api-provider-proxmox/api/v1alpha1"
	"github.com/rosskirkpat/cluster-api-provider-proxmox/pkg/session"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// VMContext is a Go context used with a ProxmoxVM.
type VMContext struct {
	*ControllerContext
	ProxmoxVM            *infrav1.ProxmoxVM
	PatchHelper          *patch.Helper
	Logger               logr.Logger
	Session              *session.Session
	ProxmoxFailureDomain *infrav1.ProxmoxFailureDomain
}

// String returns ProxmoxVMGroupVersionKind ProxmoxVMNamespace/ProxmoxVMName.
func (c *VMContext) String() string {
	return fmt.Sprintf("%s %s/%s", c.ProxmoxVM.GroupVersionKind(), c.ProxmoxVM.Namespace, c.ProxmoxVM.Name)
}

// Patch updates the object and its status on the API server.
func (c *VMContext) Patch() error {
	return c.PatchHelper.Patch(c, c.ProxmoxVM)
}

// GetLogger returns this context's logger.
func (c *VMContext) GetLogger() logr.Logger {
	return c.Logger
}

// GetSession returns this context's session.
func (c *VMContext) GetSession() *session.Session {
	return c.Session
}

func (c *VMContext) GetClient() client.Client {
	return c.ControllerContext.Client
}
