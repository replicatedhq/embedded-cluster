package kubeutils

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	NodeDeleteServiceAccountPrefix = "node-self-delete-"
	NodeDeleteClusterRolePrefix    = "node-self-delete-"
	NodeDeleteSecretPrefix         = "node-self-delete-"
	NodeDeleteTokenFileName        = "node-delete-token"
)

// NodeDeleteServiceAccountName returns the ServiceAccount name for a node.
func NodeDeleteServiceAccountName(nodeName string) string {
	return NodeDeleteServiceAccountPrefix + nodeName
}

// NodeDeleteClusterRoleName returns the ClusterRole name for a node.
func NodeDeleteClusterRoleName(nodeName string) string {
	return NodeDeleteClusterRolePrefix + nodeName
}

// NodeDeleteSecretName returns the Secret name for a node's ServiceAccount token.
func NodeDeleteSecretName(nodeName string) string {
	return NodeDeleteSecretPrefix + nodeName
}

// EnsureNodeDeleteRBAC creates the ServiceAccount, ClusterRole, ClusterRoleBinding, and Secret
// needed for a node to delete itself from the cluster.
func EnsureNodeDeleteRBAC(ctx context.Context, cli client.Client, namespace, nodeName string) error {
	saName := NodeDeleteServiceAccountName(nodeName)
	roleName := NodeDeleteClusterRoleName(nodeName)
	secretName := NodeDeleteSecretName(nodeName)

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName,
			Namespace: namespace,
		},
	}
	if err := cli.Create(ctx, sa); err != nil && !k8serrors.IsAlreadyExists(err) {
		return fmt.Errorf("create serviceaccount: %w", err)
	}

	role := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: roleName,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{""},
				Resources:     []string{"nodes"},
				ResourceNames: []string{nodeName},
				Verbs:         []string{"get", "delete"},
			},
		},
	}
	if err := cli.Create(ctx, role); err != nil && !k8serrors.IsAlreadyExists(err) {
		return fmt.Errorf("create clusterrole: %w", err)
	}

	binding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: roleName,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     roleName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      saName,
				Namespace: namespace,
			},
		},
	}
	if err := cli.Create(ctx, binding); err != nil && !k8serrors.IsAlreadyExists(err) {
		return fmt.Errorf("create clusterrolebinding: %w", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
			Annotations: map[string]string{
				"kubernetes.io/service-account.name": saName,
			},
		},
		Type: corev1.SecretTypeServiceAccountToken,
	}
	if err := cli.Create(ctx, secret); err != nil && !k8serrors.IsAlreadyExists(err) {
		return fmt.Errorf("create secret: %w", err)
	}

	return nil
}

// DeleteNodeDeleteRBAC removes the per-node RBAC resources for a node that has left the cluster.
func DeleteNodeDeleteRBAC(ctx context.Context, cli client.Client, namespace, nodeName string) error {
	saName := NodeDeleteServiceAccountName(nodeName)
	roleName := NodeDeleteClusterRoleName(nodeName)
	secretName := NodeDeleteSecretName(nodeName)

	_ = cli.Delete(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
	})

	_ = cli.Delete(ctx, &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: roleName,
		},
	})

	_ = cli.Delete(ctx, &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: roleName,
		},
	})

	_ = cli.Delete(ctx, &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName,
			Namespace: namespace,
		},
	})

	return nil
}
