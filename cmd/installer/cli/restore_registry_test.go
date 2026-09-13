package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
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

func TestWaitForRegistryReadyRetriesUntilV2EndpointIsReady(t *testing.T) {
	var requests atomic.Int32
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, "/v2/", r.URL.Path)
		if requests.Add(1) < 3 {
			return nil, errors.New("connection refused")
		}
		return httpResponse(http.StatusBadRequest), nil
	})}

	err := waitForRegistryReadyWithBackoff(context.Background(), client, "registry.test:5000", wait.Backoff{
		Steps:    3,
		Duration: time.Millisecond,
		Factor:   1,
	})
	require.NoError(t, err)
	assert.Equal(t, int32(3), requests.Load())
}

func TestWaitForRegistryReadyReportsLastTransportError(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return nil, errors.New("connection refused")
	})}

	err := waitForRegistryReadyWithBackoff(context.Background(), client, "registry.test:5000", wait.Backoff{Steps: 1})
	require.ErrorContains(t, err, "connection refused")
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func httpResponse(statusCode int) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Status:     fmt.Sprintf("%d %s", statusCode, http.StatusText(statusCode)),
		Body:       io.NopCloser(strings.NewReader("")),
	}
}

func TestRegistryCredentialsFromSecretsNotFound(t *testing.T) {
	cli := fake.NewClientBuilder().Build()

	_, _, err := registryCredentialsFromSecrets(context.Background(), cli, "kotsadm", "10.96.0.10:5000")
	require.ErrorContains(t, err, `unable to find restored credentials for registry "10.96.0.10:5000"`)
}
