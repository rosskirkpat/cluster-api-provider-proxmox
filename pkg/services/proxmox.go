package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	gonet "net"
	"strconv"
	"time"

	"github.com/luthermonson/go-proxmox"
	"github.com/pkg/errors"
	infrav1 "github.com/rosskirkpat/cluster-api-provider-proxmox/api/v1alpha1"
	"github.com/rosskirkpat/cluster-api-provider-proxmox/pkg/context"
	"github.com/rosskirkpat/cluster-api-provider-proxmox/pkg/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

// VMService provides an API to interact with the VMs using proxmox API.
type VMService struct{}

type virtualMachineContext struct {
	context.VMContext
	Ref       *proxmox.ClusterResource
	Obj       *proxmox.VirtualMachine
	State     *infrav1.VirtualMachine
	IPAMState map[string]infrav1.NetworkDeviceSpec
}

// ReconcileVM makes sure that the VM is in the desired state by:
//  1. Creating the VM if it does not exist, then...
//  2. Updating the VM with the bootstrap data, such as the cloud-init meta and user data, before...
//  3. Powering on the VM, and finally...
//  4. Returning the real-time state of the VM to the caller
func (vms *VMService) ReconcileVM(ctx *context.VMContext) (vm infrav1.VirtualMachine, _ error) {
	// Initialize the result.
	vm = infrav1.VirtualMachine{
		Name:  ctx.ProxmoxVM.Name,
		State: infrav1.VirtualMachineStatePending,
	}

	// If there is an in-flight task associated with this VM then do not
	// reconcile the VM until the task is completed.
	if inFlight, err := reconcileInFlightTask(ctx); err != nil || inFlight {
		return vm, err
	}

	// This deferred function will trigger a reconcile event for the
	// ProxmoxVM resource once its associated task completes. If
	// there is no task for the ProxmoxVM resource then no reconcile
	// event is triggered.
	defer reconcileProxmoxVMOnTaskCompletion(ctx)

	// Before going further, we need the VM's proxmox reference.
	vmRef, err := findVMResource(ctx)
	//nolint:nestif
	if err != nil {
		if !isNotFound(err) {
			return vm, err
		}

		// If the machine was not found by VMID it means that it got deleted from proxmox directly
		if wasNotFoundByVMID(err) {
			ctx.ProxmoxVM.Status.FailureReason = capierrors.MachineStatusErrorPtr(capierrors.UpdateMachineError)
			ctx.ProxmoxVM.Status.FailureMessage = pointer.String(fmt.Sprintf("Unable to find VM by VMID %s. The vm was removed from infra", ctx.ProxmoxVM.Spec.VMID))
			return vm, err
		}

		// Otherwise, this is a new machine and the VM should be created.
		// NOTE: We are setting this condition only in case it does not exist, so we avoid to get flickering LastConditionTime
		// in case of cloning errors or powering on errors.
		if !conditions.Has(ctx.ProxmoxVM, infrav1.VMProvisionedCondition) {
			conditions.MarkFalse(ctx.ProxmoxVM, infrav1.VMProvisionedCondition, infrav1.CloningReason, clusterv1.ConditionSeverityInfo, "")
		}

		// Get the bootstrap data.
		bootstrapData, format, err := vms.getBootstrapData(ctx)
		if err != nil {
			conditions.MarkFalse(ctx.ProxmoxVM, infrav1.VMProvisionedCondition, infrav1.CloningFailedReason, clusterv1.ConditionSeverityWarning, err.Error())
			return vm, err
		}

		// Create the VM.
		err = createVM(ctx, vmRef, bootstrapData, format)
		if err != nil {
			conditions.MarkFalse(ctx.ProxmoxVM, infrav1.VMProvisionedCondition, infrav1.CloningFailedReason, clusterv1.ConditionSeverityWarning, err.Error())
			return vm, err
		}
		return vm, nil
	}

	//
	// At this point we know the VM exists, so it needs to be updated.
	//

	// fetch the new vm
	newVM, err := fetchVMByClusterResource(ctx, vmRef.ID)
	if err != nil {
		return vm, err
	}

	// Create a new virtualMachineContext to reconcile the VM.
	vmCtx := &virtualMachineContext{
		VMContext: *ctx,
		Obj:       &newVM,
		Ref:       vmRef,
		State:     &vm,
	}

	vms.reconcileVMID(vmCtx)

	if err := vms.reconcilePCIDevices(vmCtx); err != nil {
		return vm, err
	}

	if err := vms.reconcileNetworkStatus(vmCtx); err != nil {
		return vm, err
	}

	if ok, err := vms.reconcileIPAddresses(vmCtx); err != nil || !ok {
		return vm, err
	}

	if ok, err := vms.reconcileMetadata(vmCtx); err != nil || !ok {
		return vm, err
	}

	if ok, err := vms.reconcileVMGroupInfo(vmCtx); err != nil || !ok {
		return vm, err
	}

	if ok, err := vms.reconcilePowerState(vmCtx); err != nil || !ok {
		return vm, err
	}

	vms.reconcileHostInfo(vmCtx)

	if err := vms.reconcileTags(vmCtx); err != nil {
		conditions.MarkFalse(ctx.ProxmoxVM, infrav1.VMProvisionedCondition, infrav1.TagsAttachmentFailedReason, clusterv1.ConditionSeverityError, err.Error())
		return vm, err
	}

	vm.State = infrav1.VirtualMachineStateReady
	return vm, nil
}

