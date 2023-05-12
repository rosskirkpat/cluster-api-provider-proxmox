package identity

import (
	"context"
	"errors"
	"fmt"
	"strings"

	infrav1 "github.com/rosskirkpat/cluster-api-provider-proxmox/api/v1alpha1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	UsernameKey = "username"
	PasswordKey = "password"
)

type Credentials struct {
	Username string
	Password string
}

func GetCredentials(ctx context.Context, c client.Client, cluster *infrav1.ProxmoxCluster, controllerNamespace string) (*Credentials, error) {
	if err := validateInputs(c, cluster); err != nil {
		return nil, err
	}

	ref := cluster.Spec.IdentityRef
	secret := &apiv1.Secret{}
	var secretKey client.ObjectKey

	switch ref.Kind {
	case infrav1.SecretKind:
		secretKey = client.ObjectKey{
			Namespace: cluster.Namespace,
			Name:      ref.Name,
		}
	case infrav1.ProxmoxClusterIdentityKind:
		identity := &infrav1.ProxmoxClusterIdentity{}
		key := client.ObjectKey{
			Name: ref.Name,
		}
		if err := c.Get(ctx, key, identity); err != nil {
			return nil, err
		}

		if !identity.Status.Ready {
			return nil, errors.New("identity isn't ready to be used yet")
		}

		if identity.Spec.AllowedNamespaces == nil {
			return nil, errors.New("allowedNamespaces set to nil, no namespaces are allowed to use this identity")
		}

		selector, err := metav1.LabelSelectorAsSelector(&identity.Spec.AllowedNamespaces.Selector)
		if err != nil {
			return nil, errors.New("failed to build selector")
		}

		ns := &apiv1.Namespace{}
		nsKey := client.ObjectKey{
			Name: cluster.Namespace,
		}
		if err := c.Get(ctx, nsKey, ns); err != nil {
			return nil, err
		}
		if !selector.Matches(labels.Set(ns.GetLabels())) {
			return nil, fmt.Errorf("namespace %s is not allowed to use specifified identity", cluster.Namespace)
		}

		secretKey = client.ObjectKey{
			Name:      identity.Spec.SecretName,
			Namespace: controllerNamespace,
		}
	default:
		return nil, fmt.Errorf("unknown type %s used for Identity", ref.Kind)
	}

	if err := c.Get(ctx, secretKey, secret); err != nil {
		return nil, err
	}

	credentials := &Credentials{
		Username: getData(secret, UsernameKey),
		Password: getData(secret, PasswordKey),
	}

	return credentials, nil
}

func validateInputs(c client.Client, cluster *infrav1.ProxmoxCluster) error {
	if c == nil {
		return errors.New("kubernetes client is required")
	}
	if cluster == nil {
		return errors.New("proxmox cluster is required")
	}
	ref := cluster.Spec.IdentityRef
	if ref == nil {
		return errors.New("IdentityRef is required")
	}
	return nil
}

func IsSecretIdentity(cluster *infrav1.ProxmoxCluster) bool {
	if cluster == nil || cluster.Spec.IdentityRef == nil {
		return false
	}

	return cluster.Spec.IdentityRef.Kind == infrav1.SecretKind
}

func IsOwnedByIdentityOrCluster(ownerReferences []metav1.OwnerReference) bool {
	if len(ownerReferences) > 0 {
		for _, ownerReference := range ownerReferences {
			if !strings.Contains(ownerReference.APIVersion, infrav1.GroupName+"/") {
				continue
			}
			if ownerReference.Kind == "ProxmoxCluster" || ownerReference.Kind == "ProxmoxClusterIdentity" {
				return true
			}
		}
	}
	return false
}

func getData(secret *apiv1.Secret, key string) string {
	if secret.Data == nil {
		return ""
	}
	if val, ok := secret.Data[key]; ok {
		return string(val)
	}
	return ""
}
