package embeddedclusteroperator

import (
	"context"
	"testing"

	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestUpgradeEnsureCAConfigmap(t *testing.T) {
	tests := []struct {
		name           string
		setupObjects   []runtime.Object
		expectCAExists bool
		expectError    bool
	}{
		{
			name:           "no source configmap",
			setupObjects:   []runtime.Object{},
			expectCAExists: false,
			expectError:    false,
		},
		{
			name: "source configmap exists",
			setupObjects: []runtime.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kotsadm-private-cas",
						Namespace: runtimeconfig.KotsadmNamespace,
					},
					Data: map[string]string{
						"ca_0.crt": "test certificate data",
					},
				},
			},
			expectCAExists: true,
			expectError:    false,
		},
		{
			name: "configmap exists in both namespaces",
			setupObjects: []runtime.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kotsadm-private-cas",
						Namespace: runtimeconfig.KotsadmNamespace,
					},
					Data: map[string]string{
						"ca_0.crt": "kotsadm certificate data",
					},
				},
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "private-cas",
						Namespace: namespace,
					},
					Data: map[string]string{
						"ca_0.crt": "test certificate data",
					},
				},
			},
			expectCAExists: true,
			expectError:    false,
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
			err := UpgradeEnsureCAConfigmap(context.Background(), cli)

			// Check error expectations
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			// Check if target configmap was created when expected
			if tt.expectCAExists {
				var targetCM corev1.ConfigMap
				err := cli.Get(context.Background(),
					types.NamespacedName{Namespace: namespace, Name: "private-cas"},
					&targetCM)

				require.NoError(t, err)
				assert.Equal(t, "test certificate data", targetCM.Data["ca_0.crt"])
				assert.Equal(t, namespace, targetCM.Namespace)
				assert.Equal(t, "private-cas", targetCM.Name)
			}
		})
	}
}
