package v1alpha1

import (
	"fmt"
	"net"
	"reflect"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (m *ProxmoxMachine) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(m).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1alpha1-proxmoxmachine,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=proxmoxmachines,versions=v1alpha1,name=validation.proxmoxmachine.infrastructure.x-k8s.io,sideEffects=None,admissionReviewVersions=v1alpha1
// +kubebuilder:webhook:verbs=create;update,path=/mutate-infrastructure-cluster-x-k8s-io-v1alpha1-proxmoxmachine,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=proxmoxmachines,versions=v1alpha1,name=default.proxmoxmachine.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1alpha1

var _ webhook.Validator = &ProxmoxMachine{}

var _ webhook.Defaulter = &ProxmoxMachine{}

func (m *ProxmoxMachine) Default() {
	if m.Spec.Datacenter == "" {
		m.Spec.Datacenter = "*"
	}
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (m *ProxmoxMachine) ValidateCreate() error {
	var allErrs field.ErrorList
	spec := m.Spec

	if spec.Network.PreferredAPIServerCIDR != "" {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "PreferredAPIServerCIDR"), spec.Network.PreferredAPIServerCIDR, "cannot be set, as it will be removed and is no longer used"))
	}

	for i, device := range spec.Network.Devices {
		for j, ip := range device.IPAddrs {
			if _, _, err := net.ParseCIDR(ip); err != nil {
				allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "network", fmt.Sprintf("devices[%d]", i), fmt.Sprintf("ipAddrs[%d]", j)), ip, "ip addresses should be in the CIDR format"))
			}
		}
	}

	return aggregateObjErrors(m.GroupVersionKind().GroupKind(), m.Name, allErrs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
//
//nolint:forcetypeassert
func (m *ProxmoxMachine) ValidateUpdate(old runtime.Object) error {
	newProxmoxMachine, err := runtime.DefaultUnstructuredConverter.ToUnstructured(m)
	if err != nil {
		return apierrors.NewInternalError(errors.Wrap(err, "failed to convert new ProxmoxMachine to unstructured object"))
	}

	oldProxmoxMachine, err := runtime.DefaultUnstructuredConverter.ToUnstructured(old)
	if err != nil {
		return apierrors.NewInternalError(errors.Wrap(err, "failed to convert old ProxmoxMachine to unstructured object"))
	}

	var allErrs field.ErrorList

	newProxmoxMachineSpec := newProxmoxMachine["spec"].(map[string]interface{})
	oldProxmoxMachineSpec := oldProxmoxMachine["spec"].(map[string]interface{})

	// allow changes to providerID
	delete(oldProxmoxMachineSpec, "providerID")
	delete(newProxmoxMachineSpec, "providerID")

	newProxmoxMachineNetwork := newProxmoxMachineSpec["network"].(map[string]interface{})
	oldProxmoxMachineNetwork := oldProxmoxMachineSpec["network"].(map[string]interface{})

	// allow changes to the devices
	delete(oldProxmoxMachineNetwork, "devices")
	delete(newProxmoxMachineNetwork, "devices")

	// validate that IPAddrs in updaterequest are valid.
	spec := m.Spec
	for i, device := range spec.Network.Devices {
		for j, ip := range device.IPAddrs {
			if _, _, err := net.ParseCIDR(ip); err != nil {
				allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "network", fmt.Sprintf("devices[%d]", i), fmt.Sprintf("ipAddrs[%d]", j)), ip, "ip addresses should be in the CIDR format"))
			}
		}
	}

	if !reflect.DeepEqual(oldProxmoxMachineSpec, newProxmoxMachineSpec) {
		allErrs = append(allErrs, field.Forbidden(field.NewPath("spec"), "cannot be modified"))
	}

	return aggregateObjErrors(m.GroupVersionKind().GroupKind(), m.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (m *ProxmoxMachine) ValidateDelete() error {
	return nil
}
