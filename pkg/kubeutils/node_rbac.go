package kubeutils

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
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
	NodeDeleteTokenJobPrefix       = "node-delete-token-"
	NodeDeleteTokenFileName        = "node-delete-token"
)

// NameWithLengthLimit returns a DNS-1123-subdomain-safe name no longer than 63
// characters. When prefix+suffix fits, it is returned unchanged. Otherwise the
// suffix is truncated from the right and an 8-char content hash of the original
// suffix is appended so distinct suffixes cannot collide.
func NameWithLengthLimit(prefix, suffix string) string {
	candidate := prefix + suffix
	if len(candidate) <= 63 {
		return candidate
	}
	sum := sha256.Sum256([]byte(suffix))
	hash := hex.EncodeToString(sum[:])[:8]
	// 1 for the '-' separator between truncated suffix and hash.
	room := 63 - len(prefix) - 1 - len(hash)
	if room < 0 {
		room = 0
	}
	if room > len(suffix) {
		room = len(suffix)
	}
	return prefix + suffix[:room] + "-" + hash
}

// NodeDeleteTokenJobName returns the Job name for a node's token delivery job.
func NodeDeleteTokenJobName(nodeName string) string {
	return NameWithLengthLimit(NodeDeleteTokenJobPrefix, nodeName)
}

// NodeDeleteServiceAccountName returns the ServiceAccount name for a node.
func NodeDeleteServiceAccountName(nodeName string) string {
	return NameWithLengthLimit(NodeDeleteServiceAccountPrefix, nodeName)
}

// NodeDeleteClusterRoleName returns the ClusterRole name for a node.
func NodeDeleteClusterRoleName(nodeName string) string {
	return NameWithLengthLimit(NodeDeleteClusterRolePrefix, nodeName)
}

// NodeDeleteSecretName returns the Secret name for a node's ServiceAccount token.
func NodeDeleteSecretName(nodeName string) string {
	return NameWithLengthLimit(NodeDeleteSecretPrefix, nodeName)
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

// DeleteNodeDeleteRBAC removes the per-node RBAC resources and token delivery job for a node
// that has left the cluster.
func DeleteNodeDeleteRBAC(ctx context.Context, cli client.Client, namespace, nodeName string) error {
	saName := NodeDeleteServiceAccountName(nodeName)
	roleName := NodeDeleteClusterRoleName(nodeName)
	secretName := NodeDeleteSecretName(nodeName)
	jobName := NodeDeleteTokenJobName(nodeName)

	var errs []error
	if err := cli.Delete(ctx, &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: namespace,
		},
	}); err != nil && !k8serrors.IsNotFound(err) {
		errs = append(errs, fmt.Errorf("delete job: %w", err))
	}

	if err := cli.Delete(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
	}); err != nil && !k8serrors.IsNotFound(err) {
		errs = append(errs, fmt.Errorf("delete secret: %w", err))
	}

	if err := cli.Delete(ctx, &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: roleName,
		},
	}); err != nil && !k8serrors.IsNotFound(err) {
		errs = append(errs, fmt.Errorf("delete clusterrolebinding: %w", err))
	}

	if err := cli.Delete(ctx, &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: roleName,
		},
	}); err != nil && !k8serrors.IsNotFound(err) {
		errs = append(errs, fmt.Errorf("delete clusterrole: %w", err))
	}

	if err := cli.Delete(ctx, &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName,
			Namespace: namespace,
		},
	}); err != nil && !k8serrors.IsNotFound(err) {
		errs = append(errs, fmt.Errorf("delete serviceaccount: %w", err))
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
