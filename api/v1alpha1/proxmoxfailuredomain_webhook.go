package v1alpha1

import (
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (r *ProxmoxFailureDomain) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1alpha1-proxmoxfailuredomain,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=proxmoxfailuredomains,versions=v1alpha1,name=validation.proxmoxfailuredomain.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1alpha1
//+kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1alpha1-proxmoxfailuredomain,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=proxmoxfailuredomains,verbs=create;update,versions=v1alpha1,name=default.proxmoxfailuredomain.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1alpha1

var _ webhook.Validator = &ProxmoxFailureDomain{}

var _ webhook.Defaulter = &ProxmoxFailureDomain{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *ProxmoxFailureDomain) ValidateCreate() error {
	var allErrs field.ErrorList

	if r.Spec.Topology.ComputeCluster == nil && r.Spec.Topology.Hosts != nil {
		allErrs = append(allErrs, field.Forbidden(field.NewPath("spec", "Topology", "ComputeCluster"), "cannot be empty if Hosts is not empty"))
	}

	if r.Spec.Region.Type == HostGroupFailureDomain {
		allErrs = append(allErrs, field.Forbidden(field.NewPath("spec", "Region", "Type"), fmt.Sprintf("region's Failure Domain type cannot be %s", r.Spec.Region.Type)))
	}

	if r.Spec.Zone.Type == HostGroupFailureDomain && r.Spec.Topology.Hosts == nil {
		allErrs = append(allErrs, field.Forbidden(field.NewPath("spec", "Topology", "Hosts"), fmt.Sprintf("cannot be nil if zone's Failure Domain type is %s", r.Spec.Zone.Type)))
	}

	if r.Spec.Region.Type == ComputeClusterFailureDomain && r.Spec.Topology.ComputeCluster == nil {
		allErrs = append(allErrs, field.Forbidden(field.NewPath("spec", "Topology", "ComputeCluster"), fmt.Sprintf("cannot be nil if region's Failure Domain type is %s", r.Spec.Region.Type)))
	}

	if r.Spec.Zone.Type == ComputeClusterFailureDomain && r.Spec.Topology.ComputeCluster == nil {
		allErrs = append(allErrs, field.Forbidden(field.NewPath("spec", "Topology", "ComputeCluster"), fmt.Sprintf("cannot be nil if zone's Failure Domain type is %s", r.Spec.Zone.Type)))
	}

	return aggregateObjErrors(r.GroupVersionKind().GroupKind(), r.Name, allErrs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *ProxmoxFailureDomain) ValidateUpdate(old runtime.Object) error {
	oldProxmoxFailureDomain, ok := old.(*ProxmoxFailureDomain)
	if !ok || !reflect.DeepEqual(r.Spec, oldProxmoxFailureDomain.Spec) {
		return field.Forbidden(field.NewPath("spec"), "ProxmoxFailureDomainSpec is immutable")
	}
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *ProxmoxFailureDomain) ValidateDelete() error {
	return nil
}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (r *ProxmoxFailureDomain) Default() {
	if r.Spec.Zone.AutoConfigure == nil {
		r.Spec.Zone.AutoConfigure = pointer.Bool(false)
	}

	if r.Spec.Region.AutoConfigure == nil {
		r.Spec.Region.AutoConfigure = pointer.Bool(false)
	}
}
