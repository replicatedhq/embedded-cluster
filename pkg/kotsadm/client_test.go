package kotsadm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type mockClient struct {
	client.Client
	service *corev1.Service
	secret  *corev1.Secret
}

func (m *mockClient) Get(ctx context.Context, key types.NamespacedName, input client.Object, opts ...client.GetOption) error {
	switch obj := input.(type) {
	case *corev1.Service:
		if m.service == nil {
			return fmt.Errorf("service not found")
		}
		*obj = *m.service
	case *corev1.Secret:
		if m.secret == nil {
			return fmt.Errorf("secret not found")
		}
		*obj = *m.secret
	}
	return nil
}

func TestGetJoinCommand(t *testing.T) {
	tests := []struct {
		name          string
		roles         []string
		service       *corev1.Service
		secret        *corev1.Secret
		handler       http.HandlerFunc
		expectedError string
		expectedCmd   string
	}{
		{
			name:  "successful join command generation",
			roles: []string{"worker"},
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kotsadm",
					Namespace: "kotsadm",
				},
				Spec: corev1.ServiceSpec{
					ClusterIP: "127.0.0.1",
					Ports: []corev1.ServicePort{
						{},
					},
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kotsadm-authstring",
					Namespace: "kotsadm",
				},
				Data: map[string][]byte{
					"kotsadm-authstring": []byte("test-auth-token"),
				},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "POST", r.Method)
				require.Equal(t, "/api/v1/embedded-cluster/generate-node-join-command", r.URL.Path)
				require.Equal(t, "test-auth-token", r.Header.Get("Authorization"))

				var requestBody struct {
					Roles []string `json:"roles"`
				}
				err := json.NewDecoder(r.Body).Decode(&requestBody)
				require.NoError(t, err)
				require.Equal(t, []string{"worker"}, requestBody.Roles)

				response := map[string][]string{
					"command": {"embedded-cluster", "join", "--token", "test-token"},
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
			},
			expectedCmd: "embedded-cluster join --token test-token",
		},
		{
			name:          "missing service",
			roles:         []string{"worker"},
			service:       nil,
			expectedError: "unable to get kotsadm service",
		},
		{
			name:  "missing secret",
			roles: []string{"worker"},
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kotsadm",
					Namespace: "kotsadm",
				},
				Spec: corev1.ServiceSpec{
					ClusterIP: "127.0.0.1",
					Ports: []corev1.ServicePort{
						{},
					},
				},
			},
			secret:        nil,
			expectedError: "failed to get kotsadm auth slug",
		},
		{
			name:  "server returns error status",
			roles: []string{"worker"},
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kotsadm",
					Namespace: "kotsadm",
				},
				Spec: corev1.ServiceSpec{
					ClusterIP: "127.0.0.1",
					Ports: []corev1.ServicePort{
						{},
					},
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kotsadm-authstring",
					Namespace: "kotsadm",
				},
				Data: map[string][]byte{
					"kotsadm-authstring": []byte("test-auth-token"),
				},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				response := map[string]string{
					"error": "internal server error",
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
			},
			expectedError: "unexpected status code: 500",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Create a test server if we have a handler
			var server *httptest.Server
			if test.handler != nil {
				server = httptest.NewServer(test.handler)
				defer server.Close()

				// Update the service IP and port to match the test server
				serverURL, err := url.Parse(server.URL)
				require.NoError(t, err)

				host := serverURL.Hostname()
				port, err := strconv.ParseInt(serverURL.Port(), 10, 32)
				require.NoError(t, err)

				test.service.Spec.ClusterIP = host
				test.service.Spec.Ports[0].Port = int32(port)
			}

			// Create mock client
			mockK8sClient := &mockClient{
				service: test.service,
				secret:  test.secret,
			}

			// Create kotsadm client
			kotsadmClient := &Client{}

			// Call GetJoinCommand
			cmd, err := kotsadmClient.GetJoinCommand(context.Background(), mockK8sClient, test.roles)

			// Verify results
			if test.expectedError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), test.expectedError)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.expectedCmd, cmd)
			}
		})
	}
}
