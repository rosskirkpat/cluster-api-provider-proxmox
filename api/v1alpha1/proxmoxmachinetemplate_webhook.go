package v1alpha1

import (
	"context"
	"fmt"
	"reflect"
	"regexp"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/cluster-api/util/topology"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const machineTemplateImmutableMsg = "ProxmoxMachineTemplate spec.template.spec field is immutable. Please create a new resource instead."

func (p *ProxmoxMachineTemplateWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&ProxmoxMachineTemplate{}).
		WithValidator(p).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1alpha1-proxmoxmachinetemplate,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=proxmoxmachinetemplates,versions=v1alpha1,name=validation.proxmoxmachinetemplate.infrastructure.x-k8s.io,sideEffects=None,admissionReviewVersions=v1alpha1

// ProxmoxMachineTemplateWebhook implements a custom validation webhook for DockerMachineTemplate.
// +kubebuilder:object:generate=false
type ProxmoxMachineTemplateWebhook struct{}

var _ webhook.CustomValidator = &ProxmoxMachineTemplateWebhook{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (p *ProxmoxMachineTemplateWebhook) ValidateCreate(_ context.Context, raw runtime.Object) error {
	obj, ok := raw.(*ProxmoxMachineTemplate)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a ProxmoxMachineTemplate but got a %T", raw))
	}

	var allErrs field.ErrorList
	spec := obj.Spec.Template.Spec

	if spec.Network.PreferredAPIServerCIDR != "" {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "PreferredAPIServerCIDR"), spec.Network.PreferredAPIServerCIDR, "cannot be set, as it will be removed and is no longer used"))
	}
	if spec.ProviderID != nil {
		allErrs = append(allErrs, field.Forbidden(field.NewPath("spec", "template", "spec", "providerID"), "cannot be set in templates"))
	}
	for _, device := range spec.Network.Devices {
		if len(device.IPAddrs) != 0 {
			allErrs = append(allErrs, field.Forbidden(field.NewPath("spec", "template", "spec", "network", "devices", "ipAddrs"), "cannot be set in templates"))
		}
	}
	if spec.HardwareVersion != "" {
		// TODO add proxmox hardware version
		r := regexp.MustCompile("^pve-[1-9][0-9]?$")
		if !r.MatchString(spec.HardwareVersion) {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "template", "spec", "hardwareVersion"), spec.HardwareVersion, "should be a valid VM hardware version, example vmx-17"))
		}
	}
	return aggregateObjErrors(obj.GroupVersionKind().GroupKind(), obj.Name, allErrs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (p *ProxmoxMachineTemplateWebhook) ValidateUpdate(ctx context.Context, oldRaw runtime.Object, newRaw runtime.Object) error {
	newObj, ok := newRaw.(*ProxmoxMachineTemplate)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a ProxmoxMachineTemplate but got a %T", newRaw))
	}
	oldObj, ok := oldRaw.(*ProxmoxMachineTemplate)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a ProxmoxMachineTemplate but got a %T", oldRaw))
	}

	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a admission.Request inside context: %v", err))
	}

	var allErrs field.ErrorList
	if !topology.ShouldSkipImmutabilityChecks(req, newObj) &&
		!reflect.DeepEqual(newObj.Spec.Template.Spec, oldObj.Spec.Template.Spec) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "template", "spec"), newObj, machineTemplateImmutableMsg))
	}
	return aggregateObjErrors(newObj.GroupVersionKind().GroupKind(), newObj.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (p *ProxmoxMachineTemplateWebhook) ValidateDelete(_ context.Context, _ runtime.Object) error {
	return nil
}
