package controllers

import (
	goctx "context"
	"fmt"
	"github.com/rosskirkpat/cluster-api-provider-proxmox/pkg/identity"
	"github.com/rosskirkpat/cluster-api-provider-proxmox/pkg/session"
	"k8s.io/apimachinery/pkg/runtime"
	"reflect"
	"strings"
	"time"

	"github.com/pkg/errors"
	infrav1 "github.com/rosskirkpat/cluster-api-provider-proxmox/api/v1alpha1"
	"github.com/rosskirkpat/cluster-api-provider-proxmox/pkg/context"
	"github.com/rosskirkpat/cluster-api-provider-proxmox/pkg/record"
	"github.com/rosskirkpat/cluster-api-provider-proxmox/pkg/services"
	"github.com/rosskirkpat/cluster-api-provider-proxmox/pkg/util"
	apiv1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/remote"
	ipamv1 "sigs.k8s.io/cluster-api/exp/ipam/api/v1alpha1"
	clusterutilv1 "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=proxmoxvms,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=proxmoxvms/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machinedeployments;machinesets,verbs=get;list;watch
// +kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=kubeadmcontrolplanes,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;delete

// AddVMControllerToManager adds the VM controller to the provided manager.
//
//nolint:forcetypeassert
func AddVMControllerToManager(ctx *context.ControllerContext, mgr manager.Manager) error {
	var (
		controlledType     = &infrav1.ProxmoxVM{}
		controlledTypeName = reflect.TypeOf(controlledType).Elem().Name()
		controlledTypeGVK  = infrav1.GroupVersion.WithKind(controlledTypeName)

		controllerNameShort = fmt.Sprintf("%s-controller", strings.ToLower(controlledTypeName))
		controllerNameLong  = fmt.Sprintf("%s/%s/%s", ctx.Namespace, ctx.Name, controllerNameShort)
	)

	// Build the controller context.
	controllerContext := &context.ControllerContext{
		Name:     controllerNameShort,
		Recorder: record.New(mgr.GetEventRecorderFor(controllerNameLong)),
		Logger:   ctx.Logger.WithName(controllerNameShort),
	}
	r := ProxmoxVMReconciler{
		ControllerContext: controllerContext,
		//VMService:         &services.VirtualMachineService{},
	}

	controller, err := ctrl.NewControllerManagedBy(mgr).
		// Watch the controlled, infrastructure resource.
		For(controlledType).
		// Watch a GenericEvent channel for the controlled resource.
		//
		// This is useful when there are events outside of Kubernetes that
		// should cause a resource to be synchronized, such as a goroutine
		// waiting on some asynchronous, external task to complete.
		Watches(
			&source.Channel{Source: ctx.GetGenericEventChannelFor(controlledTypeGVK)},
			&handler.EnqueueRequestForObject{},
		).
		WithOptions(controller.Options{MaxConcurrentReconciles: ctx.MaxConcurrentReconciles}).
		Build(&r)
	if err != nil {
		return err
	}

	err = controller.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		handler.EnqueueRequestsFromMapFunc(r.clusterToProxmoxVMs),
		predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldCluster := e.ObjectOld.(*clusterv1.Cluster)
				newCluster := e.ObjectNew.(*clusterv1.Cluster)
				return oldCluster.Spec.Paused && !newCluster.Spec.Paused
			},
			CreateFunc: func(e event.CreateEvent) bool {
				if _, ok := e.Object.GetAnnotations()[clusterv1.PausedAnnotation]; !ok {
					return false
				}
				return true
			},
		})
	if err != nil {
		return err
	}

	err = controller.Watch(
		&source.Kind{Type: &infrav1.ProxmoxCluster{}},
		handler.EnqueueRequestsFromMapFunc(r.proxmoxClusterToProxmoxVMs),
		predicate.Funcs{
			UpdateFunc:  func(e event.UpdateEvent) bool { return false },
			CreateFunc:  func(e event.CreateEvent) bool { return false },
			DeleteFunc:  func(e event.DeleteEvent) bool { return false },
			GenericFunc: func(e event.GenericEvent) bool { return false },
		})
	if err != nil {
		return err
	}

	err = controller.Watch(
		&source.Kind{Type: &ipamv1.IPAddressClaim{}},
		handler.EnqueueRequestsFromMapFunc(r.ipAddressClaimToProxmoxVM))
	if err != nil {
		return err
	}
	return nil
}

