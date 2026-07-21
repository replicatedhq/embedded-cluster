package artifacts

import (
	"context"
	"testing"

	clusterv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestEnsureRegistrySecretInECNamespace(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clusterv1beta1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	kotsadmSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RegistryCredsSecretName,
			Namespace: "kotsadm",
		},
		Data: map[string][]byte{".dockerconfigjson": []byte("some-creds")},
	}

	currentInstallation := &clusterv1beta1.Installation{
		ObjectMeta: metav1.ObjectMeta{
			Name: "current-installation",
			UID:  "current-uid",
		},
	}

	t.Run("owned by an obsolete installation does not error", func(t *testing.T) {
		obsoleteInstallation := &clusterv1beta1.Installation{
			ObjectMeta: metav1.ObjectMeta{
				Name: "obsolete-installation",
				UID:  "obsolete-uid",
			},
		}

		existingSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      RegistryCredsSecretName,
				Namespace: ecNamespace,
			},
			Type: corev1.SecretTypeDockerConfigJson,
		}
		require.NoError(t, ctrl.SetControllerReference(obsoleteInstallation, existingSecret, scheme))

		cli := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(kotsadmSecret, existingSecret).
			Build()

		_, err := EnsureRegistrySecretInECNamespace(context.Background(), cli, currentInstallation)
		require.NoError(t, err)

		var got corev1.Secret
		require.NoError(t, cli.Get(context.Background(), types.NamespacedName{Name: RegistryCredsSecretName, Namespace: ecNamespace}, &got))
		require.Equal(t, kotsadmSecret.Data, got.Data)
	})

	t.Run("no existing secret creates one without an owner", func(t *testing.T) {
		cli := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(kotsadmSecret).
			Build()

		_, err := EnsureRegistrySecretInECNamespace(context.Background(), cli, currentInstallation)
		require.NoError(t, err)

		var got corev1.Secret
		require.NoError(t, cli.Get(context.Background(), types.NamespacedName{Name: RegistryCredsSecretName, Namespace: ecNamespace}, &got))
		require.Empty(t, got.OwnerReferences)
		require.Equal(t, kotsadmSecret.Data, got.Data)
	})
}
