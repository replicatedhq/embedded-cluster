package embeddedclusteroperator

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
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_installEnsureCAConfigmap(t *testing.T) {
	tmpDir := t.TempDir()
	err := os.WriteFile(filepath.Join(tmpDir, "ca_0.crt"), []byte("test certificate 1 data"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "ca_1.crt"), []byte("test certificate 2 data"), 0644)
	require.NoError(t, err)

	tests := []struct {
		name         string
		setupObjects []runtime.Object
		expectError  bool
	}{
		{
			name:         "configmap does not exist",
			setupObjects: []runtime.Object{},
			expectError:  false,
		},
		{
			name: "configmap exists",
			setupObjects: []runtime.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "private-cas",
						Namespace: namespace,
					},
					Data: map[string]string{
						"ca_0.crt": "test certificate 1 data",
						"ca_1.crt": "test certificate 2 data",
					},
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new scheme and register CoreV1 types
			scheme := runtime.NewScheme()
			require.NoError(t, corev1.AddToScheme(scheme))

			// Create a fake client with the test objects
			builder := fake.NewClientBuilder().
				WithScheme(scheme)

			if len(tt.setupObjects) > 0 {
				builder = builder.WithRuntimeObjects(tt.setupObjects...)
			}

			cli := builder.Build()

			// Call the function under test
			privateCAs := []string{
				filepath.Join(tmpDir, "ca_0.crt"),
				filepath.Join(tmpDir, "ca_1.crt"),
			}
			err := installEnsureCAConfigmap(context.Background(), cli, privateCAs)

			// Check error expectations
			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			// Check if target configmap was created when expected
			var targetCM corev1.ConfigMap
			err = cli.Get(context.Background(),
				types.NamespacedName{Namespace: namespace, Name: "private-cas"},
				&targetCM)

			require.NoError(t, err)
			assert.Equal(t, "test certificate 1 data", targetCM.Data["ca_0.crt"])
			assert.Equal(t, "test certificate 2 data", targetCM.Data["ca_1.crt"])
			assert.Equal(t, namespace, targetCM.Namespace)
			assert.Equal(t, "private-cas", targetCM.Name)
		})
	}
}