type ProxmoxVMReconciler struct {
	*context.ControllerContext
	ctrlclient.Client
	Scheme    *runtime.Scheme
	VMService services.VirtualMachineService
}

// Reconcile ensures the back-end state reflects the Kubernetes resource state intent.
func (r *ProxmoxVMReconciler) Reconcile(ctx goctx.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	// Get the ProxmoxVM resource for this request.
	proxmoxVM := &infrav1.ProxmoxVM{}
	if err := r.Client.Get(r, req.NamespacedName, proxmoxVM); err != nil {
		if apierrors.IsNotFound(err) {
			r.Logger.Info("ProxmoxVM not found, won't reconcile", "key", req.NamespacedName)
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Create the patch helper.
	patchHelper, err := patch.NewHelper(proxmoxVM, r.Client)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(
			err,
			"failed to init patch helper for %s %s/%s",
			proxmoxVM.GroupVersionKind(),
			proxmoxVM.Namespace,
			proxmoxVM.Name)
	}

	authSession, err := r.retrieveProxmoxSession(ctx, proxmoxVM)
	if err != nil {
		conditions.MarkFalse(proxmoxVM, infrav1.ProxmoxAvailableCondition, infrav1.ProxmoxUnreachableReason, clusterv1.ConditionSeverityError, err.Error())
		return reconcile.Result{}, err
	}
	conditions.MarkTrue(proxmoxVM, infrav1.ProxmoxAvailableCondition)

	// Fetch the owner ProxmoxMachine.
	proxmoxMachine, err := util.GetOwnerProxmoxMachine(r, r.Client, proxmoxVM.ObjectMeta)
	// proxmoxMachine can be nil in cases where custom mover other than clusterctl
	// moves the resources without ownerreferences set
	// in that case nil proxmoxMachine can cause panic and CrashLoopBackOff the pod
	// preventing proxmoxmachine_controller from setting the ownerref
	if err != nil || proxmoxMachine == nil {
		r.Logger.Info("Owner ProxmoxMachine not found, won't reconcile", "key", req.NamespacedName)
		return reconcile.Result{}, nil
	}

	proxmoxCluster, err := util.GetProxmoxClusterFromProxmoxMachine(r, r.Client, proxmoxMachine)
	if err != nil || proxmoxCluster == nil {
		r.Logger.Info("proxmoxCluster not found, won't reconcile", "key", ctrlclient.ObjectKeyFromObject(proxmoxMachine))
		return reconcile.Result{}, nil
	}

	// Fetch the CAPI Machine.
	machine, err := clusterutilv1.GetOwnerMachine(r, r.Client, proxmoxMachine.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, err
	}
	if machine == nil {
		r.Logger.Info("Waiting for OwnerRef to be set on ProxmoxMachine", "key", proxmoxMachine.Name)
		return reconcile.Result{}, nil
	}

	var proxmoxFailureDomain *infrav1.ProxmoxFailureDomain
	if failureDomain := machine.Spec.FailureDomain; failureDomain != nil {
		proxmoxDeploymentZone := &infrav1.ProxmoxDeploymentZone{}
		if err := r.Client.Get(r, apitypes.NamespacedName{Name: *failureDomain}, proxmoxDeploymentZone); err != nil {
			return reconcile.Result{}, errors.Wrapf(err, "failed to find proxmox deployment zone %s", *failureDomain)
		}

		proxmoxFailureDomain = &infrav1.ProxmoxFailureDomain{}
		if err := r.Client.Get(r, apitypes.NamespacedName{Name: proxmoxDeploymentZone.Spec.FailureDomain}, proxmoxFailureDomain); err != nil {
			return reconcile.Result{}, errors.Wrapf(err, "failed to find proxmox failure domain %s", proxmoxDeploymentZone.Spec.FailureDomain)
		}
	}

	// Create the VM context for this request.
	vmContext := &context.VMContext{
		ControllerContext:    r.ControllerContext,
		ProxmoxVM:            proxmoxVM,
		ProxmoxFailureDomain: proxmoxFailureDomain,
		Session:              authSession,
		Logger:               r.Logger.WithName(req.Namespace).WithName(req.Name),
		PatchHelper:          patchHelper,
	}

	// Print the task-ref upon entry and upon exit.
	vmContext.Logger.V(4).Info(
		"ProxmoxVM.Status.TaskRef OnEntry",
		"task-ref", vmContext.ProxmoxVM.Status.TaskRef)
	defer func() {
		vmContext.Logger.V(4).Info(
			"ProxmoxVM.Status.TaskRef OnExit",
			"task-ref", vmContext.ProxmoxVM.Status.TaskRef)
	}()

	// Always issue a patch when exiting this function so changes to the
	// resource are patched back to the API server.
	defer func() {
		// always update the readyCondition.
		conditions.SetSummary(vmContext.ProxmoxVM,
			conditions.WithConditions(
				infrav1.ProxmoxAvailableCondition,
				infrav1.IPAddressClaimedCondition,
				infrav1.VMProvisionedCondition,
			),
		)

		// Patch the ProxmoxVM resource.
		if err := vmContext.Patch(); err != nil {
			if reterr == nil {
				reterr = err
			}
			vmContext.Logger.Error(err, "patch failed", "vm", vmContext.String())
		}
	}()

	cluster, err := clusterutilv1.GetClusterFromMetadata(r.ControllerContext, r.Client, proxmoxVM.ObjectMeta)
	if err == nil {
		if annotations.IsPaused(cluster, proxmoxVM) {
			r.Logger.V(4).Info("ProxmoxVM %s/%s linked to a cluster that is paused",
				proxmoxVM.Namespace, proxmoxVM.Name)
			return reconcile.Result{}, nil
		}
	}

	return r.reconcile(vmContext)
}

// reconcile encases the behavior of the controller around cluster module information
// retrieval depending upon inputs passed.
//
// This logic was moved to a smaller function outside the main Reconcile() loop
// for the ease of testing.
func (r *ProxmoxVMReconciler) reconcile(ctx *context.VMContext) (reconcile.Result, error) {
	// Handle deleted machines
	if !ctx.ProxmoxVM.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx)
	}

	// Handle non-deleted machines
	return r.reconcileNormal(ctx)
}

