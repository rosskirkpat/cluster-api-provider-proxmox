package controllers

import (
	goctx "context"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	infrav1 "github.com/rosskirkpat/cluster-api-provider-proxmox/api/v1alpha1"
	"github.com/rosskirkpat/cluster-api-provider-proxmox/pkg/context"
	"github.com/rosskirkpat/cluster-api-provider-proxmox/pkg/identity"
	"github.com/rosskirkpat/cluster-api-provider-proxmox/pkg/session"
	infrautilv1 "github.com/rosskirkpat/cluster-api-provider-proxmox/pkg/util"
	apiv1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	clusterutilv1 "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type ProxmoxClusterReconciler struct {
	*context.ControllerContext
	client.Client
	Scheme *runtime.Scheme
}

// Reconcile ensures the back-end state reflects the Kubernetes resource state intent.
func (r *ProxmoxClusterReconciler) Reconcile(_ goctx.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	// Get the ProxmoxCluster resource for this request.
	proxmoxCluster := &infrav1.ProxmoxCluster{}
	if err := r.Client.Get(r, req.NamespacedName, proxmoxCluster); err != nil {
		if apierrors.IsNotFound(err) {
			r.Logger.V(4).Info("ProxmoxCluster not found, unable to reconcile", "key", req.NamespacedName)
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Fetch the CAPI Cluster.
	cluster, err := clusterutilv1.GetOwnerCluster(r, r.Client, proxmoxCluster.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, err
	}
	if cluster == nil {
		r.Logger.Info("Waiting for Cluster Controller to set OwnerRef on ProxmoxCluster")
		return reconcile.Result{}, nil
	}
	if annotations.IsPaused(cluster, proxmoxCluster) {
		r.Logger.V(4).Info("ProxmoxCluster %s/%s linked to a cluster that is paused",
			proxmoxCluster.Namespace, proxmoxCluster.Name)
		return reconcile.Result{}, nil
	}

	// Create the patch helper.
	patchHelper, err := patch.NewHelper(proxmoxCluster, r.Client)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(
			err,
			"failed to init patch helper for %s %s/%s",
			proxmoxCluster.GroupVersionKind(),
			proxmoxCluster.Namespace,
			proxmoxCluster.Name)
	}

	// Create the cluster context for this request.
	clusterContext := &context.ClusterContext{
		ControllerContext: r.ControllerContext,
		Cluster:           cluster,
		ProxmoxCluster:    proxmoxCluster,
		Logger:            r.Logger.WithName(req.Namespace).WithName(req.Name),
		PatchHelper:       patchHelper,
	}

	// Always issue a patch when exiting this function so changes to the
	// resource are patched back to the API server.
	defer func() {
		if err := clusterContext.Patch(); err != nil {
			if reterr == nil {
				reterr = err
			}
			clusterContext.Logger.Error(err, "patch failed", "cluster", clusterContext.String())
		}
	}()

	if err := setOwnerRefsOnProxmoxMachines(clusterContext); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to set owner refs on ProxmoxMachine objects")
	}

	// Handle deleted clusters
	if !proxmoxCluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(clusterContext)
	}

	// Handle non-deleted clusters
	return r.reconcileNormal(clusterContext)
}

