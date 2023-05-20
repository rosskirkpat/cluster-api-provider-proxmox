package v1alpha1

import (
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *ProxmoxMachineTemplateList) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1alpha1-proxmoxmachinetemplatelist,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=proxmoxmachinetemplatelist,versions=v1alpha1,name=validation.proxmoxmachinetemplatelist.infrastructure.x-k8s.io,sideEffects=None,admissionReviewVersions=v1alpha1