func (r *ProxmoxVMReconciler) reconcileDelete(ctx *context.VMContext) (reconcile.Result, error) {
	ctx.Logger.Info("Handling deleted ProxmoxVM")

	conditions.MarkFalse(ctx.ProxmoxVM, infrav1.VMProvisionedCondition, clusterv1.DeletingReason, clusterv1.ConditionSeverityInfo, "")
	vm, err := r.VMService.DestroyVM(ctx)
	if err != nil {
		conditions.MarkFalse(ctx.ProxmoxVM, infrav1.VMProvisionedCondition, "DeletionFailed", clusterv1.ConditionSeverityWarning, err.Error())
		return reconcile.Result{}, errors.Wrapf(err, "failed to destroy VM")
	}

	// Requeue the operation until the VM is "notfound".
	if vm.State != infrav1.VirtualMachineStateNotFound {
		ctx.Logger.Info("vm state is not reconciled", "expected-vm-state", infrav1.VirtualMachineStateNotFound, "actual-vm-state", vm.State)
		return reconcile.Result{}, nil
	}

	// Attempt to delete the node corresponding to the proxmox VM
	if err := r.deleteNode(ctx, vm.Name); err != nil {
		r.Logger.V(6).Info("unable to delete node", "err", err)
	}

	if err := r.deleteIPAddressClaims(ctx); err != nil {
		return reconcile.Result{}, err
	}

	// The VM is deleted so remove the finalizer.
	ctrlutil.RemoveFinalizer(ctx.ProxmoxVM, infrav1.VMFinalizer)

	return reconcile.Result{}, nil
}

// deleteNode attempts to find and best effort delete the node corresponding to the VM
// This is necessary since CAPI does not the nodeRef field on the owner Machine object
// until the node moves to Ready state. Hence, on Machine deletion it is unable to delete
// the kubernetes node corresponding to the VM.
func (r *ProxmoxVMReconciler) deleteNode(ctx *context.VMContext, name string) error {
	// Fetching the cluster object from the ProxmoxVM object to create a remote client to the cluster
	cluster, err := clusterutilv1.GetClusterFromMetadata(r.ControllerContext, r.Client, ctx.ProxmoxVM.ObjectMeta)
	if err != nil {
		return err
	}
	clusterClient, err := remote.NewClusterClient(ctx, r.ControllerContext.Name, r.Client, ctrlclient.ObjectKeyFromObject(cluster))
	if err != nil {
		return err
	}

	// Attempt to delete the corresponding node
	node := &apiv1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	return clusterClient.Delete(ctx, node)
}

