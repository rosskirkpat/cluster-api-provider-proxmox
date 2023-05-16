package controllers

import (
	"github.com/luthermonson/go-proxmox"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/util/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	ctrl "sigs.k8s.io/controller-runtime"

	infrav1 "github.com/rosskirkpat/cluster-api-provider-proxmox/api/v1alpha1"
	"github.com/rosskirkpat/cluster-api-provider-proxmox/pkg/context"
)

func (r proxmoxDeploymentZoneReconciler) reconcileFailureDomain(ctx *context.ProxmoxDeploymentZoneContext) error {
	logger := ctrl.LoggerFrom(ctx).WithValues("failure domain", ctx.ProxmoxFailureDomain.Name)

	// verify the failure domain for the region
	if err := r.reconcileInfraFailureDomain(ctx, ctx.ProxmoxFailureDomain.Spec.Region); err != nil {
		conditions.MarkFalse(ctx.ProxmoxDeploymentZone, infrav1.ProxmoxFailureDomainValidatedCondition, infrav1.RegionMisconfiguredReason, clusterv1.ConditionSeverityError, err.Error())
		logger.Error(err, "region is not configured correctly")
		return errors.Wrapf(err, "region is not configured correctly")
	}

	// verify the failure domain for the zone
	if err := r.reconcileInfraFailureDomain(ctx, ctx.ProxmoxFailureDomain.Spec.Zone); err != nil {
		conditions.MarkFalse(ctx.ProxmoxDeploymentZone, infrav1.ProxmoxFailureDomainValidatedCondition, infrav1.ZoneMisconfiguredReason, clusterv1.ConditionSeverityError, err.Error())
		logger.Error(err, "zone is not configured correctly")
		return errors.Wrapf(err, "zone is not configured correctly")
	}

	if computeCluster := ctx.ProxmoxFailureDomain.Spec.Topology.ComputeCluster; computeCluster != nil {
		if err := r.reconcileComputeCluster(ctx); err != nil {
			logger.Error(err, "compute cluster is not configured correctly", "name", *computeCluster)
			return errors.Wrap(err, "compute cluster is not configured correctly")
		}
	}

	if err := r.reconcileTopology(ctx); err != nil {
		logger.Error(err, "topology is not configured correctly")
		return errors.Wrap(err, "topology is not configured correctly")
	}
	return nil
}

func (r proxmoxDeploymentZoneReconciler) reconcileInfraFailureDomain(ctx *context.ProxmoxDeploymentZoneContext, failureDomain infrav1.FailureDomain) error {
	if *failureDomain.AutoConfigure {
		return r.createAndAttachMetadata(ctx, failureDomain)
	}
	return r.verifyFailureDomain(ctx, failureDomain)
}

func (r proxmoxDeploymentZoneReconciler) reconcileTopology(ctx *context.ProxmoxDeploymentZoneContext) error {
	topology := ctx.ProxmoxFailureDomain.Spec.Topology
	if datastore := topology.Datastore; datastore != "" {
		storage := proxmox.Storage{}
		if err := ctx.AuthSession.Get(datastore, storage); err != nil {
			conditions.MarkFalse(ctx.ProxmoxDeploymentZone, infrav1.ProxmoxFailureDomainValidatedCondition, infrav1.DatastoreNotFoundReason, clusterv1.ConditionSeverityError, "datastore %s is misconfigured", datastore)
			return errors.Wrapf(err, "unable to find cluster datastore %s", datastore)
		}
	}

	for _, network := range topology.Networks {
		nodeNetwork := proxmox.NodeNetwork{}
		if err := ctx.AuthSession.Get(network, nodeNetwork); err != nil {
			conditions.MarkFalse(ctx.ProxmoxDeploymentZone, infrav1.ProxmoxFailureDomainValidatedCondition, infrav1.NetworkNotFoundReason, clusterv1.ConditionSeverityError, "network %s is misconfigured", network)
			return errors.Wrapf(err, "unable to find node network %s", network)
		}
	}

	if hostPlacementInfo := topology.Hosts; hostPlacementInfo != nil {
		// FailureDomainHosts refers to nodes in HA Proxmox Group
		// TODO (go-proxmox) add methods to proxmox cluster to return permission-based pools for a cluster
		// TODO (go-proxmox) add HA Groups and Fencing types
		// TODO (go-proxmox) add methods to HA groups + fencing to return groups and fencing for a cluster
		cluster := proxmox.Cluster{}
		if err := ctx.AuthSession.Get(*topology.ComputeCluster, cluster); err != nil {
			conditions.MarkFalse(ctx.ProxmoxDeploymentZone, infrav1.ProxmoxFailureDomainValidatedCondition, infrav1.ComputeClusterNotFoundReason, clusterv1.ConditionSeverityError, "compute cluster %s is misconfigured", cluster.Name)
			return errors.Wrapf(err, "unable to find compute cluster %s", *topology.ComputeCluster)
		}
		// switch cluster.HA.Groups
		// switch cluster.HA.Fencing
		// switch cluster.Permissions.Pools
		switch cluster.Nodes {
		case nil:
			ctrl.LoggerFrom(ctx).V(4).Info("warning: HA group for the failure domain is disabled", "cluster", hostPlacementInfo.ClusterVMGroupName, "haGroup", hostPlacementInfo.HAGroupName)
			conditions.MarkFalse(ctx.ProxmoxDeploymentZone, infrav1.ProxmoxFailureDomainValidatedCondition, infrav1.HostsMisconfiguredReason, clusterv1.ConditionSeverityError, "vm host affinity does not exist")
		default:
			conditions.MarkTrue(ctx.ProxmoxDeploymentZone, infrav1.ProxmoxFailureDomainValidatedCondition)
		}
	}
	return nil
}

