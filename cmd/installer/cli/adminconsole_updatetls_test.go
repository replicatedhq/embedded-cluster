package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_readAndValidateTLSFiles(t *testing.T) {
	tests := []struct {
		name        string
		certContent string
		keyContent  string
		wantErr     string
	}{
		{
			name:        "valid certificate and key",
			certContent: testCertData,
			keyContent:  testKeyData,
		},
		{
			name:        "invalid certificate",
			certContent: "not a valid certificate",
			keyContent:  testKeyData,
			wantErr:     "invalid TLS certificate/key pair",
		},
		{
			name:        "invalid key",
			certContent: testCertData,
			keyContent:  "not a valid key",
			wantErr:     "invalid TLS certificate/key pair",
		},
		{
			name:        "mismatched certificate and key",
			certContent: testCertData,
			keyContent: `-----BEGIN PRIVATE KEY-----
MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQC7o5GAWiZm7sxT
NXrFfPmVLPyGtvXj5G4R9F9RZJ8/YKyHfrHC1YyWPQYCCQnFzaZKONBEWLGHhMmy
u84kLbTy7F6U8lACAoLr4jXvF0sME1KKj3ZU9qxGbZHHKqP6rHcByQn8hT1hdrM0
YT8yHxVFc6L0p0FyZBwfHDBHfHn0X0lEWYJgGfMpPvSYPbQ8f/0mJxPk0d+SZ5zl
vHsYPdCG8LXQB6FvDqRXOvZPkEh+KfLbH9sBr0XZh+LJKq3QnqPJfqBqtTB3xjLS
xGf4Xv8ACXvQ8rPj+Z4VnRJLxLXF5F7JFh8HjJp8y9PXGQ5k3TkRfwcGT+KfJhMm
TnTv8uDhAgMBAAECggEAHDVkczBOKOxgT8ZjqNB2G/3HGVk0PEX9kPrQ8XDCJ3wZ
sBqfHGHdGQy0zhUzXL8G/RDrPQvX8JoRGXL0dNsUgvrfgLJwvQPvGGmhPiKZPYBx
xbIq9Ts9V5CdqwGF7TYE8YoFLhC7e7FzQcQr7Eh7kGVxbYCYqPbAWv9y5FyffXnb
WnpGXvG+Qf8XsDO0pvaDKQltVr7j5PCQE8L7xabwsOT7yrQQX7GJrS8z4W5S3Pmb
3HqKXLKQ8YRZsYNABQ3eaD0xDBaQ/PPZH7Q+bYxsQQ8nC7f5y0S0xRDLfD0S9KOb
lEgBnJPjQ0L0mKWdQMPQ0u5sH5bWPQ0MDZC8Z6hXQQKBgQDlQ0mJhFLF9Nv5C8Qv
9F7x6L9E6AKjDgSPXsH2TGuvshKP7pRHg9E6h5T6tHQxC/JkXfGPXj5Q5k7Q0vfb
QJMdPQ0Q9Hc7dE7XdGk0KvD8a8Q1C0FDo9E6hT6t9E6A8Q1C0FDo9E6hT6t9E6A8
Q1C0FDo9E6hT6t9E6A8Q1C0FDwKBgQDTH9E6A8Q1C0FDo9E6hT6t9E6A8Q1C0FDo
9E6hT6t9E6A8Q1C0FDo9E6hT6t9E6A8Q1C0FDo9E6hT6t9E6A8Q1C0FDo9E6hT6t
9E6A8Q1C0FDo9E6hT6t9E6A8Q1C0FDo9E6hT6t9E6A8Q1C0FDo9E6hT6t9E6A8Q1
-----END PRIVATE KEY-----`,
			wantErr: "invalid TLS certificate/key pair",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			tmpDir := t.TempDir()
			certPath := filepath.Join(tmpDir, "tls.crt")
			keyPath := filepath.Join(tmpDir, "tls.key")

			err := os.WriteFile(certPath, []byte(tt.certContent), 0600)
			req.NoError(err)

			err = os.WriteFile(keyPath, []byte(tt.keyContent), 0600)
			req.NoError(err)

			certBytes, keyBytes, err := readAndValidateTLSFiles(certPath, keyPath)

			if tt.wantErr != "" {
				req.Error(err)
				req.Contains(err.Error(), tt.wantErr)
				req.Nil(certBytes)
				req.Nil(keyBytes)
			} else {
				req.NoError(err)
				req.NotEmpty(certBytes)
				req.NotEmpty(keyBytes)
			}
		})
	}
}

