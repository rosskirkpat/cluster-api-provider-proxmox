package v1alpha1

import (
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (z *ProxmoxDeploymentZone) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(z).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/mutate-infrastructure-cluster-x-k8s-io-v1alpha1-proxmoxdeploymentzone,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=proxmoxdeploymentzones,versions=v1alpha1,name=default.proxmoxdeploymentzone.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1alpha1

var _ webhook.Defaulter = &ProxmoxDeploymentZone{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (z *ProxmoxDeploymentZone) Default() {
	if z.Spec.ControlPlane == nil {
		z.Spec.ControlPlane = pointer.Bool(true)
	}
}