func (r *ProxmoxClusterReconciler) reconcileDelete(ctx *context.ClusterContext) (reconcile.Result, error) {
	ctx.Logger.Info("Reconciling ProxmoxCluster delete")

	proxmoxMachines, err := infrautilv1.GetProxmoxMachinesInCluster(ctx, ctx.Client, ctx.Cluster.Namespace, ctx.Cluster.Name)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err,
			"unable to list ProxmoxMachines part of ProxmoxCluster %s/%s", ctx.ProxmoxCluster.Namespace, ctx.ProxmoxCluster.Name)
	}

	machineDeletionCount := 0
	var deletionErrors []error
	for _, proxmoxMachine := range proxmoxMachines {
		// If the ProxmoxMachine is not owned by the CAPI Machine object because the machine object was deleted
		// before setting the owner references, then proceed with the deletion of the ProxmoxMachine object.
		// This is required until CAPI has a solution for https://github.com/kubernetes-sigs/cluster-api/issues/5483
		// TODO may be fixed by https://github.com/kubernetes-sigs/cluster-api/pull/5865
		if clusterutilv1.IsOwnedByObject(proxmoxMachine, ctx.ProxmoxCluster) && len(proxmoxMachine.OwnerReferences) == 1 {
			machineDeletionCount++
			// Remove the finalizer since VM creation wouldn't proceed
			r.Logger.Info("Removing finalizer from ProxmoxMachine", "namespace", proxmoxMachine.Namespace, "name", proxmoxMachine.Name)
			ctrlutil.RemoveFinalizer(proxmoxMachine, infrav1.MachineFinalizer)
			if err := r.Client.Update(ctx, proxmoxMachine); err != nil {
				return reconcile.Result{}, err
			}
			if err := r.Client.Delete(ctx, proxmoxMachine); err != nil && !apierrors.IsNotFound(err) {
				ctx.Logger.Error(err, "Failed to delete for ProxmoxMachine", "namespace", proxmoxMachine.Namespace, "name", proxmoxMachine.Name)
				deletionErrors = append(deletionErrors, err)
			}
		}
	}
	if len(deletionErrors) > 0 {
		return reconcile.Result{}, kerrors.NewAggregate(deletionErrors)
	}

	if len(proxmoxMachines)-machineDeletionCount > 0 {
		ctx.Logger.Info("Waiting for ProxmoxMachines to be deleted", "count", len(proxmoxMachines)-machineDeletionCount)
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// Remove finalizer on Identity Secret
	if identity.IsSecretIdentity(ctx.ProxmoxCluster) {
		secret := &apiv1.Secret{}
		secretKey := client.ObjectKey{
			Namespace: ctx.ProxmoxCluster.Namespace,
			Name:      ctx.ProxmoxCluster.Spec.IdentityRef.Name,
		}
		err := ctx.Client.Get(ctx, secretKey, secret)
		if err != nil {
			if apierrors.IsNotFound(err) {
				ctrlutil.RemoveFinalizer(ctx.ProxmoxCluster, infrav1.ClusterFinalizer)
				return reconcile.Result{}, nil
			}
			return reconcile.Result{}, err
		}
		r.Logger.Info(fmt.Sprintf("Removing finalizer from Secret %s/%s having finalizers %v", secret.Namespace, secret.Name, secret.Finalizers))
		ctrlutil.RemoveFinalizer(secret, infrav1.SecretIdentitySetFinalizer)

		if err := ctx.Client.Update(ctx, secret); err != nil {
			return reconcile.Result{}, err
		}
		if err := ctx.Client.Delete(ctx, secret); err != nil {
			return reconcile.Result{}, err
		}
	}

	// Cluster is deleted so remove the finalizer.
	ctrlutil.RemoveFinalizer(ctx.ProxmoxCluster, infrav1.ClusterFinalizer)

	return reconcile.Result{}, nil
}

