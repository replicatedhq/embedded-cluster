package clients

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/rest"
)

// createTestKubeConfig creates a test kubeconfig file for testing
func createTestKubeConfig(t *testing.T) string {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "kube-test-*")
	require.NoError(t, err)

	kubeconfigPath := filepath.Join(tmpDir, "kubeconfig")

	// Create a minimal valid kubeconfig
	kubeconfig := `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://kubernetes.default.svc
    certificate-authority-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJnekNDQVNpZ0F3SUJBZ0lVVkpIMHg2OHJlenFtQjFLODUrbXM0bUp3T1FFd0NnWUlLb1pJemowRUF3SXcKSGpFY01Cb0dBMVVFQXd3VFRXbHVhVzFoYkNCRlEwTWdVbTl2ZENCRFFUQWdGdzB5TlRBM01UQXhOVEUxTWpGYQpHQTh5TWprNU1EUXlOREUxTVRVeU1Wb3dIakVjTUJvR0ExVUVBd3dUVFdsdWFXMWhiQ0JGUTBNZ1VtOXZkQ0JEClFUQlpNQk1HQnlxR1NNNDlBZ0VHQ0NxR1NNNDlBd0VIQTBJQUJDSnpQNVlwUEdyRVNWVEFaaEdESjNpbGhoL0sKZ25mWXFQdTdtdVMrZXdNdHlRd0MySGo5eDNveHpWdXhUcGxiNk1qSlMyNGR1Q05iNmpVOURLMy9EMlNqUWpCQQpNQThHQTFVZEV3RUIvd1FGTUFNQkFmOHdEZ1lEVlIwUEFRSC9CQVFEQWdFR01CMEdBMVVkRGdRV0JCVEpIRU1sCjk1eHl2alZzS1REKzhNQllHazZDWkRBS0JnZ3Foa2pPUFFRREFnTkpBREJHQWlFQTN6NVk3TEYrUGI3cHJWa2wKTFBXcGlseWVXWEZlVXVCbExXMXBDR3pHaEgwQ0lRQ1o1UFhaQ1hjMGRJQytKWmsyQ2JFUE8rS3hjRXY2TllGcQpGU09ITkx4YTFnPT0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=
  name: default
contexts:
- context:
    cluster: default
    user: default
  name: default
current-context: default
users:
- name: default
  user:
    token: eyJhbGciOiJSUzI1NiIsImtpZCI6IiJ9.eyJpc3MiOiJrdWJlcm5ldGVzL3NlcnZpY2VhY2NvdW50Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9uYW1lc3BhY2UiOiJkZWZhdWx0In0.test-token
`

	err = os.WriteFile(kubeconfigPath, []byte(kubeconfig), 0644)
	require.NoError(t, err)

	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})

	return kubeconfigPath
}

// createInvalidKubeConfig creates an invalid kubeconfig file for testing
func createInvalidKubeConfig(t *testing.T) string {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "kube-test-invalid-*")
	require.NoError(t, err)

	kubeconfigPath := filepath.Join(tmpDir, "kubeconfig")

	// Create an invalid kubeconfig (invalid YAML)
	kubeconfig := `invalid yaml content {[}`

	err = os.WriteFile(kubeconfigPath, []byte(kubeconfig), 0644)
	require.NoError(t, err)

	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})

	return kubeconfigPath
}

