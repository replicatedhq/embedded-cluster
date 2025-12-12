package cli

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_resetTLSSecret(t *testing.T) {
	tests := []struct {
		name       string
		secret     *corev1.Secret
		wantErr    string
		wantCreate bool
	}{
		{
			name: "update existing secret",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kotsadm-tls",
					Namespace: "test-namespace",
				},
				Data: map[string][]byte{
					"tls.crt": []byte("old-cert"),
					"tls.key": []byte("old-key"),
				},
				StringData: map[string]string{
					"hostname": "existing.example.com",
				},
			},
		},
		{
			name:       "secret not found creates new secret",
			secret:     nil,
			wantCreate: true,
		},
		{
			name: "secret with nil Data map",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kotsadm-tls",
					Namespace: "test-namespace",
				},
				Data: nil,
			},
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

			err = resetTLSSecret(context.Background(), fakeClient, "test-namespace", newCert, newKey)

			if tt.wantErr != "" {
				req.Error(err)
				req.Contains(err.Error(), tt.wantErr)
				return
			}

			req.NoError(err)

			// Verify the secret exists and has correct data
			updatedSecret := &corev1.Secret{}
			err = fakeClient.Get(context.Background(), client.ObjectKey{
				Namespace: "test-namespace",
				Name:      "kotsadm-tls",
			}, updatedSecret)
			req.NoError(err)

			assert.Equal(t, newCert, updatedSecret.Data["tls.crt"])
			assert.Equal(t, newKey, updatedSecret.Data["tls.key"])

			// Verify secret type, labels, and annotations for created secrets
			if tt.wantCreate {
				assert.Equal(t, corev1.SecretTypeTLS, updatedSecret.Type)
				assert.Equal(t, "true", updatedSecret.Labels["kots.io/kotsadm"])
				assert.Equal(t, "infra", updatedSecret.Labels["replicated.com/disaster-recovery"])
				assert.Equal(t, "admin-console", updatedSecret.Labels["replicated.com/disaster-recovery-chart"])
				assert.Equal(t, "0", updatedSecret.Annotations["acceptAnonymousUploads"])
			}

		})
	}
}

func TestAdminConsoleResetTLSCmd(t *testing.T) {
	ctx := context.Background()
	cmd := AdminConsoleResetTLSCmd(ctx, "Test App")

	assert.Equal(t, "reset-tls", cmd.Use)
	assert.Contains(t, cmd.Short, "Reset the TLS certificate")
	assert.Contains(t, cmd.Short, "self-signed")
	assert.Contains(t, cmd.Long, "self-signed TLS certificate")

	// Verify no flags are required (unlike update-tls)
	flags := cmd.Flags()
	assert.Nil(t, flags.Lookup("tls-cert"))
	assert.Nil(t, flags.Lookup("tls-key"))
}