func Test_readAndValidateTLSFiles_FileNotFound(t *testing.T) {
	tests := []struct {
		name       string
		setupFiles func(tmpDir string) (certPath, keyPath string)
		wantErr    string
	}{
		{
			name: "certificate file not found",
			setupFiles: func(tmpDir string) (string, string) {
				keyPath := filepath.Join(tmpDir, "tls.key")
				_ = os.WriteFile(keyPath, []byte(testKeyData), 0600)
				return filepath.Join(tmpDir, "nonexistent.crt"), keyPath
			},
			wantErr: "failed to read TLS certificate file",
		},
		{
			name: "key file not found",
			setupFiles: func(tmpDir string) (string, string) {
				certPath := filepath.Join(tmpDir, "tls.crt")
				_ = os.WriteFile(certPath, []byte(testCertData), 0600)
				return certPath, filepath.Join(tmpDir, "nonexistent.key")
			},
			wantErr: "failed to read TLS key file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			tmpDir := t.TempDir()
			certPath, keyPath := tt.setupFiles(tmpDir)

			certBytes, keyBytes, err := readAndValidateTLSFiles(certPath, keyPath)

			req.Error(err)
			req.Contains(err.Error(), tt.wantErr)
			req.Nil(certBytes)
			req.Nil(keyBytes)
		})
	}
}

func Test_updateTLSSecret(t *testing.T) {
	tests := []struct {
		name         string
		secret       *corev1.Secret
		hostname     string
		wantErr      string
		wantHostname string
	}{
		{
			name: "update existing secret without hostname",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      kotsadmTLSSecretName,
					Namespace: "test-namespace",
				},
				Data: map[string][]byte{
					"tls.crt": []byte("old-cert"),
					"tls.key": []byte("old-key"),
				},
			},
			hostname: "",
		},
		{
			name: "update existing secret with hostname",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      kotsadmTLSSecretName,
					Namespace: "test-namespace",
				},
				Data: map[string][]byte{
					"tls.crt": []byte("old-cert"),
					"tls.key": []byte("old-key"),
				},
			},
			hostname:     "new.example.com",
			wantHostname: "new.example.com",
		},
		{
			name:    "secret not found",
			secret:  nil,
			wantErr: "failed to get kotsadm-tls secret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			scheme := runtime.NewScheme()
			err := corev1.AddToScheme(scheme)
			req.NoError(err)

			var objects []client.Object
			if tt.secret != nil {
				objects = append(objects, tt.secret)
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objects...).
				Build()

			newCert := []byte(testCertData)
			newKey := []byte(testKeyData)

			err = updateTLSSecret(context.Background(), fakeClient, "test-namespace", newCert, newKey, tt.hostname)

			if tt.wantErr != "" {
				req.Error(err)
				req.Contains(err.Error(), tt.wantErr)
			} else {
				req.NoError(err)

				// Verify the secret was updated
				updatedSecret := &corev1.Secret{}
				err = fakeClient.Get(context.Background(), client.ObjectKey{
					Namespace: "test-namespace",
					Name:      kotsadmTLSSecretName,
				}, updatedSecret)
				req.NoError(err)

				assert.Equal(t, newCert, updatedSecret.Data["tls.crt"])
				assert.Equal(t, newKey, updatedSecret.Data["tls.key"])

				if tt.wantHostname != "" {
					assert.Equal(t, tt.wantHostname, updatedSecret.StringData["hostname"])
				}
			}
		})
	}
}

func TestAdminConsoleUpdateTLSCmd(t *testing.T) {
	ctx := context.Background()
	cmd := AdminConsoleUpdateTLSCmd(ctx, "Test App")

	assert.Equal(t, "update-tls", cmd.Use)
	assert.Contains(t, cmd.Short, "Update the TLS certificate")
	assert.Contains(t, cmd.Long, "kotsadm-tls secret")
	assert.Contains(t, cmd.Long, "watch for changes")

	// Check required flags
	tlsCertFlag := cmd.Flags().Lookup("tls-cert")
	assert.NotNil(t, tlsCertFlag)

	tlsKeyFlag := cmd.Flags().Lookup("tls-key")
	assert.NotNil(t, tlsKeyFlag)

	hostnameFlag := cmd.Flags().Lookup("hostname")
	assert.NotNil(t, hostnameFlag)
	assert.Contains(t, hostnameFlag.Usage, "optional")
}