func (r *ProxmoxClusterReconciler) reconcileNormal(ctx *context.ClusterContext) (reconcile.Result, error) {
	ctx.Logger.Info("Reconciling ProxmoxCluster")

	// If the ProxmoxCluster doesn't have our finalizer, add it.
	ctrlutil.AddFinalizer(ctx.ProxmoxCluster, infrav1.ClusterFinalizer)

	ok, err := r.reconcileDeploymentZones(ctx)
	if err != nil {
		return reconcile.Result{}, err
	}
	if !ok {
		ctx.Logger.Info("waiting for failure domains to be reconciled")
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

	if err := r.reconcileIdentitySecret(ctx); err != nil {
		conditions.MarkFalse(ctx.ProxmoxCluster, infrav1.ProxmoxAvailableCondition, infrav1.ProxmoxUnreachableReason, clusterv1.ConditionSeverityError, err.Error())
		return reconcile.Result{}, err
	}

	proxmoxSession, err := r.reconcileProxmoxConnectivity(ctx)
	if err != nil {
		conditions.MarkFalse(ctx.ProxmoxCluster, infrav1.ProxmoxAvailableCondition, infrav1.ProxmoxUnreachableReason, clusterv1.ConditionSeverityError, err.Error())
		return reconcile.Result{}, errors.Wrapf(err,
			"unexpected error while probing proxmox for %s", ctx)
	}
	conditions.MarkTrue(ctx.ProxmoxCluster, infrav1.ProxmoxAvailableCondition)

	err = r.reconcileProxmoxVersion(ctx, proxmoxSession)
	if err != nil || ctx.ProxmoxCluster.Status.ProxmoxVersion == "" {
		conditions.MarkFalse(ctx.ProxmoxCluster, infrav1.ClusterFeaturesAvailableCondition, infrav1.MissingProxmoxVersionReason, clusterv1.ConditionSeverityWarning, "Proxmox API version not set")
		ctx.Logger.Error(err, "could not reconcile Proxmox version")
	}

	ctx.ProxmoxCluster.Status.Ready = true

	// Ensure the ProxmoxCluster is reconciled when the API server first comes online.
	// A reconcile event will only be triggered if the Cluster is not marked as
	// ControlPlaneInitialized.
	r.reconcileProxmoxClusterWhenAPIServerIsOnline(ctx)
	if ctx.ProxmoxCluster.Spec.ControlPlaneEndpoint.IsZero() {
		ctx.Logger.Info("control plane endpoint is not reconciled")
		return reconcile.Result{}, nil
	}

	// If the cluster is deleted, that's mean that the workload cluster is being deleted and so the CCM/CSI instances
	if !ctx.Cluster.DeletionTimestamp.IsZero() {
		return reconcile.Result{}, nil
	}

	// Wait until the API server is online and accessible.
	if !r.isAPIServerOnline(ctx) {
		return reconcile.Result{}, nil
	}

	return reconcile.Result{}, nil
}

func (r *ProxmoxClusterReconciler) reconcileIdentitySecret(ctx *context.ClusterContext) error {
	proxmoxCluster := ctx.ProxmoxCluster
	if identity.IsSecretIdentity(proxmoxCluster) {
		secret := &apiv1.Secret{}
		secretKey := client.ObjectKey{
			Namespace: proxmoxCluster.Namespace,
			Name:      proxmoxCluster.Spec.IdentityRef.Name,
		}
		err := ctx.Client.Get(ctx, secretKey, secret)
		if err != nil {
			return err
		}

		// check if cluster is already an owner
		if !clusterutilv1.IsOwnedByObject(secret, proxmoxCluster) {
			ownerReferences := secret.GetOwnerReferences()
			if identity.IsOwnedByIdentityOrCluster(ownerReferences) {
				return fmt.Errorf("another cluster has set the OwnerRef for secret: %s/%s", secret.Namespace, secret.Name)
			}
			ownerReferences = append(ownerReferences, metav1.OwnerReference{
				APIVersion: infrav1.GroupVersion.String(),
				Kind:       proxmoxCluster.Kind,
				Name:       proxmoxCluster.Name,
				UID:        proxmoxCluster.UID,
			})
			secret.SetOwnerReferences(ownerReferences)
		}
		if !ctrlutil.ContainsFinalizer(secret, infrav1.SecretIdentitySetFinalizer) {
			ctrlutil.AddFinalizer(secret, infrav1.SecretIdentitySetFinalizer)
		}
		err = r.Client.Update(ctx, secret)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *ProxmoxClusterReconciler) reconcileProxmoxConnectivity(ctx *context.ClusterContext) (*session.Session, error) {
	params := session.NewParams().
		WithServer(ctx.ProxmoxCluster.Spec.Server).
		WithThumbprint(ctx.ProxmoxCluster.Spec.Thumbprint).
		WithFeatures(session.Feature{
			EnableKeepAlive:   r.EnableKeepAlive,
			KeepAliveDuration: r.KeepAliveDuration,
		})

	if ctx.ProxmoxCluster.Spec.IdentityRef != nil {
		creds, err := identity.GetCredentials(ctx, r.Client, ctx.ProxmoxCluster, r.Namespace)
		if err != nil {
			return nil, err
		}

		params = params.WithUserInfo(creds.Username, creds.Password)
		return session.GetOrCreate(ctx, params)
	}

	params = params.WithUserInfo(ctx.Username, ctx.Password)
	return session.GetOrCreate(ctx, params)
}

func (r *ProxmoxClusterReconciler) reconcileProxmoxVersion(ctx *context.ClusterContext, s *session.Session) error {
	version, err := s.GetVersion()
	if err != nil {
		return err
	}
	ctx.ProxmoxCluster.Status.ProxmoxVersion = version
	return nil
}

func (r *ProxmoxClusterReconciler) reconcileDeploymentZones(ctx *context.ClusterContext) (bool, error) {
	var deploymentZoneList infrav1.ProxmoxDeploymentZoneList
	err := r.Client.List(ctx, &deploymentZoneList)
	if err != nil {
		return false, errors.Wrap(err, "unable to list deployment zones")
	}

	readyNotReported, notReady := 0, 0
	failureDomains := clusterv1.FailureDomains{}
	for _, zone := range deploymentZoneList.Items {
		if zone.Spec.Server == ctx.ProxmoxCluster.Spec.Server {
			// This should never fail, validating webhook should catch this first
			if ctx.ProxmoxCluster.Spec.FailureDomainSelector != nil {
				selector, err := metav1.LabelSelectorAsSelector(ctx.ProxmoxCluster.Spec.FailureDomainSelector)
				if err != nil {
					return false, errors.Wrapf(err, "zone label selector is misconfigured")
				}

				// An empty selector allows the zone to be selected
				if !selector.Empty() && !selector.Matches(labels.Set(zone.GetLabels())) {
					r.Logger.V(5).Info("skipping the deployment zone due to label mismatch", "name", zone.Name)
					continue
				}
			}

			if zone.Status.Ready == nil {
				readyNotReported++
				failureDomains[zone.Name] = clusterv1.FailureDomainSpec{
					ControlPlane: pointer.BoolDeref(zone.Spec.ControlPlane, true),
				}
			} else {
				if *zone.Status.Ready {
					failureDomains[zone.Name] = clusterv1.FailureDomainSpec{
						ControlPlane: pointer.BoolDeref(zone.Spec.ControlPlane, true),
					}
				} else {
					notReady++
				}
			}
		}
	}

	ctx.ProxmoxCluster.Status.FailureDomains = failureDomains
	if readyNotReported > 0 {
		conditions.MarkFalse(ctx.ProxmoxCluster, infrav1.FailureDomainsAvailableCondition, infrav1.WaitingForFailureDomainStatusReason, clusterv1.ConditionSeverityInfo, "waiting for failure domains to report ready status")
		return false, nil
	}

	if len(failureDomains) > 0 {
		if notReady > 0 {
			conditions.MarkFalse(ctx.ProxmoxCluster, infrav1.FailureDomainsAvailableCondition, infrav1.FailureDomainsSkippedReason, clusterv1.ConditionSeverityInfo, "one or more failure domains are not ready")
		} else {
			conditions.MarkTrue(ctx.ProxmoxCluster, infrav1.FailureDomainsAvailableCondition)
		}
	} else {
		// Remove the condition if failure domains do not exist
		conditions.Delete(ctx.ProxmoxCluster, infrav1.FailureDomainsAvailableCondition)
	}
	return true, nil
}

var (
	// apiServerTriggers is used to prevent multiple goroutines for a single
	// Cluster that poll to see if the target API server is online.
	apiServerTriggers   = map[types.UID]struct{}{}
	apiServerTriggersMu sync.Mutex
)

func (r *ProxmoxClusterReconciler) reconcileProxmoxClusterWhenAPIServerIsOnline(ctx *context.ClusterContext) {
	if conditions.IsTrue(ctx.Cluster, clusterv1.ControlPlaneInitializedCondition) {
		ctx.Logger.Info("skipping reconcile when API server is online",
			"reason", "controlPlaneInitialized")
		return
	}
	apiServerTriggersMu.Lock()
	defer apiServerTriggersMu.Unlock()
	if _, ok := apiServerTriggers[ctx.Cluster.UID]; ok {
		ctx.Logger.Info("skipping reconcile when API server is online",
			"reason", "alreadyPolling")
		return
	}
	apiServerTriggers[ctx.Cluster.UID] = struct{}{}
	go func() {
		// Block until the target API server is online.
		ctx.Logger.Info("start polling API server for online check")
		wait.PollImmediateInfinite(time.Second*1, func() (bool, error) { return r.isAPIServerOnline(ctx), nil }) //nolint:errcheck
		ctx.Logger.Info("stop polling API server for online check")
		ctx.Logger.Info("triggering GenericEvent", "reason", "api-server-online")
		eventChannel := ctx.GetGenericEventChannelFor(ctx.ProxmoxCluster.GetObjectKind().GroupVersionKind())
		eventChannel <- event.GenericEvent{
			Object: ctx.ProxmoxCluster,
		}

		// Once the control plane has been marked as initialized it is safe to
		// remove the key from the map that prevents multiple goroutines from
		// polling the API server to see if it is online.
		ctx.Logger.Info("start polling for control plane initialized")
		wait.PollImmediateInfinite(time.Second*1, func() (bool, error) { return r.isControlPlaneInitialized(ctx), nil }) //nolint:errcheck
		ctx.Logger.Info("stop polling for control plane initialized")
		apiServerTriggersMu.Lock()
		delete(apiServerTriggers, ctx.Cluster.UID)
		apiServerTriggersMu.Unlock()
	}()
}

func (r *ProxmoxClusterReconciler) isAPIServerOnline(ctx *context.ClusterContext) bool {
	if kubeClient, err := infrautilv1.NewKubeClient(ctx, ctx.Client, ctx.Cluster); err == nil {
		if _, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{}); err == nil {
			// The target cluster is online. To make sure the correct control
			// plane endpoint information is logged, it is necessary to fetch
			// an up-to-date Cluster resource. If this fails, then set the
			// control plane endpoint information to the values from the
			// ProxmoxCluster resource, as it must have the correct information
			// if the API server is online.
			cluster := &clusterv1.Cluster{}
			clusterKey := client.ObjectKey{Namespace: ctx.Cluster.Namespace, Name: ctx.Cluster.Name}
			if err := ctx.Client.Get(ctx, clusterKey, cluster); err != nil {
				cluster = ctx.Cluster.DeepCopy()
				cluster.Spec.ControlPlaneEndpoint.Host = ctx.ProxmoxCluster.Spec.ControlPlaneEndpoint.Host
				cluster.Spec.ControlPlaneEndpoint.Port = ctx.ProxmoxCluster.Spec.ControlPlaneEndpoint.Port
				ctx.Logger.Error(err, "failed to get updated cluster object while checking if API server is online")
			}
			ctx.Logger.Info(
				"API server is online",
				"controlPlaneEndpoint", cluster.Spec.ControlPlaneEndpoint.String())
			return true
		}
	}
	return false
}

func (r *ProxmoxClusterReconciler) isControlPlaneInitialized(ctx *context.ClusterContext) bool {
	cluster := &clusterv1.Cluster{}
	clusterKey := client.ObjectKey{Namespace: ctx.Cluster.Namespace, Name: ctx.Cluster.Name}
	if err := ctx.Client.Get(ctx, clusterKey, cluster); err != nil {
		if !apierrors.IsNotFound(err) {
			ctx.Logger.Error(err, "failed to get updated cluster object while checking if control plane is initialized")
			return false
		}
		ctx.Logger.Info("exiting early because cluster no longer exists")
		return true
	}
	return conditions.IsTrue(ctx.Cluster, clusterv1.ControlPlaneInitializedCondition)
}

func setOwnerRefsOnProxmoxMachines(ctx *context.ClusterContext) error {
	proxmoxMachines, err := infrautilv1.GetProxmoxMachinesInCluster(ctx, ctx.Client, ctx.Cluster.Namespace, ctx.Cluster.Name)
	if err != nil {
		return errors.Wrapf(err,
			"unable to list ProxmoxMachines part of ProxmoxCluster %s/%s", ctx.ProxmoxCluster.Namespace, ctx.ProxmoxCluster.Name)
	}

	var patchErrors []error
	for _, proxmoxMachine := range proxmoxMachines {
		patchHelper, err := patch.NewHelper(proxmoxMachine, ctx.Client)
		if err != nil {
			patchErrors = append(patchErrors, err)
			continue
		}

		proxmoxMachine.SetOwnerReferences(clusterutilv1.EnsureOwnerRef(
			proxmoxMachine.OwnerReferences,
			metav1.OwnerReference{
				APIVersion: ctx.ProxmoxCluster.APIVersion,
				Kind:       ctx.ProxmoxCluster.Kind,
				Name:       ctx.ProxmoxCluster.Name,
				UID:        ctx.ProxmoxCluster.UID,
			}))

		if err := patchHelper.Patch(ctx, proxmoxMachine); err != nil {
			patchErrors = append(patchErrors, err)
		}
	}
	return kerrors.NewAggregate(patchErrors)
}

// controlPlaneMachineToCluster is a handler.ToRequestsFunc to be used
// to enqueue requests for reconciliation for ProxmoxCluster to update
// its status.apiEndpoints field.
func (r *ProxmoxClusterReconciler) controlPlaneMachineToCluster(o client.Object) []ctrl.Request {
	proxmoxMachine, ok := o.(*infrav1.ProxmoxMachine)
	if !ok {
		r.Logger.Error(nil, fmt.Sprintf("expected a ProxmoxMachine but got a %T", o))
		return nil
	}
	if !infrautilv1.IsControlPlaneMachine(proxmoxMachine) {
		return nil
	}
	if len(proxmoxMachine.Status.Addresses) == 0 {
		return nil
	}
	// Get the ProxmoxMachine's preferred IP address.
	if _, err := infrautilv1.GetMachinePreferredIPAddress(proxmoxMachine); err != nil {
		if err == infrautilv1.ErrNoMachineIPAddr {
			return nil
		}
		r.Logger.Error(err, "failed to get preferred IP address for ProxmoxMachine",
			"namespace", proxmoxMachine.Namespace, "name", proxmoxMachine.Name)
		return nil
	}

	// Fetch the CAPI Cluster.
	cluster, err := clusterutilv1.GetClusterFromMetadata(r, r.Client, proxmoxMachine.ObjectMeta)
	if err != nil {
		r.Logger.Error(err, "ProxmoxMachine is missing cluster label or cluster does not exist",
			"namespace", proxmoxMachine.Namespace, "name", proxmoxMachine.Name)
		return nil
	}

	if conditions.IsTrue(cluster, clusterv1.ControlPlaneInitializedCondition) {
		return nil
	}

	if !cluster.Spec.ControlPlaneEndpoint.IsZero() {
		return nil
	}

	// Fetch the ProxmoxCluster
	proxmoxCluster := &infrav1.ProxmoxCluster{}
	proxmoxClusterKey := client.ObjectKey{
		Namespace: proxmoxMachine.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	if err := r.Client.Get(r, proxmoxClusterKey, proxmoxCluster); err != nil {
		r.Logger.Error(err, "failed to get ProxmoxCluster",
			"namespace", proxmoxClusterKey.Namespace, "name", proxmoxClusterKey.Name)
		return nil
	}

	if !proxmoxCluster.Spec.ControlPlaneEndpoint.IsZero() {
		return nil
	}

	return []ctrl.Request{{
		NamespacedName: types.NamespacedName{
			Namespace: proxmoxClusterKey.Namespace,
			Name:      proxmoxClusterKey.Name,
		},
	}}
}

func (r *ProxmoxClusterReconciler) deploymentZoneToCluster(o client.Object) []ctrl.Request {
	var requests []ctrl.Request
	obj, ok := o.(*infrav1.ProxmoxDeploymentZone)
	if !ok {
		r.Logger.Error(nil, fmt.Sprintf("expected an infrav1.ProxmoxDeploymentZone but got a %T", o))
		return nil
	}

	var clusterList infrav1.ProxmoxClusterList
	err := r.Client.List(r.Context, &clusterList)
	if err != nil {
		r.Logger.Error(err, "unable to list clusters")
		return requests
	}

	for _, cluster := range clusterList.Items {
		if obj.Spec.Server == cluster.Spec.Server {
			r := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				},
			}
			requests = append(requests, r)
		}
	}
	return requests
}

// SetupWithManager sets up the controller with the Manager.
func (r *ProxmoxClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.ProxmoxCluster{}).
		Complete(r)
}
