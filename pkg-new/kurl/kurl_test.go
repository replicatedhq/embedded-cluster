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

func TestDiscoverKotsadmNamespace(t *testing.T) {
	tests := []struct {
		name        string
		objects     []client.Object
		wantNs      string
		wantErr     bool
		errContains string
	}{
		{
			name: "finds kotsadm service in default namespace",
			objects: []client.Object{
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kotsadm",
						Namespace: "default",
						Labels:    map[string]string{"app": "kotsadm"},
					},
				},
			},
			wantNs:  "default",
			wantErr: false,
		},
		{
			name: "finds kotsadm service in custom namespace when not in default",
			objects: []client.Object{
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kotsadm",
						Namespace: "kots-namespace",
						Labels:    map[string]string{"app": "kotsadm"},
					},
				},
			},
			wantNs:  "kots-namespace",
			wantErr: false,
		},
		{
			name: "prioritizes default namespace over other namespaces",
			objects: []client.Object{
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kotsadm",
						Namespace: "default",
						Labels:    map[string]string{"app": "kotsadm"},
					},
				},
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kotsadm",
						Namespace: "other-namespace",
						Labels:    map[string]string{"app": "kotsadm"},
					},
				},
			},
			wantNs:  "default",
			wantErr: false,
		},
		{
			name:        "kotsadm service not found",
			objects:     []client.Object{},
			wantNs:      "",
			wantErr:     true,
			errContains: "kotsadm service not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			kcli := fake.NewClientBuilder().WithObjects(tt.objects...).Build()

			cfg := &Config{
				Client:     kcli,
				InstallDir: "/var/lib/kurl",
			}

			gotNs, err := DiscoverKotsadmNamespace(ctx, cfg)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantNs, gotNs)
			}
		})
	}
}

func TestGetPasswordHash(t *testing.T) {
	tests := []struct {
		name        string
		objects     []client.Object
		namespace   string
		wantHash    string
		wantErr     bool
		errContains string
	}{
		{
			name: "successfully reads password hash with auto-discovery",
			objects: []client.Object{
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kotsadm",
						Namespace: "default",
						Labels:    map[string]string{"app": "kotsadm"},
					},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kotsadm-password",
						Namespace: "default",
					},
					Data: map[string][]byte{
						"passwordBcrypt": []byte("$2a$10$hashed_password"),
					},
				},
			},
			namespace: "",
			wantHash:  "$2a$10$hashed_password",
			wantErr:   false,
		},
		{
			name: "successfully reads password hash from explicit namespace",
			objects: []client.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kotsadm-password",
						Namespace: "custom-ns",
					},
					Data: map[string][]byte{
						"passwordBcrypt": []byte("$2a$10$hashed_password"),
					},
				},
			},
			namespace: "custom-ns",
			wantHash:  "$2a$10$hashed_password",
			wantErr:   false,
		},
		{
			name:        "secret not found with auto-discovery",
			objects:     []client.Object{},
			namespace:   "",
			wantHash:    "",
			wantErr:     true,
			errContains: "kotsadm service not found",
		},
		{
			name: "secret missing passwordBcrypt key",
			objects: []client.Object{
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kotsadm",
						Namespace: "default",
						Labels:    map[string]string{"app": "kotsadm"},
					},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kotsadm-password",
						Namespace: "default",
					},
					Data: map[string][]byte{},
				},
			},
			namespace:   "",
			wantHash:    "",
			wantErr:     true,
			errContains: "missing required passwordBcrypt data",
		},
		{
			name: "secret has empty passwordBcrypt value",
			objects: []client.Object{
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kotsadm",
						Namespace: "default",
						Labels:    map[string]string{"app": "kotsadm"},
					},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kotsadm-password",
						Namespace: "default",
					},
					Data: map[string][]byte{
						"passwordBcrypt": []byte(""),
					},
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
			kcli := fake.NewClientBuilder().WithObjects(tt.objects...).Build()

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
