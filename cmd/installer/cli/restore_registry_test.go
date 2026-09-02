package cli

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestRegistryCredentialsFromSecrets(t *testing.T) {
	cli := fake.NewClientBuilder().WithObjects(
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "unrelated", Namespace: "kotsadm"},
			Data: map[string][]byte{
				corev1.DockerConfigJsonKey: []byte(`{"auths":{"other.registry:5000":{"username":"other","password":"secret"}}}`),
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "arbitrary-restored-name", Namespace: "kotsadm"},
			Data: map[string][]byte{
				corev1.DockerConfigJsonKey: []byte(`{"auths":{"https://10.96.0.10:5000/":{"username":"embedded-cluster","password":"restored-password"}}}`),
			},
		},
	).Build()

	username, password, err := registryCredentialsFromSecrets(context.Background(), cli, "kotsadm", "10.96.0.10:5000")
	require.NoError(t, err)
	assert.Equal(t, "embedded-cluster", username)
	assert.Equal(t, "restored-password", password)
}

func TestRegistryCredentialsFromSecretsNotFound(t *testing.T) {
	cli := fake.NewClientBuilder().Build()

	_, _, err := registryCredentialsFromSecrets(context.Background(), cli, "kotsadm", "10.96.0.10:5000")
	require.ErrorContains(t, err, `unable to find restored credentials for registry "10.96.0.10:5000"`)
}