// DestroyVM powers off and destroys a virtual machine.
func (vms *VMService) DestroyVM(ctx *context.VMContext) (infrav1.VirtualMachine, error) {
	vm := infrav1.VirtualMachine{
		Name:  ctx.ProxmoxVM.Name,
		State: infrav1.VirtualMachineStatePending,
	}

	// If there is an in-flight task associated with this VM then do not
	// reconcile the VM until the task is completed.
	if inFlight, err := reconcileInFlightTask(ctx); err != nil || inFlight {
		return vm, err
	}

	// This deferred function will trigger a reconcile event for the
	// ProxmoxVM resource once its associated task completes. If
	// there is no task for the ProxmoxVM resource then no reconcile
	// event is triggered.
	defer reconcileProxmoxVMOnTaskCompletion(ctx)

	// Before going further, we need the VM's proxmox reference.
	vmRef, err := findVMResource(ctx)
	if err != nil {
		// If the VM's MoRef could not be found then the VM no longer exists. This
		// is the desired state.
		if isNotFound(err) || isFolderNotFound(err) {
			vm.State = infrav1.VirtualMachineStateNotFound
			return vm, nil
		}
		return vm, err
	}

	//
	// At this point we know the VM exists, so it needs to be destroyed.
	//

	// fetch the vm to be deleted
	vmToDelete, err := fetchVMByClusterResource(ctx, vmRef.ID)
	if err != nil {
		return vm, err
	}

	// Create a new virtualMachineContext to reconcile the VM.
	vmCtx := &virtualMachineContext{
		VMContext: *ctx,
		Obj:       &vmToDelete,
		Ref:       vmRef,
		State:     &vm,
	}
	// Power off the VM.
	powerState, err := vms.getPowerState(vmCtx)
	if err != nil {
		return vm, err
	}
	if powerState == infrav1.VirtualMachinePowerStatePoweredOn {
		task, err := vmCtx.Obj.Stop()
		if err != nil {
			return vm, err
		}
		ctx.ProxmoxVM.Status.TaskRef = task.ID
		if err = ctx.Patch(); err != nil {
			ctx.Logger.Error(err, "patch failed", "vm", ctx.String())
			return vm, err
		}
		ctx.Logger.Info("wait for VM to be powered off")
		return vm, nil
	}

	// At this point the VM is not powered on and can be destroyed. The
	// destroy task reference will be stored and return a requeue error.
	ctx.Logger.Info("destroying vm")
	task, err := vmCtx.Obj.Delete()
	if err != nil {
		return vm, err
	}
	ctx.ProxmoxVM.Status.TaskRef = task.ID
	ctx.Logger.Info("wait for VM to be destroyed")
	return vm, nil
}

func (vms *VMService) reconcileNetworkStatus(ctx *virtualMachineContext) error {
	netStatus, err := vms.getNetworkStatus(ctx)
	if err != nil {
		return err
	}
	ctx.State.Network = netStatus
	return nil
}

// reconcileIPAddresses works to check that all the IPAddressClaim objects for the
// ProxmoxVM object have been bound.
// This function is a no-op if the ProxmoxVM has no associated IPAddressClaims.
// A discovered IPAddress is expected to contain a valid IP, Prefix and Gateway.
func (vms *VMService) reconcileIPAddresses(ctx *virtualMachineContext) (bool, error) {
	ipamState, err := BuildState(ctx.VMContext, ctx.State.Network)
	if err != nil && !errors.Is(err, ErrWaitingForIPAddr) {
		return false, err
	}
	if errors.Is(err, ErrWaitingForIPAddr) {
		conditions.MarkFalse(ctx.ProxmoxVM, infrav1.VMProvisionedCondition, infrav1.WaitingForIPAddressReason, clusterv1.ConditionSeverityInfo, err.Error())
		return false, nil
	}
	ctx.IPAMState = ipamState
	return true, nil
}

func (vms *VMService) reconcileMetadata(ctx *virtualMachineContext) (bool, error) {
	vmc := &context.VMContext{
		ControllerContext:    ctx.ControllerContext,
		ProxmoxVM:            ctx.ProxmoxVM,
		PatchHelper:          ctx.PatchHelper,
		Logger:               ctx.Logger,
		Session:              ctx.Session,
		ProxmoxFailureDomain: ctx.ProxmoxFailureDomain,
	}
	// TODO refactor to only require metadata and userdata to be set once
	// Get the bootstrap data.
	existingBootstrapData, _, err := vms.getBootstrapData(vmc)
	if err != nil {
		return false, err
	}

	data := make(map[string]interface{})
	err = ctx.Session.Get(fmt.Sprintf("/nodes/%s/qemu/%d/cloudinit", ctx.Obj.Node, ctx.Obj.VMID), data)
	if err != nil {
		return false, err
	}
	existingMetadata, err := json.Marshal(data)
	if err != nil {
		return false, err
	}

	newMetadata, err := util.GetMachineMetadata(ctx.ProxmoxVM.Name, *ctx.ProxmoxVM, ctx.IPAMState, ctx.State.Network...)
	if err != nil {
		return false, err
	}

	// If the metadata is the same then return early.
	if bytes.Equal(newMetadata, existingMetadata) {
		return true, nil
	}

	ctx.Logger.Info("updating metadata")
	err = ctx.Obj.CloudInit(ctx.Obj.VirtualMachineConfig.IDE0, string(existingBootstrapData), string(newMetadata))
	if err != nil {
		return false, errors.Wrapf(err, "unable to set metadata on vm %s", ctx)
	}

	ctx.Logger.Info("wait for VM metadata to be updated")
	return false, nil
}

