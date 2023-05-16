package controllers

import (
	_context "context"
	"fmt"
	"reflect"
	"strings"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	clusterutilv1 "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "github.com/rosskirkpat/cluster-api-provider-proxmox/api/v1alpha1"
	"github.com/rosskirkpat/cluster-api-provider-proxmox/pkg/context"
	pkgidentity "github.com/rosskirkpat/cluster-api-provider-proxmox/pkg/identity"
	"github.com/rosskirkpat/cluster-api-provider-proxmox/pkg/record"
)

var (
	identityControlledType     = &infrav1.ProxmoxClusterIdentity{}
	identityControlledTypeName = reflect.TypeOf(identityControlledType).Elem().Name()
)

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=proxmoxclusteridentities,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=proxmoxclusteridentities/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;patch;update;delete

func AddProxmoxClusterIdentityControllerToManager(ctx *context.ControllerContext, mgr manager.Manager) error {
	var (
		controllerNameShort = fmt.Sprintf("%s-controller", strings.ToLower(identityControlledTypeName))
		controllerNameLong  = fmt.Sprintf("%s/%s/%s", ctx.Namespace, ctx.Name, controllerNameShort)
	)

	// Build the controller context.
	controllerContext := &context.ControllerContext{
		Context:  ctx,
		Name:     controllerNameShort,
		Recorder: record.New(mgr.GetEventRecorderFor(controllerNameLong)),
		Logger:   ctx.Logger.WithName(controllerNameShort),
	}

	reconciler := clusterIdentityReconciler{ControllerContext: controllerContext}

	return ctrl.NewControllerManagedBy(mgr).
		For(identityControlledType).
		WithOptions(controller.Options{MaxConcurrentReconciles: ctx.MaxConcurrentReconciles}).
		Complete(reconciler)
}

type clusterIdentityReconciler struct {
	*context.ControllerContext
}

func (r clusterIdentityReconciler) Reconcile(ctx _context.Context, req reconcile.Request) (_ reconcile.Result, reterr error) {
	// TODO(gab-satchi) consider creating a context for the clusterIdentity
	// Get ProxmoxClusterIdentity
	identity := &infrav1.ProxmoxClusterIdentity{}
	if err := r.Client.Get(r, req.NamespacedName, identity); err != nil {
		if apierrors.IsNotFound(err) {
			r.Logger.V(4).Info("ProxmoxClusterIdentity not found, won't reconcile", "key", req.NamespacedName)
			return reconcile.Result{}, nil
		}

		return reconcile.Result{}, err
	}

	// Create the patch helper.
	patchHelper, err := patch.NewHelper(identity, r.Client)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(
			err,
			"failed to init patch helper for %s %s/%s",
			identity.GroupVersionKind(),
			identity.Namespace,
			identity.Name)
	}

	defer func() {
		conditions.SetSummary(identity, conditions.WithConditions(infrav1.CredentialsAvailableCondition))

		if err := patchHelper.Patch(ctx, identity); err != nil {
			if reterr == nil {
				reterr = err
			}
			r.Logger.Error(err, "patch failed", "namespace", identity.Namespace, "name", identity.Name)
		}
	}()

	if !identity.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, identity)
	}

	// fetch secret
	secret := &corev1.Secret{}
	secretKey := client.ObjectKey{
		Namespace: r.Namespace,
		Name:      identity.Spec.SecretName,
	}
	if err := r.Client.Get(ctx, secretKey, secret); err != nil {
		conditions.MarkFalse(identity, infrav1.CredentialsAvailableCondition, infrav1.SecretNotAvailableReason, clusterv1.ConditionSeverityWarning, err.Error())
		return reconcile.Result{}, errors.Errorf("secret: %s not found in namespace: %s", secretKey.Name, secretKey.Namespace)
	}

	if !clusterutilv1.IsOwnedByObject(secret, identity) {
		ownerReferences := secret.GetOwnerReferences()
		if pkgidentity.IsOwnedByIdentityOrCluster(ownerReferences) {
			conditions.MarkFalse(identity, infrav1.CredentialsAvailableCondition, infrav1.SecretAlreadyInUseReason, clusterv1.ConditionSeverityError, "secret being used by another Cluster/ProxmoxIdentity")
			identity.Status.Ready = false
			return reconcile.Result{}, errors.New("secret being used by another Cluster/ProxmoxIdentity")
		}

		ownerReferences = append(ownerReferences, metav1.OwnerReference{
			APIVersion: infrav1.GroupVersion.String(),
			Kind:       identity.Kind,
			Name:       identity.Name,
			UID:        identity.UID,
		})
		secret.SetOwnerReferences(ownerReferences)

		if !ctrlutil.ContainsFinalizer(secret, infrav1.SecretIdentitySetFinalizer) {
			ctrlutil.AddFinalizer(secret, infrav1.SecretIdentitySetFinalizer)
		}
		err = r.Client.Update(ctx, secret)
		if err != nil {
			conditions.MarkFalse(identity, infrav1.CredentialsAvailableCondition, infrav1.SecretOwnerReferenceFailedReason, clusterv1.ConditionSeverityWarning, err.Error())
			return reconcile.Result{}, err
		}
	}

	conditions.MarkTrue(identity, infrav1.CredentialsAvailableCondition)
	identity.Status.Ready = true
	return reconcile.Result{}, nil
}

func (r clusterIdentityReconciler) reconcileDelete(ctx _context.Context, identity *infrav1.ProxmoxClusterIdentity) (reconcile.Result, error) {
	r.Logger.Info("Reconciling ProxmoxClusterIdentity delete")
	secret := &corev1.Secret{}
	secretKey := client.ObjectKey{
		Namespace: r.Namespace,
		Name:      identity.Spec.SecretName,
	}
	err := r.Client.Get(ctx, secretKey, secret)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	r.Logger.Info(fmt.Sprintf("Removing finalizer from Secret %s/%s", secret.Namespace, secret.Name))

	ctrlutil.RemoveFinalizer(secret, infrav1.SecretIdentitySetFinalizer)
	if err := r.Client.Update(ctx, secret); err != nil {
		return reconcile.Result{}, err
	}
	if err := r.Client.Delete(ctx, secret); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}
