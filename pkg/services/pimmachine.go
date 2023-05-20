package services

import (
	goctx "context"
	"encoding/json"
	"github.com/pkg/errors"
	infrav1 "github.com/rosskirkpat/cluster-api-provider-proxmox/api/v1alpha1"
	"github.com/rosskirkpat/cluster-api-provider-proxmox/pkg/context"
	infrautilv1 "github.com/rosskirkpat/cluster-api-provider-proxmox/pkg/util"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/utils/integer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	clusterutilv1 "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"strings"
)

type PimMachineService struct{}

func (v *PimMachineService) FetchProxmoxMachine(c client.Client, name types.NamespacedName) (context.MachineContext, error) {
	proxmoxMachine := &infrav1.ProxmoxMachine{}
	err := c.Get(goctx.Background(), name, proxmoxMachine)

	return &context.PIMMachineContext{ProxmoxMachine: proxmoxMachine}, err
}

func (v *PimMachineService) FetchProxmoxCluster(c client.Client, cluster *clusterv1.Cluster, machineContext context.MachineContext) (context.MachineContext, error) {
	ctx, ok := machineContext.(*context.PIMMachineContext)
	if !ok {
		return nil, errors.New("received unexpected PIMMachineContext type")
	}
	proxmoxCluster := &infrav1.ProxmoxCluster{}
	proxmoxClusterName := client.ObjectKey{
		Namespace: machineContext.GetObjectMeta().Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	err := c.Get(goctx.Background(), proxmoxClusterName, proxmoxCluster)

	ctx.ProxmoxCluster = proxmoxCluster
	return ctx, err
}

func (v *PimMachineService) ReconcileDelete(c context.MachineContext) error {
	ctx, ok := c.(*context.PIMMachineContext)
	if !ok {
		return errors.New("received unexpected PIMMachineContext type")
	}

	vm, err := v.findVM(ctx)
	// Attempt to find the associated ProxmoxVM resource.
	if err != nil {
		return err
	}

	if vm != nil && vm.GetDeletionTimestamp().IsZero() {
		// If the ProxmoxVM was found, and it's not already enqueued for
		// deletion, go ahead and attempt to delete it.
		if err := ctx.Client.Delete(ctx, vm); err != nil {
			return err
		}
	}

	// ProxmoxMachine wraps a ProxmoxVM, so we are mirroring status from the underlying ProxmoxVM
	// in order to provide evidences about machine deletion.
	conditions.SetMirror(ctx.ProxmoxMachine, infrav1.VMProvisionedCondition, vm)
	return nil
}

func (v *PimMachineService) SyncFailureReason(c context.MachineContext) (bool, error) {
	ctx, ok := c.(*context.PIMMachineContext)
	if !ok {
		return false, errors.New("received unexpected PIMMachineContext type")
	}

	proxmoxVM, err := v.findVM(ctx)
	if err != nil {
		return false, err
	}
	if proxmoxVM != nil {
		// Reconcile ProxmoxMachine's failures
		ctx.ProxmoxMachine.Status.FailureReason = proxmoxVM.Status.FailureReason
		ctx.ProxmoxMachine.Status.FailureMessage = proxmoxVM.Status.FailureMessage
	}

	return ctx.ProxmoxMachine.Status.FailureReason != nil || ctx.ProxmoxMachine.Status.FailureMessage != nil, err
}

func (v *PimMachineService) ReconcileNormal(c context.MachineContext) (bool, error) {
	ctx, ok := c.(*context.PIMMachineContext)
	if !ok {
		return false, errors.New("received unexpected PIMMachineContext type")
	}
	proxmoxVM, err := v.findVM(ctx)
	if err != nil && !apierrors.IsNotFound(err) {
		return false, err
	}

	vm, err := v.createOrPatchProxmoxVM(ctx, proxmoxVM)
	if err != nil {
		ctx.Logger.Error(err, "error creating or patching VM", "proxmoxVM", proxmoxVM)
		return false, err
	}

	// Convert the VM resource to unstructured data.
	vmData, err := runtime.DefaultUnstructuredConverter.ToUnstructured(vm)
	if err != nil {
		return false, errors.Wrapf(err,
			"failed to convert %s to unstructured data",
			vm.GetObjectKind().GroupVersionKind().String())
	}
	vmObj := &unstructured.Unstructured{Object: vmData}
	vmObj.SetGroupVersionKind(vm.GetObjectKind().GroupVersionKind())
	vmObj.SetAPIVersion(vm.GetObjectKind().GroupVersionKind().GroupVersion().String())
	vmObj.SetKind(vm.GetObjectKind().GroupVersionKind().Kind)

	// Waits the VM's ready state.
	if ok, err := v.waitReadyState(ctx, vmObj); !ok {
		if err != nil {
			return false, errors.Wrapf(err, "unexpected error while reconciling ready state for %s", ctx)
		}
		ctx.Logger.Info("waiting for ready state")
		// ProxmoxMachine wraps a ProxmoxVM, so we are mirroring status from the underlying ProxmoxVM
		// in order to provide evidences about machine provisioning while provisioning is actually happening.
		conditions.SetMirror(ctx.ProxmoxMachine, infrav1.VMProvisionedCondition, conditions.UnstructuredGetter(vmObj))
		return true, nil
	}

	// Reconcile the ProxmoxMachine's provider ID using the VM's BIOS UUID.
	if ok, err := v.reconcileProviderID(ctx, vmObj); !ok {
		if err != nil {
			return false, errors.Wrapf(err, "unexpected error while reconciling provider ID for %s", ctx)
		}
		ctx.Logger.Info("provider ID is not reconciled")
		return true, nil
	}

	// Reconcile the ProxmoxMachine's node addresses from the VM's IP addresses.
	if ok, err := v.reconcileNetwork(ctx, vmObj); !ok {
		if err != nil {
			return false, errors.Wrapf(err, "unexpected error while reconciling network for %s", ctx)
		}
		ctx.Logger.Info("network is not reconciled")
		conditions.MarkFalse(ctx.ProxmoxMachine, infrav1.VMProvisionedCondition, infrav1.WaitingForNetworkAddressesReason, clusterv1.ConditionSeverityInfo, "")
		return true, nil
	}

	ctx.ProxmoxMachine.Status.Ready = true
	return false, nil
}

func (v *PimMachineService) GetHostInfo(c context.MachineContext) (string, error) {
	ctx, ok := c.(*context.PIMMachineContext)
	if !ok {
		return "", errors.New("received unexpected PIMMachineContext type")
	}

	proxmoxVM := &infrav1.ProxmoxVM{}
	if err := ctx.Client.Get(ctx, client.ObjectKey{
		Namespace: ctx.ProxmoxMachine.Namespace,
		Name:      generateVMObjectName(ctx, ctx.Machine.Name),
	}, proxmoxVM); err != nil {
		return "", err
	}

	if conditions.IsTrue(proxmoxVM, infrav1.VMProvisionedCondition) {
		return proxmoxVM.Status.Host, nil
	}
	ctx.Logger.V(4).Info("VMProvisionedCondition is set to false", "proxmoxVM", proxmoxVM.Name)
	return "", nil
}

func (v *PimMachineService) findVM(ctx *context.PIMMachineContext) (*infrav1.ProxmoxVM, error) {
	// Get ready to find the associated ProxmoxVM resource.
	vm := &infrav1.ProxmoxVM{}
	vmKey := types.NamespacedName{
		Namespace: ctx.ProxmoxMachine.Namespace,
		Name:      generateVMObjectName(ctx, ctx.Machine.Name),
	}
	// Attempt to find the associated ProxmoxVM resource.
	if err := ctx.Client.Get(ctx, vmKey, vm); err != nil {
		return nil, err
	}
	return vm, nil
}

func (v *PimMachineService) waitReadyState(ctx *context.PIMMachineContext, vm *unstructured.Unstructured) (bool, error) {
	ready, ok, err := unstructured.NestedBool(vm.Object, "status", "ready")
	if !ok {
		if err != nil {
			return false, errors.Wrapf(err,
				"unexpected error when getting status.ready from %s %s/%s for %s",
				vm.GroupVersionKind(),
				vm.GetNamespace(),
				vm.GetName(),
				ctx)
		}
		ctx.Logger.Info("status.ready not found",
			"vmGVK", vm.GroupVersionKind().String(),
			"vmNamespace", vm.GetNamespace(),
			"vmName", vm.GetName())
		return false, nil
	}
	if !ready {
		ctx.Logger.Info("status.ready is false",
			"vmGVK", vm.GroupVersionKind().String(),
			"vmNamespace", vm.GetNamespace(),
			"vmName", vm.GetName())
		return false, nil
	}

	return true, nil
}

func (v *PimMachineService) reconcileProviderID(ctx *context.PIMMachineContext, vm *unstructured.Unstructured) (bool, error) {
	biosUUID, ok, err := unstructured.NestedString(vm.Object, "spec", "biosUUID")
	if !ok {
		if err != nil {
			return false, errors.Wrapf(err,
				"unexpected error when getting spec.biosUUID from %s %s/%s for %s",
				vm.GroupVersionKind(),
				vm.GetNamespace(),
				vm.GetName(),
				ctx)
		}
		ctx.Logger.Info("spec.biosUUID not found",
			"vmGVK", vm.GroupVersionKind().String(),
			"vmNamespace", vm.GetNamespace(),
			"vmName", vm.GetName())
		return false, nil
	}
	if biosUUID == "" {
		ctx.Logger.Info("spec.biosUUID is empty",
			"vmGVK", vm.GroupVersionKind().String(),
			"vmNamespace", vm.GetNamespace(),
			"vmName", vm.GetName())
		return false, nil
	}

	providerID := infrautilv1.ConvertUUIDToProviderID(biosUUID)
	if providerID == "" {
		return false, errors.Errorf("invalid BIOS UUID %s from %s %s/%s for %s",
			biosUUID,
			vm.GroupVersionKind(),
			vm.GetNamespace(),
			vm.GetName(),
			ctx)
	}
	if ctx.ProxmoxMachine.Spec.ProviderID == nil || *ctx.ProxmoxMachine.Spec.ProviderID != providerID {
		ctx.ProxmoxMachine.Spec.ProviderID = &providerID
		ctx.Logger.Info("updated provider ID", "provider-id", providerID)
	}

	return true, nil
}

//nolint:nestif
func (v *PimMachineService) reconcileNetwork(ctx *context.PIMMachineContext, vm *unstructured.Unstructured) (bool, error) {
	var errs []error
	if networkStatusListOfIfaces, ok, _ := unstructured.NestedSlice(vm.Object, "status", "network"); ok {
		var networkStatusList []infrav1.NetworkStatus
		for i, networkStatusListMemberIface := range networkStatusListOfIfaces {
			if buf, err := json.Marshal(networkStatusListMemberIface); err != nil {
				ctx.Logger.Error(err,
					"unsupported data for member of status.network list",
					"index", i)
				errs = append(errs, err)
			} else {
				var networkStatus infrav1.NetworkStatus
				err := json.Unmarshal(buf, &networkStatus)
				if err == nil && networkStatus.MACAddr == "" {
					err = errors.New("macAddr is required")
					errs = append(errs, err)
				}
				if err != nil {
					ctx.Logger.Error(err,
						"unsupported data for member of status.network list",
						"index", i, "data", string(buf))
					errs = append(errs, err)
				} else {
					networkStatusList = append(networkStatusList, networkStatus)
				}
			}
		}
		ctx.ProxmoxMachine.Status.Network = networkStatusList
	}

	if addresses, ok, _ := unstructured.NestedStringSlice(vm.Object, "status", "addresses"); ok {
		var machineAddresses []clusterv1.MachineAddress
		for _, addr := range addresses {
			machineAddresses = append(machineAddresses, clusterv1.MachineAddress{
				Type:    clusterv1.MachineExternalIP,
				Address: addr,
			})
		}
		ctx.ProxmoxMachine.Status.Addresses = machineAddresses
	}

	if len(ctx.ProxmoxMachine.Status.Addresses) == 0 {
		ctx.Logger.Info("waiting on IP addresses")
		return false, kerrors.NewAggregate(errs)
	}

	return true, nil
}

func (v *PimMachineService) createOrPatchProxmoxVM(ctx *context.PIMMachineContext, proxmoxVM *infrav1.ProxmoxVM) (runtime.Object, error) {
	// Create or update the ProxmoxVM resource.
	vm := &infrav1.ProxmoxVM{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ctx.ProxmoxMachine.Namespace,
			Name:      generateVMObjectName(ctx, ctx.Machine.Name),
		},
	}
	mutateFn := func() (err error) {
		// Ensure the ProxmoxMachine is marked as an owner of the ProxmoxVM.
		vm.SetOwnerReferences(clusterutilv1.EnsureOwnerRef(
			vm.OwnerReferences,
			metav1.OwnerReference{
				APIVersion: ctx.ProxmoxMachine.APIVersion,
				Kind:       ctx.ProxmoxMachine.Kind,
				Name:       ctx.ProxmoxMachine.Name,
				UID:        ctx.ProxmoxMachine.UID,
			}))

		// Instruct the ProxmoxVM to use the CAPI bootstrap data resource.
		// TODO: BootstrapRef field should be replaced with BootstrapSecret of type string
		vm.Spec.BootstrapRef = &corev1.ObjectReference{
			APIVersion: "v1",
			Kind:       "Secret",
			Name:       *ctx.Machine.Spec.Bootstrap.DataSecretName,
			Namespace:  ctx.Machine.ObjectMeta.Namespace,
		}

		// Initialize the ProxmoxVM's labels map if it is nil.
		if vm.Labels == nil {
			vm.Labels = map[string]string{}
		}

		// Ensure the ProxmoxVM has a label that can be used when searching for
		// resources associated with the target cluster.
		vm.Labels[clusterv1.ClusterNameLabel] = ctx.Machine.Labels[clusterv1.ClusterNameLabel]

		// For convenience, add a label that makes it easy to figure out if the
		// ProxmoxVM resource is part of some control plane.
		if val, ok := ctx.Machine.Labels[clusterv1.MachineControlPlaneLabel]; ok {
			vm.Labels[clusterv1.MachineControlPlaneLabel] = val
		}

		// Copy the ProxmoxMachine's VM clone spec into the ProxmoxVM's
		// clone spec.
		ctx.ProxmoxMachine.Spec.VirtualMachineCloneSpec.DeepCopyInto(&vm.Spec.VirtualMachineCloneSpec)

		// If Failure Domain is present on CAPI machine, use that to override the vm clone spec.
		if overrideFunc, ok := v.generateOverrideFunc(ctx); ok {
			overrideFunc(vm)
		}

		// Several of the ProxmoxVM's clone spec properties can be derived
		// from multiple places. The order is:
		//
		//   1. From the Machine.Spec.FailureDomain
		//   2. From the ProxmoxMachine.Spec (the DeepCopyInto above)
		//   3. From the ProxmoxCluster.Spec
		if vm.Spec.Server == "" {
			vm.Spec.Server = ctx.ProxmoxCluster.Spec.Server
		}
		if vm.Spec.Thumbprint == "" {
			vm.Spec.Thumbprint = ctx.ProxmoxCluster.Spec.Thumbprint
		}
		if proxmoxVM != nil {
			vm.Spec.VMID = proxmoxVM.Spec.VMID
		}
		return nil
	}

	vmKey := types.NamespacedName{
		Namespace: vm.Namespace,
		Name:      vm.Name,
	}
	result, err := ctrlutil.CreateOrPatch(ctx, ctx.Client, vm, mutateFn)
	if err != nil {
		ctx.Logger.Error(
			err,
			"failed to CreateOrPatch ProxmoxVM",
			"namespace",
			vm.Namespace,
			"name",
			vm.Name,
		)
		return nil, err
	}
	switch result {
	case ctrlutil.OperationResultNone:
		ctx.Logger.Info(
			"no update required for vm",
			"vm",
			vmKey,
		)
	case ctrlutil.OperationResultCreated:
		ctx.Logger.Info(
			"created vm",
			"vm",
			vmKey,
		)
	case ctrlutil.OperationResultUpdated:
		ctx.Logger.Info(
			"updated vm",
			"vm",
			vmKey,
		)
	case ctrlutil.OperationResultUpdatedStatus:
		ctx.Logger.Info(
			"updated vm and vm status",
			"vm",
			vmKey,
		)
	case ctrlutil.OperationResultUpdatedStatusOnly:
		ctx.Logger.Info(
			"updated vm status",
			"vm",
			vmKey,
		)
	}

	return vm, nil
}

