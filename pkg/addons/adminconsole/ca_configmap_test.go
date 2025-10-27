package adminconsole

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"path/filepath"
	"testing"

	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metadata "k8s.io/client-go/metadata"
	metadatafake "k8s.io/client-go/metadata/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestEnsureCAConfigmap(t *testing.T) {
	metascheme := metadatafake.NewTestScheme()
	metav1.AddMetaToScheme(metascheme)

	newConfigMap := func(content string) *corev1.ConfigMap {
		hash := md5.Sum([]byte(content))
		checksum := hex.EncodeToString(hash[:])
		return &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      PrivateCASConfigMapName,
				Namespace: _namespace,
				Annotations: map[string]string{
					"replicated.com/cas-checksum": checksum,
				},
			},
			Data: map[string]string{
				"ca_0.crt": content,
			},
		}
	}

	tests := []struct {
		name        string
		initClients func(t *testing.T) (client.Client, metadata.Interface)
		setup       func(t *testing.T) string
		wantErr     bool
		assert      func(t *testing.T, client client.Client)
	}{
		{
			name: "empty CA path should do nothing",
			initClients: func(t *testing.T) (client.Client, metadata.Interface) {
				kcli := clientfake.NewClientBuilder().Build()
				mcli := metadatafake.NewSimpleMetadataClient(metascheme)
				return kcli, mcli
			},
			setup: func(t *testing.T) string {
				// No setup needed for this test
				return ""
			},
			wantErr: false,
			assert: func(t *testing.T, c client.Client) {
				cm := &corev1.ConfigMap{}
				err := c.Get(context.Background(), client.ObjectKey{
					Namespace: _namespace,
					Name:      PrivateCASConfigMapName,
				}, cm)
				assert.True(t, k8serrors.IsNotFound(err), "ConfigMap should not exist")
			},
		},
		{
			name: "should create configmap when it doesn't exist",
			initClients: func(t *testing.T) (client.Client, metadata.Interface) {
				kcli := clientfake.NewClientBuilder().Build()
				mcli := metadatafake.NewSimpleMetadataClient(metascheme)
				return kcli, mcli
			},
			setup: func(t *testing.T) string {
				cafile := filepath.Join(t.TempDir(), "ca.crt")
				err := helpers.WriteFile(cafile, []byte("test-ca-content"), 0644)
				require.NoError(t, err)
				return cafile
			},
			wantErr: false,
			assert: func(t *testing.T, c client.Client) {
				cm := &corev1.ConfigMap{}
				err := c.Get(context.Background(), client.ObjectKey{
					Namespace: _namespace,
					Name:      PrivateCASConfigMapName,
				}, cm)
				require.NoError(t, err)

				assert.Equal(t, "test-ca-content", cm.Data["ca_0.crt"])

				hash := md5.Sum([]byte("test-ca-content"))
				checksum := hex.EncodeToString(hash[:])
				assert.Equal(t, checksum, cm.Annotations["replicated.com/cas-checksum"])
			},
		},
		{
			name: "should update configmap when it exists with different content",
			initClients: func(t *testing.T) (client.Client, metadata.Interface) {
				cm := newConfigMap("old-ca-content")
				kcli := clientfake.NewClientBuilder().WithObjects(cm).Build()
				mcli := metadatafake.NewSimpleMetadataClient(metascheme,
					&metav1.PartialObjectMetadata{TypeMeta: cm.TypeMeta, ObjectMeta: cm.ObjectMeta})
				return kcli, mcli
			},
			setup: func(t *testing.T) string {
				cafile := filepath.Join(t.TempDir(), "ca.crt")
				err := helpers.WriteFile(cafile, []byte("new-ca-content"), 0644)
				require.NoError(t, err)
				return cafile
			},
			wantErr: false,
			assert: func(t *testing.T, c client.Client) {
				cm := &corev1.ConfigMap{}
				err := c.Get(context.Background(), client.ObjectKey{
					Namespace: _namespace,
					Name:      PrivateCASConfigMapName,
				}, cm)
				require.NoError(t, err)

				assert.Equal(t, "new-ca-content", cm.Data["ca_0.crt"])

				hash := md5.Sum([]byte("new-ca-content"))
				checksum := hex.EncodeToString(hash[:])
				assert.Equal(t, checksum, cm.Annotations["replicated.com/cas-checksum"])
			},
		},
		{
			name: "should not update configmap when content is the same",
			initClients: func(t *testing.T) (client.Client, metadata.Interface) {
				cm := newConfigMap("same-ca-content")
				cm.Annotations["some-old-annotation"] = "some-old-value" // this should stay the same
				kcli := clientfake.NewClientBuilder().WithObjects(cm).Build()
				mcli := metadatafake.NewSimpleMetadataClient(metascheme,
					&metav1.PartialObjectMetadata{TypeMeta: cm.TypeMeta, ObjectMeta: cm.ObjectMeta})
				return kcli, mcli
			},
			setup: func(t *testing.T) string {
				cafile := filepath.Join(t.TempDir(), "ca.crt")
				err := helpers.WriteFile(cafile, []byte("same-ca-content"), 0644)
				require.NoError(t, err)
				return cafile
			},
			wantErr: false,
			assert: func(t *testing.T, c client.Client) {
				cm := &corev1.ConfigMap{}
				err := c.Get(context.Background(), client.ObjectKey{
					Namespace: _namespace,
					Name:      PrivateCASConfigMapName,
				}, cm)
				require.NoError(t, err)

				assert.Equal(t, "same-ca-content", cm.Data["ca_0.crt"])

				hash := md5.Sum([]byte("same-ca-content"))
				checksum := hex.EncodeToString(hash[:])
				assert.Equal(t, checksum, cm.Annotations["replicated.com/cas-checksum"])

				assert.Equal(t, "some-old-value", cm.Annotations["some-old-annotation"])
			},
		},
		{
			name: "should return error when CA file doesn't exist",
			initClients: func(t *testing.T) (client.Client, metadata.Interface) {
				kcli := clientfake.NewClientBuilder().Build()
				mcli := metadatafake.NewSimpleMetadataClient(metascheme)
				return kcli, mcli
			},
			setup: func(t *testing.T) string {
				return "/nonexistent/path/ca.crt"
			},
			wantErr: true,
			assert:  func(t *testing.T, c client.Client) {},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var caPath string
			if tt.setup != nil {
				caPath = tt.setup(t)
			}

			kcli, mcli := tt.initClients(t)

			logf := func(format string, args ...any) {} // discard logs
			err := EnsureCAConfigmap(context.Background(), logf, kcli, mcli, caPath)
			if (err != nil) != tt.wantErr {
				t.Fatalf("EnsureCAConfigmap() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.assert != nil {
				tt.assert(t, kcli)
			}
		})
	}
}