func TestNewKubeClient(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(t *testing.T, mockGetter *MockRESTClientGetter) KubeClientOptions
		expectedError bool
		errorContains string
	}{
		{
			name: "success with valid kubeconfig path",
			setup: func(t *testing.T, mockGetter *MockRESTClientGetter) KubeClientOptions {
				kubeconfigPath := createTestKubeConfig(t)
				return KubeClientOptions{
					KubeConfigPath: kubeconfigPath,
				}
			},
			expectedError: false,
		},
		{
			name: "success with RESTClientGetter",
			setup: func(t *testing.T, mockGetter *MockRESTClientGetter) KubeClientOptions {
				mockGetter.On("ToRESTConfig").Return(&rest.Config{
					Host: "https://kubernetes.default.svc",
				}, nil)

				return KubeClientOptions{
					RESTClientGetter: mockGetter,
				}
			},
			expectedError: false,
		},
		{
			name: "error with no kubeconfig and no RESTClientGetter",
			setup: func(t *testing.T, mockGetter *MockRESTClientGetter) KubeClientOptions {
				return KubeClientOptions{}
			},
			expectedError: true,
			errorContains: "a valid kube config is required to create a kube client",
		},
		{
			name: "error with invalid kubeconfig path",
			setup: func(t *testing.T, mockGetter *MockRESTClientGetter) KubeClientOptions {
				return KubeClientOptions{
					KubeConfigPath: "/nonexistent/path/kubeconfig",
				}
			},
			expectedError: true,
			errorContains: "invalid kubeconfig path",
		},
		{
			name: "error with invalid kubeconfig content",
			setup: func(t *testing.T, mockGetter *MockRESTClientGetter) KubeClientOptions {
				kubeconfigPath := createInvalidKubeConfig(t)
				return KubeClientOptions{
					KubeConfigPath: kubeconfigPath,
				}
			},
			expectedError: true,
			errorContains: "invalid kubeconfig path",
		},
		{
			name: "error with RESTClientGetter returning error",
			setup: func(t *testing.T, mockGetter *MockRESTClientGetter) KubeClientOptions {
				mockGetter.On("ToRESTConfig").Return((*rest.Config)(nil), assert.AnError)

				return KubeClientOptions{
					RESTClientGetter: mockGetter,
				}
			},
			expectedError: true,
			errorContains: "invalid rest client getter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockGetter := &MockRESTClientGetter{}
			opts := tt.setup(t, mockGetter)

			client, err := NewKubeClient(opts)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, client)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
			}

			// Verify mock expectations if using mock
			mockGetter.AssertExpectations(t)
		})
	}
}

func TestNewMetadataClient(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(t *testing.T, mockGetter *MockRESTClientGetter) KubeClientOptions
		expectedError bool
		errorContains string
	}{
		{
			name: "success with valid kubeconfig path",
			setup: func(t *testing.T, mockGetter *MockRESTClientGetter) KubeClientOptions {
				kubeconfigPath := createTestKubeConfig(t)
				return KubeClientOptions{
					KubeConfigPath: kubeconfigPath,
				}
			},
			expectedError: false,
		},
		{
			name: "success with RESTClientGetter",
			setup: func(t *testing.T, mockGetter *MockRESTClientGetter) KubeClientOptions {
				mockGetter.On("ToRESTConfig").Return(&rest.Config{
					Host: "https://kubernetes.default.svc",
				}, nil)

				return KubeClientOptions{
					RESTClientGetter: mockGetter,
				}
			},
			expectedError: false,
		},
		{
			name: "error with no kubeconfig and no RESTClientGetter",
			setup: func(t *testing.T, mockGetter *MockRESTClientGetter) KubeClientOptions {
				return KubeClientOptions{}
			},
			expectedError: true,
			errorContains: "a valid kube config is required to create a kube client",
		},
		{
			name: "error with invalid kubeconfig path",
			setup: func(t *testing.T, mockGetter *MockRESTClientGetter) KubeClientOptions {
				return KubeClientOptions{
					KubeConfigPath: "/nonexistent/path/kubeconfig",
				}
			},
			expectedError: true,
			errorContains: "invalid kubeconfig path",
		},
		{
			name: "error with invalid kubeconfig content",
			setup: func(t *testing.T, mockGetter *MockRESTClientGetter) KubeClientOptions {
				kubeconfigPath := createInvalidKubeConfig(t)
				return KubeClientOptions{
					KubeConfigPath: kubeconfigPath,
				}
			},
			expectedError: true,
			errorContains: "invalid kubeconfig path",
		},
		{
			name: "error with RESTClientGetter returning error",
			setup: func(t *testing.T, mockGetter *MockRESTClientGetter) KubeClientOptions {
				mockGetter.On("ToRESTConfig").Return((*rest.Config)(nil), assert.AnError)

				return KubeClientOptions{
					RESTClientGetter: mockGetter,
				}
			},
			expectedError: true,
			errorContains: "invalid rest client getter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockGetter := &MockRESTClientGetter{}
			opts := tt.setup(t, mockGetter)

			client, err := NewMetadataClient(opts)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, client)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
			}

			mockGetter.AssertExpectations(t)
		})
	}
}