func (vms *VMService) reconcilePowerState(ctx *virtualMachineContext) (bool, error) {
	powerState, err := vms.getPowerState(ctx)
	if err != nil {
		return false, err
	}
	switch powerState {
	case infrav1.VirtualMachinePowerStatePoweredOff:
		ctx.Logger.Info("powering on")
		task, err := ctx.Obj.Start()
		if err != nil {
			conditions.MarkFalse(ctx.ProxmoxVM, infrav1.VMProvisionedCondition, infrav1.PoweringOnFailedReason, clusterv1.ConditionSeverityWarning, err.Error())
			return false, errors.Wrapf(err, "failed to trigger power on op for vm %s", ctx)
		}
		conditions.MarkFalse(ctx.ProxmoxVM, infrav1.VMProvisionedCondition, infrav1.PoweringOnReason, clusterv1.ConditionSeverityInfo, "")

		// Update the ProxmoxVM.Status.TaskRef to track the power-on task.
		ctx.ProxmoxVM.Status.TaskRef = task.ID
		if err = ctx.Patch(); err != nil {
			ctx.Logger.Error(err, "patch failed", "vm", ctx.String())
			return false, err
		}

		// Once the VM is successfully powered on, a reconcile request should be
		// triggered once the VM reports IP addresses are available.
		reconcileProxmoxVMWhenNetworkIsReady(ctx, task)

		ctx.Logger.Info("wait for VM to be powered on")
		return false, nil
	case infrav1.VirtualMachinePowerStatePoweredOn:
		ctx.Logger.Info("powered on")
		return true, nil
	default:
		return false, errors.Errorf("unexpected power state %q for vm %s", powerState, ctx)
	}
}

func (vms *VMService) reconcileVMID(ctx *virtualMachineContext) {
	ctx.State.VMID = ctx.ProxmoxVM.Spec.VMID
}

func (vms *VMService) reconcilePCIDevices(ctx *virtualMachineContext) error {
	if expectedPciDevices := ctx.ProxmoxVM.Spec.VirtualMachineCloneSpec.PciDevices; len(expectedPciDevices) != 0 {
		// fetch existing PCI devices
		pciDevices := ctx.Obj.VirtualMachineConfig.MergeHostPCIs()

		var newDevices []infrav1.PCIDeviceSpec
		// iterate over the devices to be added and compare them to existing PCI devices
		for _, newDevice := range ctx.ProxmoxVM.Spec.VirtualMachineCloneSpec.PciDevices {
			for _, deviceID := range pciDevices {
				if strconv.Itoa(int(*newDevice.DeviceID)) == deviceID {
					// device is already attached
					continue
				} else {
					newDevices = append(newDevices, newDevice)
				}
			}
		}

		if len(newDevices) == 0 {
			if conditions.Has(ctx.ProxmoxVM, infrav1.PCIDevicesDetachedCondition) {
				conditions.Delete(ctx.ProxmoxVM, infrav1.PCIDevicesDetachedCondition)
			}
			ctx.Logger.V(5).Info("no new PCI devices to be added")
			return nil
		}

		// at this point, we should only be adding new pci device IDs to the existing map
		for _, newDevice := range ctx.ProxmoxVM.Spec.VirtualMachineCloneSpec.PciDevices {
			pciDevices[ctx.Obj.Node] = strconv.Itoa(int(*newDevice.DeviceID))
		}

		powerState := ctx.Obj.Status
		if powerState == proxmox.StatusVirtualMachineRunning {
			// This would arise only when the PCI device is manually removed from
			// the VM post creation.
			ctx.Logger.Info("PCI device cannot be attached in powered on state")
			conditions.MarkFalse(ctx.ProxmoxVM,
				infrav1.PCIDevicesDetachedCondition,
				infrav1.NotFoundReason,
				clusterv1.ConditionSeverityWarning,
				"PCI devices removed after VM was powered on")
			return errors.Errorf("missing PCI devices")
		}
		ctx.Logger.Info("PCI devices to be added", "number", len(newDevices))

		// update the virtual machine config with new pci devices
		ctx.Obj.VirtualMachineConfig.HostPCIs = pciDevices
		vmc := proxmox.VirtualMachineConfig{}
		if err := ctx.Session.Post(fmt.Sprintf("/nodes/%s/qemu/%d/config", ctx.Obj.Node, ctx.Obj.VMID), ctx.Obj.VirtualMachineConfig, vmc); err != nil {
			return errors.Wrapf(err, "error adding pci devices for %q", ctx)
		}
	}
	return nil
}

func (vms *VMService) getPowerState(ctx *virtualMachineContext) (infrav1.VirtualMachinePowerState, error) {
	powerState := ctx.Obj.Status

	switch powerState {
	case proxmox.StatusVirtualMachineRunning:
		return infrav1.VirtualMachinePowerStatePoweredOn, nil
	case proxmox.StatusVirtualMachineStopped:
		return infrav1.VirtualMachinePowerStatePoweredOff, nil
	case proxmox.StatusVirtualMachinePaused:
		return infrav1.VirtualMachinePowerStateSuspended, nil
	default:
		return "", errors.Errorf("unexpected power state %q for vm %s", powerState, ctx)
	}
}

func (vms *VMService) reconcileHostInfo(ctx *virtualMachineContext) {
	ctx.ProxmoxVM.Status.Host = ctx.Obj.Name
}

func (vms *VMService) setMetadata(ctx *virtualMachineContext, userdata, metadata []byte) error {
	return ctx.Obj.CloudInit(ctx.Obj.VirtualMachineConfig.IDE0, string(userdata), string(metadata))
}

