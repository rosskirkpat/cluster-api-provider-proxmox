package controllers

import (
	goctx "context"
	"fmt"
	"k8s.io/apimachinery/pkg/runtime"
	"reflect"
	"strings"
	"time"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	clusterutilv1 "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlbldr "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	infrav1 "github.com/rosskirkpat/cluster-api-provider-proxmox/api/v1alpha1"
	"github.com/rosskirkpat/cluster-api-provider-proxmox/pkg/constants"
	"github.com/rosskirkpat/cluster-api-provider-proxmox/pkg/context"
	"github.com/rosskirkpat/cluster-api-provider-proxmox/pkg/record"
	"github.com/rosskirkpat/cluster-api-provider-proxmox/pkg/services"
	"github.com/rosskirkpat/cluster-api-provider-proxmox/pkg/util"
)

const hostInfoErrStr = "host info cannot be used as a label value"

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=proxmoxmachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=proxmoxmachines/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=proxmoxmachines/finalizers,verbs=update
// +kubebuilder:rbac:groups=vmware.infrastructure.cluster.x-k8s.io,resources=proxmoxmachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=vmware.infrastructure.cluster.x-k8s.io,resources=proxmoxmachines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=vmware.infrastructure.cluster.x-k8s.io,resources=proxmoxmachinetemplates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=vmware.infrastructure.cluster.x-k8s.io,resources=proxmoxmachinetemplates/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines,verbs=get;list;watch;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines/status,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=nodes;events;configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps/status,verbs=get;update;patch

// AddMachineControllerToManager adds the machine controller to the provided manager.
func AddMachineControllerToManager(ctx *context.ControllerContext, mgr manager.Manager, controlledType client.Object) error {

	var (
		controlledTypeName  = reflect.TypeOf(controlledType).Elem().Name()
		controlledTypeGVK   = infrav1.GroupVersion.WithKind(controlledTypeName)
		controllerNameShort = fmt.Sprintf("%s-controller", strings.ToLower(controlledTypeName))
		controllerNameLong  = fmt.Sprintf("%s/%s/%s", ctx.Namespace, ctx.Name, controllerNameShort)
	)

	// Build the controller context.
	controllerContext := &context.ControllerContext{
		Name:     controllerNameShort,
		Recorder: record.New(mgr.GetEventRecorderFor(controllerNameLong)),
		Logger:   ctx.Logger.WithName(controllerNameShort),
	}

	builder := ctrl.NewControllerManagedBy(mgr).
		// Watch the controlled, infrastructure resource.
		For(controlledType).
		// Watch the CAPI resource that owns this infrastructure resource.
		Watches(
			&source.Kind{Type: &clusterv1.Machine{}},
			handler.EnqueueRequestsFromMapFunc(clusterutilv1.MachineToInfrastructureMapFunc(controlledTypeGVK)),
		).
		// Watch a GenericEvent channel for the controlled resource.
		//
		// This is useful when there are events outside of Kubernetes that
		// should cause a resource to be synchronized, such as a goroutine
		// waiting on some asynchronous, external task to complete.
		Watches(
			&source.Channel{Source: ctx.GetGenericEventChannelFor(controlledTypeGVK)},
			&handler.EnqueueRequestForObject{},
		).
		WithOptions(controller.Options{MaxConcurrentReconciles: ctx.MaxConcurrentReconciles})

	r := ProxmoxMachineReconciler{
		ControllerContext: controllerContext,
		VMService:         &services.PimMachineService{},
	}

	// Watch any ProxmoxVM resources owned by the controlled type.
	builder.Watches(
		&source.Kind{Type: &infrav1.ProxmoxVM{}},
		&handler.EnqueueRequestForOwner{OwnerType: controlledType, IsController: false},
		ctrlbldr.WithPredicates(predicate.Funcs{
			// ignore creation events since this controller is responsible for
			// the creation of the type.
			CreateFunc: func(e event.CreateEvent) bool {
				return false
			},
		}),
	)

	c, err := builder.Build(&r)
	if err != nil {
		return err
	}

	err = c.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		handler.EnqueueRequestsFromMapFunc(r.clusterToProxmoxMachines),
		predicates.ClusterUnpausedAndInfrastructureReady(r.Logger))
	if err != nil {
		return err
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ProxmoxMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.ProxmoxMachine{}).
		Complete(r)
}

// ProxmoxMachineReconciler reconciles a ProxmoxMachine object
type ProxmoxMachineReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	*context.ControllerContext
	VMService       services.ProxmoxMachineService
	networkProvider services.NetworkProvider
}

