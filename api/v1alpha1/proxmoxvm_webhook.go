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
)

func (r *ProxmoxVM) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-proxmoxvm,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=proxmoxvms,versions=v1alpha1,name=validation.proxmoxvm.infrastructure.x-k8s.io,sideEffects=None,admissionReviewVersions=v1alpha1
// +kubebuilder:webhook:verbs=create;update,path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-proxmoxvm,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=proxmoxvms,versions=v1alpha1,name=default.proxmoxvm.infrastructure.x-k8s.io,sideEffects=None,admissionReviewVersions=v1alpha1

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (r *ProxmoxVM) Default() {
	// Set Linux as default OS value
	if r.Spec.OS == "" {
		r.Spec.OS = Linux
	}
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *ProxmoxVM) ValidateCreate() error {
	var allErrs field.ErrorList
	spec := r.Spec

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

	if r.Spec.OS == Windows && len(r.Name) > 15 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("name"), r.Name, "name has to be less than 16 characters for Windows VM"))
	}
	return aggregateObjErrors(r.GroupVersionKind().GroupKind(), r.Name, allErrs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
//
//nolint:forcetypeassert
func (r *ProxmoxVM) ValidateUpdate(old runtime.Object) error {
	newProxmoxVM, err := runtime.DefaultUnstructuredConverter.ToUnstructured(r)
	if err != nil {
		return apierrors.NewInternalError(errors.Wrap(err, "failed to convert new ProxmoxVM to unstructured object"))
	}
	oldProxmoxVM, err := runtime.DefaultUnstructuredConverter.ToUnstructured(old)
	if err != nil {
		return apierrors.NewInternalError(errors.Wrap(err, "failed to convert old ProxmoxVM to unstructured object"))
	}

	var allErrs field.ErrorList

	newProxmoxVMSpec := newProxmoxVM["spec"].(map[string]interface{})
	oldProxmoxVMSpec := oldProxmoxVM["spec"].(map[string]interface{})

	// allow changes to biosUUID
	delete(oldProxmoxVMSpec, "biosUUID")
	delete(newProxmoxVMSpec, "biosUUID")

	// allow changes to bootstrapRef
	delete(oldProxmoxVMSpec, "bootstrapRef")
	delete(newProxmoxVMSpec, "bootstrapRef")

	newProxmoxVMNetwork := newProxmoxVMSpec["network"].(map[string]interface{})
	oldProxmoxVMNetwork := oldProxmoxVMSpec["network"].(map[string]interface{})

	// allow changes to the network devices
	delete(oldProxmoxVMNetwork, "devices")
	delete(newProxmoxVMNetwork, "devices")

	// allow changes to os only if the old spec has empty OS field
	if _, ok := oldProxmoxVMSpec["os"]; !ok {
		delete(oldProxmoxVMSpec, "os")
		delete(newProxmoxVMSpec, "os")
	}

	if !reflect.DeepEqual(oldProxmoxVMSpec, newProxmoxVMSpec) {
		allErrs = append(allErrs, field.Forbidden(field.NewPath("spec"), "cannot be modified"))
	}

	return aggregateObjErrors(r.GroupVersionKind().GroupKind(), r.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *ProxmoxVM) ValidateDelete() error {
	return nil
}
