package controllers

import (
	goctx "context"
	"fmt"
	"github.com/luthermonson/go-proxmox"
	"reflect"
	"strings"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	clusterutilv1 "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/collections"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	infrav1 "github.com/rosskirkpat/cluster-api-provider-proxmox/api/v1alpha1"
	"github.com/rosskirkpat/cluster-api-provider-proxmox/pkg/context"
	"github.com/rosskirkpat/cluster-api-provider-proxmox/pkg/identity"
	"github.com/rosskirkpat/cluster-api-provider-proxmox/pkg/record"
	"github.com/rosskirkpat/cluster-api-provider-proxmox/pkg/session"
	"github.com/rosskirkpat/cluster-api-provider-proxmox/pkg/util"
)

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=proxmoxdeploymentzones,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=proxmoxdeploymentzones/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=proxmoxfailuredomains,verbs=get;list;watch;create;update;patch;delete

// AddProxmoxDeploymentZoneControllerToManager adds the ProxmoxDeploymentZone controller to the provided manager.
func AddProxmoxDeploymentZoneControllerToManager(ctx *context.ControllerContext, mgr manager.Manager) error {
	var (
		controlledType     = &infrav1.ProxmoxDeploymentZone{}
		controlledTypeName = reflect.TypeOf(controlledType).Elem().Name()
		controlledTypeGVK  = infrav1.GroupVersion.WithKind(controlledTypeName)

		controllerNameShort = fmt.Sprintf("%s-controller", strings.ToLower(controlledTypeName))
		controllerNameLong  = fmt.Sprintf("%s/%s/%s", ctx.Namespace, ctx.Name, controllerNameShort)
	)

	// Build the controller context.
	controllerContext := &context.ControllerContext{
		Context:  ctx,
		Name:     controllerNameShort,
		Recorder: record.New(mgr.GetEventRecorderFor(controllerNameLong)),
		Logger:   ctx.Logger.WithName(controllerNameShort),
	}
	reconciler := proxmoxDeploymentZoneReconciler{ControllerContext: controllerContext}

	return ctrl.NewControllerManagedBy(mgr).
		// Watch the controlled, infrastructure resource.
		For(controlledType).
		Watches(
			&source.Kind{Type: &infrav1.ProxmoxFailureDomain{}},
			handler.EnqueueRequestsFromMapFunc(reconciler.failureDomainsToDeploymentZones)).
		// Watch a GenericEvent channel for the controlled resource.
		// This is useful when there are events outside of Kubernetes that
		// should cause a resource to be synchronized, such as a goroutine
		// waiting on some asynchronous, external task to complete.
		Watches(
			&source.Channel{Source: ctx.GetGenericEventChannelFor(controlledTypeGVK)},
			&handler.EnqueueRequestForObject{},
		).
		WithOptions(controller.Options{MaxConcurrentReconciles: ctx.MaxConcurrentReconciles}).
		Complete(reconciler)
}

type proxmoxDeploymentZoneReconciler struct {
	*context.ControllerContext
}