// Reconcile ensures the back-end state reflects the Kubernetes resource state intent.
func (r *ProxmoxMachineReconciler) Reconcile(_ goctx.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	var machineContext context.MachineContext
	logger := r.Logger.WithName(req.Namespace).WithName(req.Name)
	logger.V(3).Info("Starting Reconcile ProxmoxMachine")

	// Fetch ProxmoxMachine object and populate the machine context
	machineContext, err := r.VMService.FetchProxmoxMachine(r.Client, req.NamespacedName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Fetch the CAPI Machine and CAPI Cluster.
	machine, err := clusterutilv1.GetOwnerMachine(r, r.Client, machineContext.GetObjectMeta())
	if err != nil {
		return reconcile.Result{}, err
	}
	if machine == nil {
		logger.V(2).Info("waiting on Machine controller to set OwnerRef on infra machine")
		return reconcile.Result{}, nil
	}

	cluster := r.fetchCAPICluster(machine, machineContext.GetProxmoxMachine())

	// Create the patch helper.
	patchHelper, err := patch.NewHelper(machineContext.GetProxmoxMachine(), r.Client)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(
			err,
			"failed to init patch helper for %s/%s",
			machineContext.GetObjectMeta().Namespace,
			machineContext.GetObjectMeta().Name)
	}
	machineContext.SetBaseMachineContext(&context.BaseMachineContext{
		ControllerContext: r.ControllerContext,
		Cluster:           cluster,
		Machine:           machine,
		Logger:            logger,
		PatchHelper:       patchHelper,
	})
	// always patch the ProxmoxMachine object
	defer func() {
		// always update the readyCondition.
		conditions.SetSummary(machineContext.GetProxmoxMachine(),
			conditions.WithConditions(
				infrav1.VMProvisionedCondition,
			),
		)

		// Patch the ProxmoxMachine resource.
		if err := machineContext.Patch(); err != nil {
			if reterr == nil {
				reterr = err
			}
			machineContext.GetLogger().Error(err, "patch failed", "machine", machineContext.String())
		}
	}()

	if !machineContext.GetObjectMeta().DeletionTimestamp.IsZero() {
		return r.reconcileDelete(machineContext)
	}

	// Checking whether cluster is nil here as we still want to allow deletion even if cluster is not found.
	if cluster == nil {
		return reconcile.Result{}, nil
	}

	// Fetch the ProxmoxCluster and update the machine context
	machineContext, err = r.VMService.FetchProxmoxCluster(r.Client, cluster, machineContext)
	if err != nil {
		logger.Info("unable to retrieve ProxmoxCluster", "error", err)
		return reconcile.Result{}, nil
	}

	// Handle non-deleted machines
	return r.reconcileNormal(machineContext)
}

func (r *ProxmoxMachineReconciler) reconcileDelete(ctx context.MachineContext) (reconcile.Result, error) {
	ctx.GetLogger().Info("Handling deleted ProxmoxMachine")
	conditions.MarkFalse(ctx.GetProxmoxMachine(), infrav1.VMProvisionedCondition, clusterv1.DeletingReason, clusterv1.ConditionSeverityInfo, "")

	if err := r.VMService.ReconcileDelete(ctx); err != nil {
		if apierrors.IsNotFound(err) {
			// The VM is deleted so remove the finalizer.
			ctrlutil.RemoveFinalizer(ctx.GetProxmoxMachine(), infrav1.MachineFinalizer)
			return reconcile.Result{}, nil
		}
		conditions.MarkFalse(ctx.GetProxmoxMachine(), infrav1.VMProvisionedCondition, clusterv1.DeletionFailedReason, clusterv1.ConditionSeverityWarning, "")
		return reconcile.Result{}, err
	}

	// VM is being deleted
	return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
}

