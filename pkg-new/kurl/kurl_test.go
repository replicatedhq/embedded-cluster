package kurl

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetInstallDirectory(t *testing.T) {
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
					Name:      "kurl-config",
					Namespace: "kube-system",
				},
				Data: map[string]string{
					"kurl_install_directory": "/custom/kurl/path",
				},
			},
			wantDir: "/custom/kurl/path",
			wantErr: false,
		},
		{
			name: "successfully reads default install directory",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kurl-config",
					Namespace: "kube-system",
				},
				Data: map[string]string{
					"kurl_install_directory": "/var/lib/kurl",
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
			errContains: "not found",
		},
		{
			name: "configmap missing key - returns default",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kurl-config",
					Namespace: "kube-system",
				},
				Data: map[string]string{},
			},
			wantDir: "/var/lib/kurl",
			wantErr: false,
		},
		{
			name: "configmap has empty key value - returns default",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kurl-config",
					Namespace: "kube-system",
				},
				Data: map[string]string{
					"kurl_install_directory": "",
				},
			},
			wantDir: "/var/lib/kurl",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Create fake client with or without the configmap
			var kcli client.Client
			if tt.configMap != nil {
				kcli = fake.NewClientBuilder().WithObjects(tt.configMap).Build()
			} else {
				kcli = fake.NewClientBuilder().Build()
			}

			gotDir, err := getInstallDirectory(ctx, kcli)

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

func TestGetPasswordHash(t *testing.T) {
	tests := []struct {
		name        string
		secret      *corev1.Secret
		namespace   string
		wantHash    string
		wantErr     bool
		errContains string
	}{
		{
			name: "successfully reads password hash",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kotsadm-password",
					Namespace: "kotsadm",
				},
				Data: map[string][]byte{
					"passwordBcrypt": []byte("$2a$10$hashed_password"),
				},
			},
			namespace: "",
			wantHash:  "$2a$10$hashed_password",
			wantErr:   false,
		},
		{
			name: "successfully reads password hash from custom namespace",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kotsadm-password",
					Namespace: "custom-ns",
				},
				Data: map[string][]byte{
					"passwordBcrypt": []byte("$2a$10$hashed_password"),
				},
			},
			namespace: "custom-ns",
			wantHash:  "$2a$10$hashed_password",
			wantErr:   false,
		},
		{
			name:        "secret not found",
			secret:      nil,
			namespace:   "",
			wantHash:    "",
			wantErr:     true,
			errContains: "read kotsadm-password secret",
		},
		{
			name: "secret missing passwordBcrypt key",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kotsadm-password",
					Namespace: "kotsadm",
				},
				Data: map[string][]byte{},
			},
			namespace:   "",
			wantHash:    "",
			wantErr:     true,
			errContains: "missing required passwordBcrypt data",
		},
		{
			name: "secret has empty passwordBcrypt value",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kotsadm-password",
					Namespace: "kotsadm",
				},
				Data: map[string][]byte{
					"passwordBcrypt": []byte(""),
				},
			},
			namespace:   "",
			wantHash:    "",
			wantErr:     true,
			errContains: "missing required passwordBcrypt data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Create fake client with or without the secret
			var kcli client.Client
			if tt.secret != nil {
				kcli = fake.NewClientBuilder().WithObjects(tt.secret).Build()
			} else {
				kcli = fake.NewClientBuilder().Build()
			}

			cfg := &Config{
				Client:     kcli,
				InstallDir: "/var/lib/kurl",
			}

			gotHash, err := GetPasswordHash(ctx, cfg, tt.namespace)

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
