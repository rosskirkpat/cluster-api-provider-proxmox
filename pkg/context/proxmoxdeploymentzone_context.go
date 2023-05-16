package context

import (
	"fmt"

	"github.com/go-logr/logr"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"

	infrav1 "github.com/rosskirkpat/cluster-api-provider-proxmox/api/v1alpha1"
	"github.com/rosskirkpat/cluster-api-provider-proxmox/pkg/session"
)

type ProxmoxDeploymentZoneContext struct {
	*ControllerContext
	ProxmoxDeploymentZone *infrav1.ProxmoxDeploymentZone
	ProxmoxFailureDomain  *infrav1.ProxmoxFailureDomain
	Logger                logr.Logger
	PatchHelper           *patch.Helper
	AuthSession           *session.Session
}

func (c *ProxmoxDeploymentZoneContext) Patch() error {
	conditions.SetSummary(c.ProxmoxDeploymentZone,
		conditions.WithConditions(
			infrav1.ProxmoxAvailableCondition,
			infrav1.ProxmoxFailureDomainValidatedCondition,
			infrav1.PlacementConstraintMetCondition,
		),
	)
	return c.PatchHelper.Patch(c, c.ProxmoxDeploymentZone)
}

func (c *ProxmoxDeploymentZoneContext) String() string {
	return fmt.Sprintf("%s %s", c.ProxmoxDeploymentZone.GroupVersionKind(), c.ProxmoxDeploymentZone.Name)
}

func (c *ProxmoxDeploymentZoneContext) GetSession() *session.Session {
	return c.AuthSession
}

func (c *ProxmoxDeploymentZoneContext) GetProxmoxFailureDomain() infrav1.ProxmoxFailureDomain {
	return *c.ProxmoxFailureDomain
}
