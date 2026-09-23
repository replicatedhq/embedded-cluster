package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

func TestWaitForRegistry(t *testing.T) {
	for _, readyStatus := range []int{http.StatusOK, http.StatusFound, http.StatusUnauthorized} {
		t.Run(http.StatusText(readyStatus), func(t *testing.T) {
			requests := 0
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				assert.Equal(t, "/v2/", r.URL.Path)
				_, _, ok := r.BasicAuth()
				assert.False(t, ok)
				if requests == 1 {
					http.Error(w, "not ready", http.StatusServiceUnavailable)
					return
				}
				w.WriteHeader(readyStatus)
			}))
			defer server.Close()

			httpClient := server.Client()
			httpClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			}
			err := waitForRegistry(context.Background(), httpClient, server.URL, time.Millisecond, time.Second)
			require.NoError(t, err)
			assert.Equal(t, 2, requests)
		})
	}
}
