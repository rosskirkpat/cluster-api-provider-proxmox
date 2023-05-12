package v1alpha1

import (
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (r *ProxmoxClusterTemplate) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1alpha1-proxmoxclustertemplate,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=proxmoxclustertemplates,versions=v1alpha1,name=validation.proxmoxclustertemplate.infrastructure.x-k8s.io,sideEffects=None,admissionReviewVersions=v1alpha1

var _ webhook.Validator = &ProxmoxClusterTemplate{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *ProxmoxClusterTemplate) ValidateCreate() error {
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *ProxmoxClusterTemplate) ValidateUpdate(oldRaw runtime.Object) error {
	old := oldRaw.(*ProxmoxClusterTemplate) //nolint:forcetypeassert
	if !reflect.DeepEqual(r.Spec.Template.Spec, old.Spec.Template.Spec) {
		return field.Forbidden(field.NewPath("spec", "template", "spec"), "ProxmoxClusterTemplate spec is immutable")
	}
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *ProxmoxClusterTemplate) ValidateDelete() error {
	return nil
}