func (r proxmoxDeploymentZoneReconciler) reconcileComputeCluster(ctx *context.ProxmoxDeploymentZoneContext) error {
	computeCluster := ctx.ProxmoxFailureDomain.Spec.Topology.ComputeCluster
	if computeCluster == nil {
		return nil
	}

	cluster := proxmox.Cluster{}
	if err := ctx.AuthSession.Get(*computeCluster, cluster); err != nil {
		conditions.MarkFalse(ctx.ProxmoxDeploymentZone, infrav1.ProxmoxFailureDomainValidatedCondition, infrav1.ComputeClusterNotFoundReason, clusterv1.ConditionSeverityError, "compute cluster %s is misconfigured", cluster.Name)
		return errors.Wrapf(err, "unable to find compute cluster %s", *computeCluster)
	}

	if resourcePool := ctx.ProxmoxDeploymentZone.Spec.PlacementConstraint.ResourcePool; resourcePool != "" {
		rpc := proxmox.Cluster{}
		if err := ctx.AuthSession.Get(resourcePool, rpc); err != nil {
			return errors.Wrapf(err, "unable to find cluster resource pool")
		}

		//ref, err := rpc.Owner(ctx)
		//if err != nil {
		//	conditions.MarkFalse(ctx.ProxmoxDeploymentZone, infrav1.ProxmoxFailureDomainValidatedCondition, infrav1.ComputeClusterNotFoundReason, clusterv1.ConditionSeverityError, "cluster resource pool owner not found")
		//	return errors.Wrap(err, "unable to find owner compute resource")
		//}

		if rpc.ID != cluster.ID {
			conditions.MarkFalse(ctx.ProxmoxDeploymentZone, infrav1.ProxmoxFailureDomainValidatedCondition, infrav1.ResourcePoolNotFoundReason, clusterv1.ConditionSeverityError, "resource pool is not owned by compute cluster")
			return errors.Errorf("compute cluster %s does not own resource pool %s", *computeCluster, resourcePool)
		}
	}
	return nil
}

// verifyFailureDomain verifies the Failure Domain. It verifies the existence of tag and pool specified and
// checks whether the specified tags exist on the DataCenter or Compute Cluster or Hosts (in a HostGroup).
func (r proxmoxDeploymentZoneReconciler) verifyFailureDomain(ctx *context.ProxmoxDeploymentZoneContext, failureDomain infrav1.FailureDomain) error {
	//pool := proxmox.Pool{}
	//if _, err := ctx.AuthSession.Get(failureDomain.Name, pool); err != nil {
	//	return errors.Wrapf(err, "failed to verify tag %s and pool %s", failureDomain.Name, failureDomain.PoolCategory)
	//}

	resources := proxmox.ClusterResources{}
	err := ctx.AuthSession.Get(string(failureDomain.Type), resources)
	if err != nil {
		return errors.Wrapf(err, "failed to find resources")
	}

	// All the resources should be associated to the pool
	for _, res := range resources {
		if res.Pool != failureDomain.Name {
			return errors.Errorf("pool %s is not associated to resource %s", failureDomain.Name, res.Name)
		}
	}
	return nil
}

func (r proxmoxDeploymentZoneReconciler) createAndAttachMetadata(ctx *context.ProxmoxDeploymentZoneContext, failureDomain infrav1.FailureDomain) error {
	logger := ctrl.LoggerFrom(ctx, "pool", failureDomain.Name, "category", failureDomain.PoolCategory)
	// TODO (go-proxmox) add function to create pools
	//poolID, err := proxmox.CreatePool(ctx, failureDomain.PoolCategory, failureDomain.Type)
	//if err != nil {
	//	logger.V(4).Error(err, "pool creation failed")
	//	return errors.Wrapf(err, "failed to create pool %s", failureDomain.PoolCategory)
	//}
	// TODO create and store tags in ProxmoxDeploymentZoneContext
	// TODO (go-proxmox) create function to update tags in QEMU VM configuration
	// GET /api2/json/nodes/{node}/qemu/{vmid}/config
	// PUT /api2/json/nodes/{node}/qemu/{vmid}/config
	//	err = proxmox.CreateTagForVM(ctx, failureDomain.Name, categoryID)
	//if err != nil {
	//	logger.V(4).Error(err, "tag creation failed")
	//	return errors.Wrapf(err, "failed to create tag %s", failureDomain.Name)
	//}

	logger = logger.WithValues("type", failureDomain.Type)
	resources := proxmox.ClusterResources{}
	err := ctx.AuthSession.Get(string(failureDomain.Type), resources)
	if err != nil {
		return errors.Wrapf(err, "failed to find resources")
	}
	//
	var errList []error
	for _, res := range resources {
		logger.V(4).Info("attaching pool to object")
		// TODO (go-proxmox) add function to attach pools to cluster resources
		//err := res.AttachPool(ctx, failureDomain.Name)
		if err != nil {
			logger.V(4).Error(err, "failed to find resource", "resource", res.Name)
			errList = append(errList, errors.Wrapf(err, "failed to attach pool"))
		}
	}
	return apierrors.NewAggregate(errList)
}