func (vms *VMService) getNetworkStatus(ctx *virtualMachineContext) ([]infrav1.NetworkStatus, error) {
	ctx.Logger.V(4).Info("got all network statuses", "status", ctx.Obj.VirtualMachineConfig.Nets)
	var apiNetStatus []infrav1.NetworkStatus

	iFaces, err := ctx.Obj.AgentGetNetworkIFaces()
	if err != proxmox.ErrNotFound {
		return []infrav1.NetworkStatus{}, err
	}

	for _, iface := range iFaces {
		for _, addr := range iface.IPAddresses {
			apiNetStatus = append(apiNetStatus, infrav1.NetworkStatus{
				Connected:   true, // proxmox api does not return whether a network device is connected or disconnected
				IPAddrs:     sanitizeIPAddrs(&ctx.VMContext, iface.IPAddresses),
				MACAddr:     addr.MacAddress,
				NetworkName: iface.Name,
			})
		}
	}
	return apiNetStatus, nil
}

// getBootstrapData obtains a machine's bootstrap data from the relevant k8s secret and returns the
// data and its format.
func (vms *VMService) getBootstrapData(ctx *context.VMContext) ([]byte, bootstrapv1.Format, error) {
	if ctx.ProxmoxVM.Spec.BootstrapRef == nil {
		ctx.Logger.Info("VM has no bootstrap data")
		return nil, "", nil
	}

	secret := &corev1.Secret{}
	secretKey := apitypes.NamespacedName{
		Namespace: ctx.ProxmoxVM.Spec.BootstrapRef.Namespace,
		Name:      ctx.ProxmoxVM.Spec.BootstrapRef.Name,
	}
	if err := ctx.Client.Get(ctx, secretKey, secret); err != nil {
		return nil, "", errors.Wrapf(err, "failed to retrieve bootstrap data secret for %s", ctx)
	}

	format, ok := secret.Data["format"]
	if !ok || len(format) == 0 {
		// Bootstrap data format is missing or empty - assume cloud-config.
		format = []byte(bootstrapv1.CloudConfig)
	}

	value, ok := secret.Data["value"]
	if !ok {
		return nil, "", errors.New("error retrieving bootstrap data: secret value key is missing")
	}

	return value, bootstrapv1.Format(format), nil
}

func (vms *VMService) reconcileVMGroupInfo(ctx *virtualMachineContext) (bool, error) {
	if ctx.ProxmoxFailureDomain == nil || ctx.ProxmoxFailureDomain.Spec.Topology.Hosts == nil {
		ctx.Logger.V(5).Info("hosts topology in failure domain not defined. skipping reconcile VM group")
		return true, nil
	}

	topology := ctx.ProxmoxFailureDomain.Spec.Topology
	cluster, err := ctx.Session.Cluster()
	if err != nil {
		return false, err
	}

	hasVM := false
	vmCtx := getVMContext(ctx)
	var vm *proxmox.VirtualMachine
	for _, node := range cluster.Nodes {
		vm, err := fetchVMByClusterResource(vmCtx, node.ID)
		if err != nil {
			return false, err
		}
		if vm.Node != ctx.Ref.Node {
			err := errors.New("vm reference does not match vm ID from cluster")
			return false, errors.Wrapf(err, "unable to find VM Group %s membership", topology.Hosts.ClusterVMGroupName)
		}
		hasVM = true
	}

	if !hasVM {
		// migrate VM to expected node
		migrateTask, err := vm.Migrate(ctx.Ref.Node, ctx.Ref.Storage)
		if err != nil {
			return false, errors.Wrapf(err, "failed to migrate VM %s from node %s to node %s", ctx.ProxmoxVM.Name, vm.Node, ctx.Ref.Node)
		}
		ctx.ProxmoxVM.Status.TaskRef = migrateTask.ID
		ctx.Logger.Info("wait for VM to be migrated to new node")
		return false, nil
	}
	return true, nil
}

func (vms *VMService) reconcileTags(ctx *virtualMachineContext) error {
	if len(ctx.ProxmoxVM.Spec.TagIDs) == 0 {
		ctx.Logger.V(5).Info("no tags defined. skipping tags reconciliation")
		return nil
	}

	if ctx.Obj.VirtualMachineConfig.TagsSlice == nil {
		ctx.Obj.VirtualMachineConfig.TagsSlice = make([]string, len(ctx.ProxmoxVM.Spec.TagIDs))
		ctx.Obj.VirtualMachineConfig.TagsSlice = ctx.ProxmoxVM.Spec.TagIDs
		return nil
	}

	newTags := make([]string, len(ctx.ProxmoxVM.Spec.TagIDs)+len(ctx.Obj.VirtualMachineConfig.TagsSlice))
	//for _, newTag := range ctx.ProxmoxVM.Spec.TagIDs {
	//	ctx.Obj.VirtualMachineConfig.TagsSlice = append(ctx.Obj.VirtualMachineConfig.TagsSlice, newTag)
	//}
	newTags = append(ctx.Obj.VirtualMachineConfig.TagsSlice, ctx.ProxmoxVM.Spec.TagIDs...)
	ctx.Obj.VirtualMachineConfig.TagsSlice = newTags

	vmc := proxmox.VirtualMachineConfig{}
	err := ctx.Session.Post(fmt.Sprintf("/nodes/%s/qemu/%d/config", ctx.Obj.Node, ctx.Obj.VMID), ctx.Obj.VirtualMachineConfig, vmc)
	if err != nil {
		return errors.Wrapf(err, "failed to attach tags %v to VM %s", ctx.ProxmoxVM.Spec.TagIDs, ctx.ProxmoxVM.Name)
	}
	return nil
}