func (r *ProxmoxMachineReconciler) reconcileNormal(ctx context.MachineContext) (reconcile.Result, error) {
	machineFailed, err := r.VMService.SyncFailureReason(ctx)
	if err != nil && !apierrors.IsNotFound(err) {
		return reconcile.Result{}, err
	}

	// If the ProxmoxMachine is in an error state, return early.
	if machineFailed {
		ctx.GetLogger().Info("Error state detected, skipping reconciliation")
		return reconcile.Result{}, nil
	}

	// If the ProxmoxMachine doesn't have our finalizer, add it.
	ctrlutil.AddFinalizer(ctx.GetProxmoxMachine(), infrav1.MachineFinalizer)

	//nolint:gocritic
	if !ctx.GetCluster().Status.InfrastructureReady {
		ctx.GetLogger().Info("Cluster infrastructure is not ready yet")
		conditions.MarkFalse(ctx.GetProxmoxMachine(), infrav1.VMProvisionedCondition, infrav1.WaitingForClusterInfrastructureReason, clusterv1.ConditionSeverityInfo, "")
		return reconcile.Result{}, nil
	}

	// Make sure bootstrap data is available and populated.
	if ctx.GetMachine().Spec.Bootstrap.DataSecretName == nil {
		if !util.IsControlPlaneMachine(ctx.GetProxmoxMachine()) && !conditions.IsTrue(ctx.GetCluster(), clusterv1.ControlPlaneInitializedCondition) {
			ctx.GetLogger().Info("Waiting for the control plane to be initialized")
			conditions.MarkFalse(ctx.GetProxmoxMachine(), infrav1.VMProvisionedCondition, clusterv1.WaitingForControlPlaneAvailableReason, clusterv1.ConditionSeverityInfo, "")
			return ctrl.Result{}, nil
		}
		ctx.GetLogger().Info("Waiting for bootstrap data to be available")
		conditions.MarkFalse(ctx.GetProxmoxMachine(), infrav1.VMProvisionedCondition, infrav1.WaitingForBootstrapDataReason, clusterv1.ConditionSeverityInfo, "")
		return reconcile.Result{}, nil
	}

	requeue, err := r.VMService.ReconcileNormal(ctx)
	if err != nil {
		return reconcile.Result{}, err
	} else if requeue {
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// The machine is patched at the last stage before marking the VM as provisioned
	// This makes sure that the ProxmoxMachine exists and is in a Running state
	// before attempting to patch.
	err = r.patchMachineLabelsWithHostInfo(ctx)
	if err != nil {
		r.Logger.Error(err, "failed to patch machine with host info label", "machine ", ctx.GetMachine().Name)
		return reconcile.Result{}, err
	}

	conditions.MarkTrue(ctx.GetProxmoxMachine(), infrav1.VMProvisionedCondition)
	return reconcile.Result{}, nil
}

// patchMachineLabelsWithHostInfo adds the Proxmox host information as a label to the Machine object.
// The Proxmox host information is added with the CAPI node label prefix
// which would be added onto the node by the CAPI controllers.
func (r *ProxmoxMachineReconciler) patchMachineLabelsWithHostInfo(ctx context.MachineContext) error {
	hostInfo, err := r.VMService.GetHostInfo(ctx)
	if err != nil {
		return err
	}

	info := util.SanitizeHostInfoLabel(hostInfo)
	errs := validation.IsValidLabelValue(info)
	if len(errs) > 0 {
		err := errors.Errorf("%s: %s", hostInfoErrStr, strings.Join(errs, ","))
		r.Logger.Error(err, hostInfoErrStr, "info", hostInfo)
		return err
	}

	machine := ctx.GetMachine()
	patchHelper, err := patch.NewHelper(machine, r.Client)
	if err != nil {
		return err
	}

	labels := machine.GetLabels()
	labels[constants.ProxmoxHostInfoLabel] = info
	machine.Labels = labels

	return patchHelper.Patch(r, machine)
}

func (r *ProxmoxMachineReconciler) clusterToProxmoxMachines(a client.Object) []reconcile.Request {
	requests := []reconcile.Request{}
	machines, err := util.GetProxmoxMachinesInCluster(goctx.Background(), r.Client, a.GetNamespace(), a.GetName())
	if err != nil {
		return requests
	}
	for _, m := range machines {
		r := reconcile.Request{
			NamespacedName: apitypes.NamespacedName{
				Name:      m.Name,
				Namespace: m.Namespace,
			},
		}
		requests = append(requests, r)
	}
	return requests
}

func (r *ProxmoxMachineReconciler) fetchCAPICluster(machine *clusterv1.Machine, proxmoxMachine metav1.Object) *clusterv1.Cluster {
	cluster, err := clusterutilv1.GetClusterFromMetadata(r, r.Client, machine.ObjectMeta)
	if err != nil {
		r.Logger.Info("Machine is missing cluster label or cluster does not exist")
		return nil
	}
	if annotations.IsPaused(cluster, proxmoxMachine) {
		r.Logger.V(4).Info("ProxmoxMachine %s/%s linked to a cluster that is paused", proxmoxMachine.GetNamespace(), proxmoxMachine.GetName())
		return nil
	}

	return cluster
}
