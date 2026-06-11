package kubeutils

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNameWithLengthLimit(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		suffix string
		// either assertExact checks the full result, or assertFunc gets to inspect it.
		assertExact string
		assertFunc  func(t *testing.T, got string)
	}{
		{
			name:        "short fits unchanged",
			prefix:      "node-self-delete-",
			suffix:      "worker-1",
			assertExact: "node-self-delete-worker-1",
		},
		{
			name:        "exactly 63 chars passes through",
			prefix:      "p-",
			suffix:      strings.Repeat("a", 61),
			assertExact: "p-" + strings.Repeat("a", 61),
		},
		{
			name:   "64 chars triggers hashing",
			prefix: "p-",
			suffix: strings.Repeat("a", 62),
			assertFunc: func(t *testing.T, got string) {
				assert.LessOrEqual(t, len(got), 63)
				assert.True(t, strings.HasPrefix(got, "p-"))
				// hash is last 8 chars; separator before it
				assert.Equal(t, "-", string(got[len(got)-9]))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NameWithLengthLimit(tt.prefix, tt.suffix)
			if tt.assertExact != "" {
				assert.Equal(t, tt.assertExact, got)
			}
			if tt.assertFunc != nil {
				tt.assertFunc(t, got)
			}
		})
	}
}

func TestNameWithLengthLimit_DistinctSuffixesDoNotCollide(t *testing.T) {
	// Two distinct long suffixes that share the same head and tail would have
	// collided under the previous middle-truncation algorithm.
	prefix := "node-self-delete-"
	head := strings.Repeat("a", 40)
	tail := strings.Repeat("b", 40)
	suffixA := head + "-marker-A-" + tail
	suffixB := head + "-marker-B-" + tail

	gotA := NameWithLengthLimit(prefix, suffixA)
	gotB := NameWithLengthLimit(prefix, suffixB)

	assert.LessOrEqual(t, len(gotA), 63)
	assert.LessOrEqual(t, len(gotB), 63)
	assert.NotEqual(t, gotA, gotB, "distinct suffixes must produce distinct names")
}

func TestNameWithLengthLimit_Deterministic(t *testing.T) {
	prefix := "node-self-delete-"
	suffix := strings.Repeat("c", 80)
	a := NameWithLengthLimit(prefix, suffix)
	b := NameWithLengthLimit(prefix, suffix)
	assert.Equal(t, a, b)
}

func newRBACFakeClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	s := scheme.Scheme
	return fake.NewClientBuilder().WithScheme(s).WithObjects(objs...).Build()
}

func TestEnsureNodeDeleteRBAC_CreatesAllResources(t *testing.T) {
	ctx := context.Background()
	cli := newRBACFakeClient(t)
	ns := "embedded-cluster"
	node := "worker-1"

	require.NoError(t, EnsureNodeDeleteRBAC(ctx, cli, ns, node))

	saName := NodeDeleteServiceAccountName(node)
	roleName := NodeDeleteClusterRoleName(node)
	secretName := NodeDeleteSecretName(node)

	var sa corev1.ServiceAccount
	require.NoError(t, cli.Get(ctx, types.NamespacedName{Name: saName, Namespace: ns}, &sa))

	var role rbacv1.ClusterRole
	require.NoError(t, cli.Get(ctx, types.NamespacedName{Name: roleName}, &role))
	require.Len(t, role.Rules, 1)
	assert.Equal(t, []string{""}, role.Rules[0].APIGroups)
	assert.Equal(t, []string{"nodes"}, role.Rules[0].Resources)
	assert.Equal(t, []string{node}, role.Rules[0].ResourceNames, "ClusterRole must be scoped to this node")
	assert.ElementsMatch(t, []string{"get", "delete"}, role.Rules[0].Verbs)

	var binding rbacv1.ClusterRoleBinding
	require.NoError(t, cli.Get(ctx, types.NamespacedName{Name: roleName}, &binding))
	assert.Equal(t, "ClusterRole", binding.RoleRef.Kind)
	assert.Equal(t, roleName, binding.RoleRef.Name)
	require.Len(t, binding.Subjects, 1)
	assert.Equal(t, "ServiceAccount", binding.Subjects[0].Kind)
	assert.Equal(t, saName, binding.Subjects[0].Name)
	assert.Equal(t, ns, binding.Subjects[0].Namespace)

	var secret corev1.Secret
	require.NoError(t, cli.Get(ctx, types.NamespacedName{Name: secretName, Namespace: ns}, &secret))
	assert.Equal(t, corev1.SecretTypeServiceAccountToken, secret.Type)
	assert.Equal(t, saName, secret.Annotations["kubernetes.io/service-account.name"])
}

func TestEnsureNodeDeleteRBAC_Idempotent(t *testing.T) {
	ctx := context.Background()
	cli := newRBACFakeClient(t)
	ns := "embedded-cluster"
	node := "worker-1"

	require.NoError(t, EnsureNodeDeleteRBAC(ctx, cli, ns, node))
	require.NoError(t, EnsureNodeDeleteRBAC(ctx, cli, ns, node), "second call must not error")
}

func TestDeleteNodeDeleteRBAC_RemovesAll(t *testing.T) {
	ctx := context.Background()
	ns := "embedded-cluster"
	node := "worker-1"

	saName := NodeDeleteServiceAccountName(node)
	roleName := NodeDeleteClusterRoleName(node)
	secretName := NodeDeleteSecretName(node)

	cli := newRBACFakeClient(t,
		&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: saName, Namespace: ns}},
		&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: roleName}},
		&rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: roleName}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: ns}},
	)

	require.NoError(t, DeleteNodeDeleteRBAC(ctx, cli, ns, node))

	err := cli.Get(ctx, types.NamespacedName{Name: saName, Namespace: ns}, &corev1.ServiceAccount{})
	assert.True(t, k8serrors.IsNotFound(err), "SA should be gone: %v", err)

	err = cli.Get(ctx, types.NamespacedName{Name: roleName}, &rbacv1.ClusterRole{})
	assert.True(t, k8serrors.IsNotFound(err), "ClusterRole should be gone: %v", err)

	err = cli.Get(ctx, types.NamespacedName{Name: roleName}, &rbacv1.ClusterRoleBinding{})
	assert.True(t, k8serrors.IsNotFound(err), "ClusterRoleBinding should be gone: %v", err)

	err = cli.Get(ctx, types.NamespacedName{Name: secretName, Namespace: ns}, &corev1.Secret{})
	assert.True(t, k8serrors.IsNotFound(err), "Secret should be gone: %v", err)
}

func TestDeleteNodeDeleteRBAC_NoopWhenAbsent(t *testing.T) {
	ctx := context.Background()
	cli := newRBACFakeClient(t)
	require.NoError(t, DeleteNodeDeleteRBAC(ctx, cli, "embedded-cluster", "worker-1"),
		"deleting non-existent RBAC must not error")
}