// createVM creates a new VM with the data in the VMContext passed. This method does not wait
// for the new VM to be created.
func createVM(ctx *context.VMContext, vmRef *proxmox.ClusterResource, bootstrapData []byte, format bootstrapv1.Format) error {
	// fetch the vm to clone from
	vm := proxmox.VirtualMachine{}
	err := ctx.Session.Get(fmt.Sprintf("/nodes/%s/qemu/%s", vmRef.Node, vmRef.ID), vm)
	if err != nil {
		return err
	}
	cloneSpec := ctx.ProxmoxVM.Spec.VirtualMachineCloneSpec
	full := uint8(1)

	if ctx.ProxmoxVM.Spec.CloneMode == infrav1.LinkedClone && vm.Template {
		full = uint8(0)
	}
	cluster, err := ctx.Session.Cluster()
	if err != nil {
		return err
	}
	nextId, err := cluster.NextID()
	if err != nil {
		return err
	}
	vmco := proxmox.VirtualMachineCloneOptions{
		NewID:       nextId,
		Description: "",
		Format:      "", // only valid for full clone: raw, qcow2, vmdk
		Full:        full,
		Name:        ctx.ProxmoxVM.Name,
		Pool:        cloneSpec.ResourcePool,
		SnapName:    cloneSpec.Snapshot,
		Storage:     cloneSpec.Datastore,
		Target:      vmRef.Node,
	}
	newVMId, taskId, err := vm.Clone(&vmco)
	if err != nil {
		return err
	}
	ctx.ProxmoxVM.Status.TaskRef = taskId.ID
	ctx.ProxmoxVM.Status.VmIdRef = newVMId

	// fetch the new vm
	newVM, err := fetchVMByClusterResource(ctx, strconv.Itoa(newVMId))
	if err != nil {
		return err
	}

	return newVM.CloudInit(vm.VirtualMachineConfig.IDE0, string(bootstrapData), fmt.Sprintf("instance-id: %d\nlocal-hostname: %s\n", newVM.VMID, newVM.Name))
}

// errNotFound is returned by the findVMResource function when a VM is not found.
type errNotFound struct {
	uuid            string
	byInventoryPath string
}

func (e errNotFound) Error() string {
	if e.byInventoryPath != "" {
		return fmt.Sprintf("vm with inventory path %s not found", e.byInventoryPath)
	}
	return fmt.Sprintf("vm with bios uuid %s not found", e.uuid)
}

func isNotFound(err error) bool {
	switch err.(type) {
	case errNotFound, *errNotFound:
		return true
	default:
		return false
	}
}

func isFolderNotFound(err error) bool {
	switch err.(type) {
	case error:
		return true
	default:
		return false
	}
}

func isVirtualMachineNotFound(err error) bool {
	switch err.(type) {
	case error:
		return true
	default:
		return false
	}
}

func wasNotFoundByVMID(err error) bool {
	switch err.(type) {
	case errNotFound, *errNotFound:
		if err.(errNotFound).uuid != "" && err.(errNotFound).byInventoryPath == "" {
			return true
		}
		return false
	default:
		return false
	}
}

func sanitizeIPAddrs(ctx *context.VMContext, iPAddresses []*proxmox.AgentNetworkIPAddress) []string {
	if len(iPAddresses) == 0 {
		return nil
	}
	newIPAddrs := []string{}
	for _, addr := range iPAddresses {
		if err := ErrOnLocalOnlyIPAddr(addr.IPAddress); err != nil {
			ctx.Logger.V(4).Info("ignoring IP address", "reason", err.Error())
		} else {
			newIPAddrs = append(newIPAddrs, addr.IPAddress)
		}
	}
	return newIPAddrs
}

func findVMResource(ctx *context.VMContext) (*proxmox.ClusterResource, error) {
	var vmRef *proxmox.ClusterResource
	cluster, err := ctx.Session.Cluster()
	if err != nil {
		return &proxmox.ClusterResource{}, err
	}

	vms, err := cluster.Resources("qemu")
	if err != nil {
		return &proxmox.ClusterResource{}, err
	}

	for _, vm := range vms {
		if strconv.Itoa(ctx.ProxmoxVM.Status.VmIdRef) == vm.ID {
			ctx.Logger.Info("vm found by id", "vmid", vm.ID)
			return vmRef, nil
		}
		continue
	}
	return &proxmox.ClusterResource{}, errors.New(fmt.Sprintf("failed to find vm with id: %d", ctx.ProxmoxVM.Status.VmIdRef))
}

func fetchVMByClusterResource(ctx *context.VMContext, nodeID string) (proxmox.VirtualMachine, error) {
	cluster, err := ctx.Session.Cluster()
	if err != nil {
		return proxmox.VirtualMachine{}, err
	}

	vms, err := cluster.Resources("qemu")
	if err != nil {
		return proxmox.VirtualMachine{}, err
	}
	var ref *proxmox.ClusterResource
	for _, vm := range vms {
		if nodeID == vm.ID {
			ctx.Logger.Info("vm found by id", "vmid", vm.ID)
			ref = vm
		}
		continue
	}

	vm := proxmox.VirtualMachine{}
	err = ctx.Session.Get(fmt.Sprintf("/nodes/%s/qemu/%s", ref.Node, ref.ID), vm)
	return vm, err
}

func getTask(ctx *context.VMContext) *proxmox.Task {
	if ctx.ProxmoxVM.Status.TaskRef == "" {
		return nil
	}
	var task proxmox.Task
	if err := ctx.Session.Get(ctx.ProxmoxVM.Status.TaskRef, task); err != nil {
		return nil
	}
	return &task
}

// reconcileInFlightTask determines if a task associated to the ProxmoxVM object
// is in flight or not.
func reconcileInFlightTask(ctx *context.VMContext) (bool, error) {
	// Check to see if there is an in-flight task.
	task := getTask(ctx)
	return checkAndRetryTask(ctx, task)
}