func (r proxmoxDeploymentZoneReconciler) Reconcile(ctx goctx.Context, request reconcile.Request) (_ reconcile.Result, reterr error) {
	logr := r.Logger.WithValues("proxmoxdeploymentzone", request.Name)
	// Fetch the ProxmoxDeploymentZone for this request.
	proxmoxDeploymentZone := &infrav1.ProxmoxDeploymentZone{}
	if err := r.Client.Get(ctx, request.NamespacedName, proxmoxDeploymentZone); err != nil {
		if apierrors.IsNotFound(err) {
			logr.V(4).Info("ProxmoxDeploymentZone not found, won't reconcile", "key", request.NamespacedName)
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	failureDomain := &infrav1.ProxmoxFailureDomain{}
	failureDomainKey := client.ObjectKey{Name: proxmoxDeploymentZone.Spec.FailureDomain}
	if err := r.Client.Get(ctx, failureDomainKey, failureDomain); err != nil {
		if apierrors.IsNotFound(err) {
			logr.V(4).Info("Failure Domain not found, won't reconcile", "key", failureDomainKey)
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	patchHelper, err := patch.NewHelper(proxmoxDeploymentZone, r.Client)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(
			err,
			"failed to init patch helper for %s %s/%s",
			proxmoxDeploymentZone.GroupVersionKind(),
			proxmoxDeploymentZone.Namespace,
			proxmoxDeploymentZone.Name)
	}

	proxmoxDeploymentZoneContext := &context.ProxmoxDeploymentZoneContext{
		ControllerContext:     r.ControllerContext,
		ProxmoxDeploymentZone: proxmoxDeploymentZone,
		ProxmoxFailureDomain:  failureDomain,
		Logger:                logr,
		PatchHelper:           patchHelper,
	}
	defer func() {
		if err := proxmoxDeploymentZoneContext.Patch(); err != nil {
			if reterr == nil {
				reterr = err
			}
			logr.Error(err, "patch failed", "proxmoxDeploymentZone", proxmoxDeploymentZoneContext.String())
		}
	}()

	if !proxmoxDeploymentZone.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(proxmoxDeploymentZoneContext)
	}

	return r.reconcileNormal(proxmoxDeploymentZoneContext)
}

func (r proxmoxDeploymentZoneReconciler) reconcileNormal(ctx *context.ProxmoxDeploymentZoneContext) (reconcile.Result, error) {
	ctrlutil.AddFinalizer(ctx.ProxmoxDeploymentZone, infrav1.DeploymentZoneFinalizer)

	authSession, err := r.getProxmoxSession(ctx)
	if err != nil {
		ctx.Logger.V(4).Error(err, "unable to create session")
		conditions.MarkFalse(ctx.ProxmoxDeploymentZone, infrav1.ProxmoxAvailableCondition, infrav1.ProxmoxUnreachableReason, clusterv1.ConditionSeverityError, err.Error())
		ctx.ProxmoxDeploymentZone.Status.Ready = pointer.Bool(false)
		return reconcile.Result{}, errors.Wrapf(err, "unable to create auth session")
	}
	ctx.AuthSession = authSession
	conditions.MarkTrue(ctx.ProxmoxDeploymentZone, infrav1.ProxmoxAvailableCondition)

	if err := r.reconcilePlacementConstraint(ctx); err != nil {
		ctx.ProxmoxDeploymentZone.Status.Ready = pointer.Bool(false)
		return reconcile.Result{}, errors.Wrap(err, "placement constraint is misconfigured")
	}
	conditions.MarkTrue(ctx.ProxmoxDeploymentZone, infrav1.PlacementConstraintMetCondition)

	// reconcile the failure domain
	if err := r.reconcileFailureDomain(ctx); err != nil {
		ctx.Logger.V(4).Error(err, "failed to reconcile failure domain", "failureDomain", ctx.ProxmoxDeploymentZone.Spec.FailureDomain)
		ctx.ProxmoxDeploymentZone.Status.Ready = pointer.Bool(false)
		return reconcile.Result{}, errors.Wrapf(err, "failed to reconcile failure domain")
	}
	conditions.MarkTrue(ctx.ProxmoxDeploymentZone, infrav1.ProxmoxFailureDomainValidatedCondition)

	// Ensure the ProxmoxDeploymentZone is marked as an owner of the ProxmoxFailureDomain.
	if !clusterutilv1.HasOwnerRef(ctx.ProxmoxFailureDomain.GetOwnerReferences(), metav1.OwnerReference{
		APIVersion: infrav1.GroupVersion.String(),
		Kind:       "ProxmoxDeploymentZone",
		Name:       ctx.ProxmoxDeploymentZone.Name,
	}) {
		if err := updateOwnerReferences(ctx, ctx.ProxmoxFailureDomain, r.Client, func() []metav1.OwnerReference {
			return append(ctx.ProxmoxFailureDomain.OwnerReferences, metav1.OwnerReference{
				APIVersion: infrav1.GroupVersion.String(),
				Kind:       ctx.ProxmoxDeploymentZone.Kind,
				Name:       ctx.ProxmoxDeploymentZone.Name,
				UID:        ctx.ProxmoxDeploymentZone.UID,
			})
		}); err != nil {
			return reconcile.Result{}, err
		}
	}

	ctx.ProxmoxDeploymentZone.Status.Ready = pointer.Bool(true)
	return reconcile.Result{}, nil
}

func (r proxmoxDeploymentZoneReconciler) reconcilePlacementConstraint(ctx *context.ProxmoxDeploymentZoneContext) error {
	placementConstraint := ctx.ProxmoxDeploymentZone.Spec.PlacementConstraint

	if resourcePool := placementConstraint.ResourcePool; resourcePool != "" {
		cluster := proxmox.Cluster{}
		if err := ctx.AuthSession.Get(placementConstraint.ResourcePool, cluster); err != nil {
			ctx.Logger.V(4).Error(err, "unable to find cluster resource pool", "name", resourcePool)
			conditions.MarkFalse(ctx.ProxmoxDeploymentZone, infrav1.PlacementConstraintMetCondition, infrav1.ResourcePoolNotFoundReason, clusterv1.ConditionSeverityError, "resource pool %s is misconfigured", resourcePool)
			return errors.Wrapf(err, "unable to find cluster resource pool %s", resourcePool)
		}
	}

	if folder := placementConstraint.Folder; folder != "" {
		storage := proxmox.Storage{}
		if err := ctx.AuthSession.Get(placementConstraint.Folder, storage); err != nil {
			ctx.Logger.V(4).Error(err, "unable to find storage folder", "name", folder)
			conditions.MarkFalse(ctx.ProxmoxDeploymentZone, infrav1.PlacementConstraintMetCondition, infrav1.FolderNotFoundReason, clusterv1.ConditionSeverityError, "datastore %s is misconfigured", folder)
			return errors.Wrapf(err, "unable to find storage folder %s", folder)
		}
	}
	return nil
}

func (r proxmoxDeploymentZoneReconciler) getProxmoxSession(ctx *context.ProxmoxDeploymentZoneContext) (*session.Session, error) {
	params := session.NewParams().
		WithServer(ctx.ProxmoxDeploymentZone.Spec.Server).
		WithDatacenter(ctx.ProxmoxFailureDomain.Spec.Topology.Datacenter).
		WithUserInfo(r.ControllerContext.Username, r.ControllerContext.Password).
		WithFeatures(session.Feature{
			EnableKeepAlive:   r.EnableKeepAlive,
			KeepAliveDuration: r.KeepAliveDuration,
		})

	clusterList := &infrav1.ProxmoxClusterList{}
	if err := r.Client.List(ctx, clusterList); err != nil {
		return nil, err
	}

	for _, proxmoxCluster := range clusterList.Items {
		if ctx.ProxmoxDeploymentZone.Spec.Server == proxmoxCluster.Spec.Server && proxmoxCluster.Spec.IdentityRef != nil {
			logger := ctx.Logger.WithValues("cluster", proxmoxCluster.Name)
			params = params.WithThumbprint(proxmoxCluster.Spec.Thumbprint)
			clust := proxmoxCluster
			creds, err := identity.GetCredentials(ctx, r.Client, &clust, r.Namespace)
			if err != nil {
				logger.Error(err, "error retrieving credentials from IdentityRef")
				continue
			}
			logger.Info("using server credentials to create the authenticated session")
			params = params.WithUserInfo(creds.Username, creds.Password)
			return session.GetOrCreate(r.Context,
				params)
		}
	}

	// Fallback to using credentials provided to the manager
	return session.GetOrCreate(r.Context,
		params)
}

func (r proxmoxDeploymentZoneReconciler) reconcileDelete(ctx *context.ProxmoxDeploymentZoneContext) (reconcile.Result, error) {
	r.Logger.Info("Deleting ProxmoxDeploymentZone")

	machines := &clusterv1.MachineList{}
	if err := r.Client.List(ctx, machines); err != nil {
		r.Logger.Error(err, "unable to list machines")
		return reconcile.Result{}, errors.Wrapf(err, "unable to list machines")
	}

	machinesUsingDeploymentZone := collections.FromMachineList(machines).Filter(collections.ActiveMachines, func(machine *clusterv1.Machine) bool {
		if machine.Spec.FailureDomain != nil {
			return *machine.Spec.FailureDomain == ctx.ProxmoxDeploymentZone.Name
		}
		return false
	})
	if len(machinesUsingDeploymentZone) > 0 {
		machineNamesStr := util.MachinesAsString(machinesUsingDeploymentZone.SortedByCreationTimestamp())
		err := errors.Errorf("%s is currently in use by machines: %s", ctx.ProxmoxDeploymentZone.Name, machineNamesStr)
		r.Logger.Error(err, "Error deleting ProxmoxDeploymentZone", "name", ctx.ProxmoxDeploymentZone.Name)
		return reconcile.Result{}, err
	}

	if err := updateOwnerReferences(ctx, ctx.ProxmoxFailureDomain, r.Client, func() []metav1.OwnerReference {
		return clusterutilv1.RemoveOwnerRef(ctx.ProxmoxFailureDomain.OwnerReferences, metav1.OwnerReference{
			APIVersion: infrav1.GroupVersion.String(),
			Kind:       ctx.ProxmoxDeploymentZone.Kind,
			Name:       ctx.ProxmoxDeploymentZone.Name,
		})
	}); err != nil {
		return reconcile.Result{}, err
	}

	if len(ctx.ProxmoxFailureDomain.OwnerReferences) == 0 {
		ctx.Logger.Info("deleting proxmoxFailureDomain", "name", ctx.ProxmoxFailureDomain.Name)
		if err := r.Client.Delete(ctx, ctx.ProxmoxFailureDomain); err != nil && !apierrors.IsNotFound(err) {
			ctx.Logger.Error(err, "failed to delete related %s %s", ctx.ProxmoxFailureDomain.GroupVersionKind(), ctx.ProxmoxFailureDomain.Name)
		}
	}

	ctrlutil.RemoveFinalizer(ctx.ProxmoxDeploymentZone, infrav1.DeploymentZoneFinalizer)

	return reconcile.Result{}, nil
}

// updateOwnerReferences uses the ownerRef function to calculate the owner references
// to be set on the object and patches the object.
func updateOwnerReferences(ctx goctx.Context, obj client.Object, client client.Client, ownerRefFunc func() []metav1.OwnerReference) error {
	patchHelper, err := patch.NewHelper(obj, client)
	if err != nil {
		return errors.Wrapf(err, "failed to init patch helper for %s %s",
			obj.GetObjectKind(),
			obj.GetName())
	}

	obj.SetOwnerReferences(ownerRefFunc())
	if err := patchHelper.Patch(ctx, obj); err != nil {
		return errors.Wrapf(err, "failed to patch object %s %s",
			obj.GetObjectKind(),
			obj.GetName())
	}
	return nil
}

func (r proxmoxDeploymentZoneReconciler) failureDomainsToDeploymentZones(a client.Object) []reconcile.Request {
	failureDomain, ok := a.(*infrav1.ProxmoxFailureDomain)
	if !ok {
		r.Logger.Error(nil, fmt.Sprintf("expected a ProxmoxFailureDomain but got a %T", a))
		return nil
	}

	var zones infrav1.ProxmoxDeploymentZoneList
	if err := r.Client.List(goctx.Background(), &zones); err != nil {
		return nil
	}

	var requests []reconcile.Request
	for _, zone := range zones.Items {
		if zone.Spec.FailureDomain == failureDomain.Name {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: zone.Name,
				},
			})
		}
	}
	return requests
}