func (r *ProxmoxVMReconciler) reconcileNormal(ctx *context.VMContext) (reconcile.Result, error) {
	if ctx.ProxmoxVM.Status.FailureReason != nil || ctx.ProxmoxVM.Status.FailureMessage != nil {
		r.Logger.Info("VM is failed, won't reconcile", "namespace", ctx.ProxmoxVM.Namespace, "name", ctx.ProxmoxVM.Name)
		return reconcile.Result{}, nil
	}
	// If the ProxmoxVM doesn't have our finalizer, add it.
	ctrlutil.AddFinalizer(ctx.ProxmoxVM, infrav1.VMFinalizer)

	if r.isWaitingForStaticIPAllocation(ctx) {
		conditions.MarkFalse(ctx.ProxmoxVM, infrav1.VMProvisionedCondition, infrav1.WaitingForStaticIPAllocationReason, clusterv1.ConditionSeverityInfo, "")
		ctx.Logger.Info("vm is waiting for static ip to be available")
		return reconcile.Result{}, nil
	}

	if err := r.reconcileIPAddressClaims(ctx); err != nil {
		return reconcile.Result{}, err
	}

	// Get or create the VM.
	vm, err := r.VMService.ReconcileVM(ctx)
	if err != nil {
		ctx.Logger.Error(err, "error reconciling VM")
		return reconcile.Result{}, errors.Wrapf(err, "failed to reconcile VM")
	}

	// Do not proceed until the backend VM is marked ready.
	if vm.State != infrav1.VirtualMachineStateReady {
		ctx.Logger.Info(
			"VM state is not reconciled",
			"expected-vm-state", infrav1.VirtualMachineStateReady,
			"actual-vm-state", vm.State)
		return reconcile.Result{}, nil
	}

	// Update the ProxmoxVM's BIOS UUID.
	ctx.Logger.Info("vm bios-uuid", "biosuuid", vm.BiosUUID)

	// defensive check to ensure we are not removing the biosUUID
	if vm.BiosUUID != "" {
		ctx.ProxmoxVM.Spec.BiosUUID = vm.BiosUUID
	} else {
		return reconcile.Result{}, errors.Errorf("bios uuid is empty while VM is ready")
	}

	// Update the ProxmoxVM's network status.
	r.reconcileNetwork(ctx, vm)

	// we didn't get any addresses, requeue
	if len(ctx.ProxmoxVM.Status.Addresses) == 0 {
		conditions.MarkFalse(ctx.ProxmoxVM, infrav1.VMProvisionedCondition, infrav1.WaitingForIPAllocationReason, clusterv1.ConditionSeverityInfo, "")
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// Once the network is online the VM is considered ready.
	ctx.ProxmoxVM.Status.Ready = true
	conditions.MarkTrue(ctx.ProxmoxVM, infrav1.VMProvisionedCondition)
	ctx.Logger.Info("ProxmoxVM is ready")
	return reconcile.Result{}, nil
}

// isWaitingForStaticIPAllocation checks whether the VM should wait for a static IP
// to be allocated.
// It checks the state of both DHCP4 and DHCP6 for all the network devices and if
// any static IP addresses or IPAM Pools are specified.
func (r *ProxmoxVMReconciler) isWaitingForStaticIPAllocation(ctx *context.VMContext) bool {
	devices := ctx.ProxmoxVM.Spec.Network.Devices
	for _, dev := range devices {
		if !dev.DHCP4 && !dev.DHCP6 && len(dev.IPAddrs) == 0 && len(dev.AddressesFromPools) == 0 {
			// Static IP is not available yet
			return true
		}
	}

	return false
}

func (r *ProxmoxVMReconciler) reconcileNetwork(ctx *context.VMContext, vm infrav1.VirtualMachine) {
	ctx.ProxmoxVM.Status.Network = vm.Network
	ipAddrs := make([]string, 0, len(vm.Network))
	for _, netStatus := range ctx.ProxmoxVM.Status.Network {
		ipAddrs = append(ipAddrs, netStatus.IPAddrs...)
	}
	ctx.ProxmoxVM.Status.Addresses = ipAddrs
}

func (r *ProxmoxVMReconciler) clusterToProxmoxVMs(a ctrlclient.Object) []reconcile.Request {
	requests := []reconcile.Request{}
	vms := &infrav1.ProxmoxVMList{}
	err := r.Client.List(goctx.Background(), vms, ctrlclient.MatchingLabels(
		map[string]string{
			clusterv1.ClusterNameLabel: a.GetName(),
		},
	))
	if err != nil {
		return requests
	}
	for _, vm := range vms.Items {
		r := reconcile.Request{
			NamespacedName: apitypes.NamespacedName{
				Name:      vm.Name,
				Namespace: vm.Namespace,
			},
		}
		requests = append(requests, r)
	}
	return requests
}

func (r *ProxmoxVMReconciler) proxmoxClusterToProxmoxVMs(a ctrlclient.Object) []reconcile.Request {
	proxmoxCluster, ok := a.(*infrav1.ProxmoxCluster)
	if !ok {
		return nil
	}
	clusterName, ok := proxmoxCluster.Labels[clusterv1.ClusterNameLabel]
	if !ok {
		return nil
	}

	requests := []reconcile.Request{}
	vms := &infrav1.ProxmoxVMList{}
	err := r.Client.List(goctx.Background(), vms, ctrlclient.MatchingLabels(
		map[string]string{
			clusterv1.ClusterNameLabel: clusterName,
		},
	))
	if err != nil {
		return requests
	}
	for _, vm := range vms.Items {
		r := reconcile.Request{
			NamespacedName: apitypes.NamespacedName{
				Name:      vm.Name,
				Namespace: vm.Namespace,
			},
		}
		requests = append(requests, r)
	}
	return requests
}

func (r *ProxmoxVMReconciler) ipAddressClaimToProxmoxVM(a ctrlclient.Object) []reconcile.Request {
	ipAddressClaim, ok := a.(*ipamv1.IPAddressClaim)
	if !ok {
		return nil
	}

	requests := []reconcile.Request{}
	if clusterutilv1.HasOwner(ipAddressClaim.OwnerReferences, infrav1.GroupVersion.String(), []string{"ProxmoxVM"}) {
		for _, ref := range ipAddressClaim.OwnerReferences {
			if ref.Kind == "ProxmoxVM" {
				requests = append(requests, reconcile.Request{
					NamespacedName: apitypes.NamespacedName{
						Name:      ref.Name,
						Namespace: ipAddressClaim.Namespace,
					},
				})
				break
			}
		}
	}
	return requests
}

func (r *ProxmoxVMReconciler) retrieveProxmoxSession(ctx goctx.Context, proxmoxVM *infrav1.ProxmoxVM) (*session.Session, error) {
	// Get cluster object and then get ProxmoxCluster object

	params := session.NewParams().
		WithServer(proxmoxVM.Spec.Server).
		WithDatacenter(proxmoxVM.Spec.Datacenter).
		WithUserInfo(r.ControllerContext.Username, r.ControllerContext.Password).
		WithThumbprint(proxmoxVM.Spec.Thumbprint).
		WithFeatures(session.Feature{
			EnableKeepAlive:   r.EnableKeepAlive,
			KeepAliveDuration: r.KeepAliveDuration,
		})
	cluster, err := clusterutilv1.GetClusterFromMetadata(r.ControllerContext, r.Client, proxmoxVM.ObjectMeta)
	if err != nil {
		r.Logger.Info("ProxmoxVM is missing cluster label or cluster does not exist")
		return session.GetOrCreate(r.Context,
			params)
	}

	key := ctrlclient.ObjectKey{
		Namespace: cluster.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	proxmoxCluster := &infrav1.ProxmoxCluster{}
	err = r.Client.Get(r, key, proxmoxCluster)
	if err != nil {
		r.Logger.Info("ProxmoxCluster couldn't be retrieved")
		return session.GetOrCreate(r.Context,
			params)
	}

	if proxmoxCluster.Spec.IdentityRef != nil {
		creds, err := identity.GetCredentials(ctx, r.Client, proxmoxCluster, r.Namespace)
		if err != nil {
			return nil, errors.Wrap(err, "failed to retrieve credentials from IdentityRef")
		}
		params = params.WithUserInfo(creds.Username, creds.Password)
		return session.GetOrCreate(r.Context,
			params)
	}

	// Fallback to using credentials provided to the manager
	return session.GetOrCreate(r.Context,
		params)
}