// generateVMObjectName returns a new VM object name in specific cases, otherwise return the same
// passed in the parameter.
func generateVMObjectName(ctx *context.PIMMachineContext, machineName string) string {
	// Windows VM names must have 15 characters length at max.
	if ctx.ProxmoxMachine.Spec.OS == infrav1.Windows && len(machineName) > 15 {
		return strings.TrimSuffix(machineName[0:9], "-") + "-" + machineName[len(machineName)-5:]
	}
	return machineName
}

// generateOverrideFunc returns a function which can override the values in the ProxmoxVM Spec
// with the values from the FailureDomain (if any) set on the owner CAPI machine.
//
//nolint:nestif
func (v *PimMachineService) generateOverrideFunc(ctx *context.PIMMachineContext) (func(vm *infrav1.ProxmoxVM), bool) {
	failureDomainName := ctx.Machine.Spec.FailureDomain
	if failureDomainName == nil {
		return nil, false
	}

	// Use the failureDomain name to fetch the proxmoxDeploymentZone object
	var proxmoxDeploymentZone infrav1.ProxmoxDeploymentZone
	if err := ctx.Client.Get(ctx, client.ObjectKey{Name: *failureDomainName}, &proxmoxDeploymentZone); err != nil {
		ctx.Logger.Error(err, "unable to fetch proxmox deployment zone", "name", *failureDomainName)
		return nil, false
	}

	var proxmoxFailureDomain infrav1.ProxmoxFailureDomain
	if err := ctx.Client.Get(ctx, client.ObjectKey{Name: proxmoxDeploymentZone.Spec.FailureDomain}, &proxmoxFailureDomain); err != nil {
		ctx.Logger.Error(err, "unable to fetch failure domain", "name", proxmoxDeploymentZone.Spec.FailureDomain)
		return nil, false
	}

	overrideWithFailureDomainFunc := func(vm *infrav1.ProxmoxVM) {
		vm.Spec.Server = proxmoxDeploymentZone.Spec.Server
		vm.Spec.Datacenter = proxmoxFailureDomain.Spec.Topology.Datacenter
		if proxmoxDeploymentZone.Spec.PlacementConstraint.Folder != "" {
			vm.Spec.Folder = proxmoxDeploymentZone.Spec.PlacementConstraint.Folder
		}
		if proxmoxDeploymentZone.Spec.PlacementConstraint.ResourcePool != "" {
			vm.Spec.ResourcePool = proxmoxDeploymentZone.Spec.PlacementConstraint.ResourcePool
		}
		if proxmoxFailureDomain.Spec.Topology.Datastore != "" {
			vm.Spec.Datastore = proxmoxFailureDomain.Spec.Topology.Datastore
		}
		if len(proxmoxFailureDomain.Spec.Topology.Networks) > 0 {
			vm.Spec.Network.Devices = overrideNetworkDeviceSpecs(vm.Spec.Network.Devices, proxmoxFailureDomain.Spec.Topology.Networks)
		}
	}
	return overrideWithFailureDomainFunc, true
}

// overrideNetworkDeviceSpecs updates the network devices with the network definitions from the PlacementConstraint.
// The substitution is done based on the order in which the network devices have been defined.
//
// In case there are more network definitions than the number of network devices specified, the definitions are appended to the list.
func overrideNetworkDeviceSpecs(deviceSpecs []infrav1.NetworkDeviceSpec, networks []string) []infrav1.NetworkDeviceSpec {
	index, length := 0, len(networks)

	devices := make([]infrav1.NetworkDeviceSpec, 0, integer.IntMax(length, len(deviceSpecs)))
	// override the networks on the VM spec with placement constraint network definitions
	for i := range deviceSpecs {
		vmNetworkDeviceSpec := deviceSpecs[i]
		if i < length {
			index++
			vmNetworkDeviceSpec.NetworkName = networks[i]
		}
		devices = append(devices, vmNetworkDeviceSpec)
	}
	// append the remaining network definitions to the VM spec
	for ; index < length; index++ {
		devices = append(devices, infrav1.NetworkDeviceSpec{
			NetworkName: networks[index],
		})
	}

	return devices
}
