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

func TestGetKurlInstallDirectory(t *testing.T) {
	tests := []struct {
		name        string
		configMap   *corev1.ConfigMap
		wantDir     string
		wantErr     bool
		errContains string
	}{
		{
			name: "successfully reads custom install directory",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      kurlConfigMapName,
					Namespace: kubeSystemNamespace,
				},
				Data: map[string]string{
					kurlConfigMapKey: "/custom/kurl/path",
				},
			},
			wantDir: "/custom/kurl/path",
			wantErr: false,
		},
		{
			name: "successfully reads default install directory",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      kurlConfigMapName,
					Namespace: kubeSystemNamespace,
				},
				Data: map[string]string{
					kurlConfigMapKey: "/var/lib/kurl",
				},
			},
			wantDir: "/var/lib/kurl",
			wantErr: false,
		},
		{
			name:        "configmap not found",
			configMap:   nil,
			wantDir:     "",
			wantErr:     true,
			errContains: "get kurl configmap",
		},
		{
			name: "configmap missing key - returns default",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      kurlConfigMapName,
					Namespace: kubeSystemNamespace,
				},
				Data: map[string]string{},
			},
			wantDir: kurlDefaultInstallDir,
			wantErr: false,
		},
		{
			name: "configmap has empty key value - returns default",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      kurlConfigMapName,
					Namespace: kubeSystemNamespace,
				},
				Data: map[string]string{
					kurlConfigMapKey: "",
				},
			},
			wantDir: kurlDefaultInstallDir,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			var kcli *fake.ClientBuilder
			if tt.configMap != nil {
				kcli = fake.NewClientBuilder().WithObjects(tt.configMap)
			} else {
				kcli = fake.NewClientBuilder()
			}

			client := kcli.Build()

			gotDir, err := getKurlInstallDirectory(ctx, client)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantDir, gotDir)
			}
		})
	}
}

func TestExportKurlPasswordHash(t *testing.T) {
	tests := []struct {
		name        string
		namespace   string
		secret      *corev1.Secret
		wantHash    string
		wantErr     bool
		errContains string
	}{
		{
			name:      "successfully exports password hash",
			namespace: kotsadmNamespace,
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      kotsadmPasswordSecret,
					Namespace: kotsadmNamespace,
				},
				Data: map[string][]byte{
					kotsadmPasswordSecretKey: []byte("$2a$10$abcdefghijklmnopqrstuvwxyz"),
				},
			},
			wantHash: "$2a$10$abcdefghijklmnopqrstuvwxyz",
			wantErr:  false,
		},
		{
			name:      "uses default namespace when empty string provided",
			namespace: "",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      kotsadmPasswordSecret,
					Namespace: kotsadmNamespace,
				},
				Data: map[string][]byte{
					kotsadmPasswordSecretKey: []byte("$2a$10$defaultnamespace"),
				},
			},
			wantHash: "$2a$10$defaultnamespace",
			wantErr:  false,
		},
		{
			name:      "uses custom namespace",
			namespace: "custom-namespace",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      kotsadmPasswordSecret,
					Namespace: "custom-namespace",
				},
				Data: map[string][]byte{
					kotsadmPasswordSecretKey: []byte("$2a$10$customnamespace"),
				},
			},
			wantHash: "$2a$10$customnamespace",
			wantErr:  false,
		},
		{
			name:        "secret not found",
			namespace:   kotsadmNamespace,
			secret:      nil,
			wantHash:    "",
			wantErr:     true,
			errContains: "read kotsadm-password secret from cluster",
		},
		{
			name:      "secret missing passwordBcrypt key",
			namespace: kotsadmNamespace,
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      kotsadmPasswordSecret,
					Namespace: kotsadmNamespace,
				},
				Data: map[string][]byte{
					"someOtherKey": []byte("value"),
				},
			},
			wantHash:    "",
			wantErr:     true,
			errContains: "missing required passwordBcrypt data",
		},
		{
			name:      "secret has empty passwordBcrypt value",
			namespace: kotsadmNamespace,
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      kotsadmPasswordSecret,
					Namespace: kotsadmNamespace,
				},
				Data: map[string][]byte{
					kotsadmPasswordSecretKey: []byte(""),
				},
			},
			wantHash:    "",
			wantErr:     true,
			errContains: "missing required passwordBcrypt data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			var kcli *fake.ClientBuilder
			if tt.secret != nil {
				kcli = fake.NewClientBuilder().WithObjects(tt.secret)
			} else {
				kcli = fake.NewClientBuilder()
			}

			client := kcli.Build()

			gotHash, err := exportKurlPasswordHash(ctx, client, tt.namespace)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantHash, gotHash)
			}
		})
	}
}

func TestIsECInstalled(t *testing.T) {
	// Note: This test is primarily for documentation of the function's behavior
	// Real testing would require mocking kubeconfig existence and filesystem operations
	t.Run("documents expected behavior", func(t *testing.T) {
		// This function checks for:
		// 1. EC kubeconfig exists at RuntimeConfig.PathToKubeConfig()
		// 2. EC Installation CRD exists and has at least one Installation resource
		//
		// Returns (installed bool, error)
		//
		// In a real environment, this would be tested with:
		// - Mock: Kubeconfig file existence at ${DataDir}/k0s/pki/admin.conf
		// - Setup: Create Installation resources
		// - Execute: Call isECInstalled()
		// - Assert: Verify expected return values

		ctx := context.Background()

		assert.NotPanics(t, func() {
			_, _ = isECInstalled(ctx)
		})
	})
}

func TestIsKurlCluster(t *testing.T) {
	// Note: Similar to TestIsECInstalled, this test is primarily for documentation
	// Real testing would require mocking kubeconfig existence
	t.Run("documents expected behavior", func(t *testing.T) {
		// This function checks for:
		// 1. kURL kubeconfig exists at /etc/kubernetes/admin.conf
		// 2. kURL ConfigMap exists in kube-system namespace
		//
		// Returns (isKurl bool, installDir string, kubeClient client.Client, err error)
		//
		// In a real environment, this would be tested with:
		// - Mock: Kubeconfig file existence at /etc/kubernetes/admin.conf
		// - Setup: Create kURL ConfigMap with install directory
		// - Execute: Call isKurlCluster()
		// - Assert: Verify expected return values

		ctx := context.Background()

		assert.NotPanics(t, func() {
			_, _, _, _ = isKurlCluster(ctx)
		})
	})
}