// checkAndRetryTask verifies whether the task exists and if the
// task should be reconciled which is determined by the task state retryAfter value set.
func checkAndRetryTask(ctx *context.VMContext, task *proxmox.Task) (bool, error) {
	// If no task was found then make sure to clear the ProxmoxVM
	// resource's Status.TaskRef field.
	if task == nil {
		ctx.ProxmoxVM.Status.TaskRef = ""
		return false, nil
	}

	// Since RetryAfter is set, the last task failed. Wait for the RetryAfter time duration to expire
	// before checking/resetting the task.
	if !ctx.ProxmoxVM.Status.RetryAfter.IsZero() && time.Now().Before(ctx.ProxmoxVM.Status.RetryAfter.Time) {
		return false, errors.Errorf("last task failed retry after %v", ctx.ProxmoxVM.Status.RetryAfter)
	}

	// Otherwise the course of action is determined by the state of the task.
	logger := ctx.Logger.WithName(task.ID)
	logger.Info("task found", "status", task.Status, "id", task.ID)
	switch task.Status {
	case "pending":
		logger.Info("task is still pending", "id", task.ID)
		return true, nil
	case proxmox.TaskRunning:
		logger.Info("task is still running", "id", task.ID)
		return true, nil
	case "success":
		logger.Info("task is a success", "id", task.ID)
		ctx.ProxmoxVM.Status.TaskRef = ""
		return false, nil
	case "error":
		logger.Info("task failed", "id", task.ID)
		conditions.MarkFalse(ctx.ProxmoxVM, infrav1.VMProvisionedCondition, infrav1.TaskFailure, clusterv1.ConditionSeverityInfo, task.ExitStatus)

		// Instead of directly re-queuing the failed task, wait for the RetryAfter duration to pass
		// before resetting the taskRef from the ProxmoxVM status.
		if ctx.ProxmoxVM.Status.RetryAfter.IsZero() {
			ctx.ProxmoxVM.Status.RetryAfter = metav1.Time{Time: time.Now().Add(1 * time.Minute)}
		} else {
			ctx.ProxmoxVM.Status.TaskRef = ""
			ctx.ProxmoxVM.Status.RetryAfter = metav1.Time{}
		}
		return true, nil
	default:
		return false, errors.Errorf("unknown task status %q for %q", task.Status, ctx.Name)
	}
}

func reconcileProxmoxVMWhenNetworkIsReady(ctx *virtualMachineContext, powerOnTask *proxmox.Task) {
	reconcileProxmoxVMOnChannel(
		&ctx.VMContext,
		func() (<-chan []interface{}, <-chan error, error) {
			// Wait for the VM to be powered on.
			err := powerOnTask.WaitFor(600)
			if err != nil {
				return nil, nil, errors.Wrapf(err, "failed to wait for power on op for vm %s", ctx)
			}
			powerState := ctx.Obj.IsRunning()
			if !powerState {
				return nil, nil, errors.Errorf(
					"unexpected power state %v for vm %s",
					powerState, ctx)
			}

			// Wait for all NICs to have valid MAC addresses.
			if err := waitForMacAddresses(ctx); err != nil {
				return nil, nil, errors.Wrapf(err, "failed to wait for mac addresses for vm %s", ctx)
			}

			// Get all the MAC addresses. This is done separately from waiting
			// for all NICs to have MAC addresses in order to ensure the order
			// of the retrieved MAC addresses matches the order of the device
			// specs, and not the property change order.
			_, macToDeviceIndex, deviceToMacIndex, err := getMacAddresses(ctx)
			if err != nil {
				return nil, nil, errors.Wrapf(err, "failed to get mac addresses for vm %s", ctx)
			}

			// Wait for the IP addresses to show up for the VM.
			chanIPAddresses, chanErrs := waitForIPAddresses(ctx, macToDeviceIndex, deviceToMacIndex)

			// Trigger a reconciliation every time a new IP is discovered.
			chanOfLoggerKeysAndValues := make(chan []interface{})
			go func() {
				for ip := range chanIPAddresses {
					chanOfLoggerKeysAndValues <- []interface{}{
						"reason", "network",
						"ipAddress", ip,
					}
				}
			}()
			return chanOfLoggerKeysAndValues, chanErrs, nil
		})
}

func reconcileProxmoxVMOnTaskCompletion(ctx *context.VMContext) {
	task := getTask(ctx)
	if task == nil {
		ctx.Logger.V(4).Info(
			"skipping reconcile ProxmoxVM on task completion",
			"reason", "no-task")
		return
	}
	taskRef := task.ID
	newTask := proxmox.NewTask(proxmox.UPID(taskRef), ctx.Session.Client)

	ctx.Logger.Info(
		"enqueuing reconcile request on task completion",
		"task-ref-id", taskRef,
		"task-node", task.Node,
		"task-type", task.Type,
		"task-pid", task.PID)

	reconcileProxmoxVMOnFuncCompletion(ctx, func() ([]interface{}, error) {
		err := newTask.WaitFor(600)

		// An error is only returned if the process of waiting for the result
		// failed, *not* if the task itself failed.
		if err != nil && !newTask.IsFailed {
			return nil, err
		}
		// do not queue in the event channel when task fails as we don't
		// want to retry right away
		if newTask.Status == "error" {
			ctx.Logger.Info("async task wait failed")
			return nil, errors.Errorf("task failed")
		}

		return []interface{}{
			"reason", "task",
			"task-ref-id", taskRef,
			"task-node", task.Node,
			"task-type", task.Type,
			"task-pid", task.PID,
		}, nil
	})
}

func reconcileProxmoxVMOnFuncCompletion(ctx *context.VMContext, waitFn func() (loggerKeysAndValues []interface{}, _ error)) {
	obj := ctx.ProxmoxVM.DeepCopy()
	gvk := obj.GetObjectKind().GroupVersionKind()

	// Wait on the function to complete in a background goroutine.
	go func() {
		loggerKeysAndValues, err := waitFn()
		if err != nil {
			ctx.Logger.Error(err, "failed to wait on func")
			return
		}

		// Once the task has completed (successfully or otherwise), trigger
		// a reconcile event for the associated resource by sending a
		// GenericEvent into the event channel for the resource type.
		ctx.Logger.Info("triggering GenericEvent", loggerKeysAndValues...)
		eventChannel := ctx.GetGenericEventChannelFor(gvk)
		eventChannel <- event.GenericEvent{
			Object: obj,
		}
	}()
}

func reconcileProxmoxVMOnChannel(ctx *context.VMContext, waitFn func() (<-chan []interface{}, <-chan error, error)) {
	obj := ctx.ProxmoxVM.DeepCopy()
	gvk := obj.GetObjectKind().GroupVersionKind()

	// Send a generic event for every set of logger keys/values received
	// on the channel.
	go func() {
		chanOfLoggerKeysAndValues, chanErrs, err := waitFn()
		if err != nil {
			ctx.Logger.Error(err, "failed to wait on func")
			return
		}
		for {
			select {
			case loggerKeysAndValues := <-chanOfLoggerKeysAndValues:
				if loggerKeysAndValues == nil {
					return
				}
				go func() {
					// Trigger a reconcile event for the associated resource by
					// sending a GenericEvent into the event channel for the resource
					// type.
					ctx.Logger.Info("triggering GenericEvent", loggerKeysAndValues...)
					eventChannel := ctx.GetGenericEventChannelFor(gvk)
					eventChannel <- event.GenericEvent{
						Object: obj,
					}
				}()
			case err := <-chanErrs:
				if err != nil {
					ctx.Logger.Error(err, "error occurred while waiting to trigger a generic event")
				}
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

// waitForMacAddresses waits for all configured network devices to have
// valid MAC addresses.
func waitForMacAddresses(ctx *virtualMachineContext) error {
	iFaces, err := ctx.Obj.AgentGetNetworkIFaces()
	if err != proxmox.ErrNotFound {
		return err
	}

	go func(ifaces []*proxmox.AgentNetworkIface) bool {
		for _, iface := range ifaces {
			for _, addr := range iface.IPAddresses {
				if addr.MacAddress == "" {
					return false
				}
			}
		}
		return true
	}(iFaces)
	return nil
}

// getMacAddresses gets the MAC addresses for all network devices.
// This happens separately from waitForMacAddresses to ensure returned order of
// devices matches the spec and not order in which the property changes were
// noticed.
func getMacAddresses(ctx *virtualMachineContext) ([]string, map[string]int, map[int]string, error) {
	var (
		macAddresses         []string
		macToDeviceSpecIndex = map[string]int{}
		deviceSpecIndexToMac = map[int]string{}
	)
	iFaces, err := ctx.Obj.AgentGetNetworkIFaces()
	if err != proxmox.ErrNotFound {
		return nil, nil, nil, err
	}
	i := 0
	for _, iface := range iFaces {
		for _, addr := range iface.IPAddresses {
			macAddresses = append(macAddresses, addr.MacAddress)
			macToDeviceSpecIndex[addr.MacAddress] = i
			deviceSpecIndexToMac[i] = addr.MacAddress
			i++
		}
	}

	return macAddresses, macToDeviceSpecIndex, deviceSpecIndexToMac, nil
}

// waitForIPAddresses waits for all network devices that should be getting an
// IP address to have an IP address. This is any network device that specifies a
// network name and DHCP for v4 or v6 or one or more static IP addresses.
// The gocyclo detector is disabled for this function as it is difficult to
// rewrite much simpler due to the maps used to track state and the lambdas
// that use the maps.
//
//nolint:gocyclo,gocognit
func waitForIPAddresses(
	ctx *virtualMachineContext,
	macToDeviceIndex map[string]int,
	deviceToMacIndex map[int]string) (<-chan string, <-chan error) {
	var (
		chanErrs          = make(chan error)
		chanIPAddresses   = make(chan string)
		macToHasIPv4Lease = map[string]struct{}{}
		macToHasIPv6Lease = map[string]struct{}{}
		macToSkipped      = map[string]map[string]struct{}{}
		macToHasStaticIP  = map[string]map[string]struct{}{}
	)

	// Initialize the nested maps early.
	for mac := range macToDeviceIndex {
		macToSkipped[mac] = map[string]struct{}{}
		macToHasStaticIP[mac] = map[string]struct{}{}
	}

	iFaces, err := ctx.Obj.AgentGetNetworkIFaces()
	if err != proxmox.ErrNotFound {
		chanErrs <- errors.Errorf("unable to fetch network interfaces for vmid %d", ctx.Obj.VMID)
	}

	for _, iface := range iFaces {
		for _, addr := range iface.IPAddresses {
			if addr.MacAddress == "" || iface.IPAddresses == nil {
				continue
			}
			// Ignore any that don't correspond to a network
			// device spec.
			deviceSpecIndex, ok := macToDeviceIndex[addr.MacAddress]
			if !ok {
				chanErrs <- errors.Errorf("unknown device spec index for mac %s while waiting for ip addresses for vm %s", addr.MacAddress, ctx)
			}
			if deviceSpecIndex < 0 || deviceSpecIndex >= len(ctx.ProxmoxVM.Spec.Network.Devices) {
				chanErrs <- errors.Errorf("invalid device spec index %d for mac %s while waiting for ip addresses for vm %s", deviceSpecIndex, addr.MacAddress, ctx)
			}

			// Get the network device spec that corresponds to the MAC.
			deviceSpec := ctx.ProxmoxVM.Spec.Network.Devices[deviceSpecIndex]

			// Look at each IP and determine whether a reconciliation has
			// been triggered for the IP.
			for _, ip := range iface.IPAddresses {
				discoveredIP := ip.IPAddress

				// Ignore link-local addresses.
				if err := ErrOnLocalOnlyIPAddr(discoveredIP); err != nil {
					if _, ok := macToSkipped[addr.MacAddress][discoveredIP]; !ok {
						ctx.Logger.Info("ignoring IP address", "reason", err.Error())
						macToSkipped[addr.MacAddress][discoveredIP] = struct{}{}
					}
					continue
				}

				// Check to see if the IP is in the list of the device
				// spec's static IP addresses.
				isStatic := false
				for _, specIP := range deviceSpec.IPAddrs {
					// The static IP assigned to the VM is required in the CIDR format
					ip, _, _ := gonet.ParseCIDR(specIP)
					if discoveredIP == ip.String() {
						isStatic = true
						break
					}
				}

				// If it's a static IP then check to see if the IP has
				// triggered a reconciliation yet.
				switch {
				case isStatic:
					if _, ok := macToHasStaticIP[addr.MacAddress][discoveredIP]; !ok {
						// No reconcile yet. Record the IP send it to the
						// channel.
						ctx.Logger.Info(
							"discovered IP address",
							"addressType", "static",
							"addressValue", discoveredIP)
						macToHasStaticIP[addr.MacAddress][discoveredIP] = struct{}{}
						chanIPAddresses <- discoveredIP
					}
				case gonet.ParseIP(discoveredIP).To4() != nil:
					// An IPv4 address...
					if deviceSpec.DHCP4 {
						// Has an IPv4 lease been discovered yet?
						if _, ok := macToHasIPv4Lease[addr.MacAddress]; !ok {
							ctx.Logger.Info(
								"discovered IP address",
								"addressType", "dhcp4",
								"addressValue", discoveredIP)
							macToHasIPv4Lease[addr.MacAddress] = struct{}{}
							chanIPAddresses <- discoveredIP
						}
					}
				default:
					// An IPv6 address..
					if deviceSpec.DHCP6 {
						// Has an IPv6 lease been discovered yet?
						if _, ok := macToHasIPv6Lease[addr.MacAddress]; !ok {
							ctx.Logger.Info(
								"discovered IP address",
								"addressType", "dhcp6",
								"addressValue", discoveredIP)
							macToHasIPv6Lease[addr.MacAddress] = struct{}{}
							chanIPAddresses <- discoveredIP
						}
					}
				}
			}
		}
	}

	// Determine whether the wait operation is over by whether
	//  the VM has the requested IP addresses.
	for i, deviceSpec := range ctx.ProxmoxVM.Spec.Network.Devices {
		mac, ok := deviceToMacIndex[i]
		if !ok {
			chanErrs <- errors.Errorf("invalid mac index %d waiting for ip addresses for vm %s", i, ctx)

		}
		// If the device spec requires DHCP4 then the Wait is not
		// over if there is no IPv4 lease.
		if deviceSpec.DHCP4 {
			if _, ok := macToHasIPv4Lease[mac]; !ok {
				ctx.Logger.Info(
					"the VM is missing the requested IP address",
					"addressType", "dhcp4")
			}
		}
		// If the device spec requires DHCP6 then the Wait is not
		// over if there is no IPv4 lease.
		if deviceSpec.DHCP6 {
			if _, ok := macToHasIPv6Lease[mac]; !ok {
				ctx.Logger.Info(
					"the VM is missing the requested IP address",
					"addressType", "dhcp6")
			}
		}
		// If the device spec requires static IP addresses, the wait
		// is not over if the device lacks one of those addresses.
		for _, specIP := range deviceSpec.IPAddrs {
			ip, _, _ := gonet.ParseCIDR(specIP)
			if _, ok := macToHasStaticIP[mac][ip.String()]; !ok {
				ctx.Logger.Info(
					"the VM is missing the requested IP address",
					"addressType", "static",
					"addressValue", specIP)
			}
		}
	}

	ctx.Logger.Info("the VM has all of the requested IP addresses")

	close(chanIPAddresses)
	close(chanErrs)

	return chanIPAddresses, chanErrs
}

// ErrOnLocalOnlyIPAddr returns an error if the provided IP address is
// accessible only on the VM's guest OS.
func ErrOnLocalOnlyIPAddr(addr string) error {
	var reason string
	a := gonet.ParseIP(addr)
	switch {
	case len(a) == 0:
		reason = "invalid"
	case a.IsUnspecified():
		reason = "unspecified"
	case a.IsLinkLocalMulticast():
		reason = "link-local-mutlicast"
	case a.IsLinkLocalUnicast():
		reason = "link-local-unicast"
	case a.IsLoopback():
		reason = "loopback"
	}
	if reason != "" {
		return errors.Errorf("failed to validate ip addr=%v: %s", addr, reason)
	}
	return nil
}

func getVMContext(ctx *virtualMachineContext) *context.VMContext {
	return &context.VMContext{
		ControllerContext:    ctx.ControllerContext,
		ProxmoxVM:            ctx.ProxmoxVM,
		PatchHelper:          ctx.PatchHelper,
		Logger:               ctx.Logger,
		Session:              ctx.Session,
		ProxmoxFailureDomain: ctx.ProxmoxFailureDomain,
	}
}
