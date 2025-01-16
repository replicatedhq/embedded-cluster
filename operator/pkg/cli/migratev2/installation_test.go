package migratev2

import (
	"context"
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_copyInstallationsToConfigMaps(t *testing.T) {
	tests := []struct {
		name            string
		installs        []ecv1beta1.Installation
		existingConfigs []corev1.ConfigMap
		expectError     bool
		validate        func(*testing.T, client.Client)
	}{
		{
			name: "copies single installation to configmap when none exists",
			installs: []ecv1beta1.Installation{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-install",
					},
					Spec: ecv1beta1.InstallationSpec{
						Config: &ecv1beta1.ConfigSpec{
							Version: "1.0.0",
						},
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, cli client.Client) {
				var cm corev1.ConfigMap
				err := cli.Get(context.Background(), types.NamespacedName{
					Namespace: "embedded-cluster",
					Name:      "test-install",
				}, &cm)
				require.NoError(t, err)

				// Verify configmap has correct labels
				assert.Equal(t, "embedded-cluster", cm.Labels["replicated.com/installation"])
				assert.Equal(t, "ec-install", cm.Labels["replicated.com/disaster-recovery"])

				// Verify installation data is present
				assert.Contains(t, cm.Data["installation"], `"version":"1.0.0"`)
			},
		},
		{
			name: "updates existing configmap with new installation data",
			installs: []ecv1beta1.Installation{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-install",
					},
					Spec: ecv1beta1.InstallationSpec{
						Config: &ecv1beta1.ConfigSpec{
							Version: "2.0.0", // New version
						},
					},
				},
			},
			existingConfigs: []corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-install",
						Namespace: "embedded-cluster",
						Labels: map[string]string{
							"replicated.com/installation":      "embedded-cluster",
							"replicated.com/disaster-recovery": "ec-install",
						},
					},
					Data: map[string]string{
						"installation": `{"spec":{"config":{"version":"1.0.0"}}}`, // Old version
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, cli client.Client) {
				var cm corev1.ConfigMap
				err := cli.Get(context.Background(), types.NamespacedName{
					Namespace: "embedded-cluster",
					Name:      "test-install",
				}, &cm)
				require.NoError(t, err)

				// Verify configmap has correct labels
				assert.Equal(t, "embedded-cluster", cm.Labels["replicated.com/installation"])
				assert.Equal(t, "ec-install", cm.Labels["replicated.com/disaster-recovery"])

				// Verify installation data is updated
				assert.Contains(t, cm.Data["installation"], `"version":"2.0.0"`)
				assert.NotContains(t, cm.Data["installation"], `"version":"1.0.0"`)
			},
		},
		{
			name: "handles mix of new and existing configmaps",
			installs: []ecv1beta1.Installation{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "install-1",
					},
					Spec: ecv1beta1.InstallationSpec{
						Config: &ecv1beta1.ConfigSpec{
							Version: "2.0.0", // Updated version
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "install-2",
					},
					Spec: ecv1beta1.InstallationSpec{
						Config: &ecv1beta1.ConfigSpec{
							Version: "1.0.0", // New installation
						},
					},
				},
			},
			existingConfigs: []corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "install-1",
						Namespace: "embedded-cluster",
						Labels: map[string]string{
							"replicated.com/installation":      "embedded-cluster",
							"replicated.com/disaster-recovery": "ec-install",
						},
					},
					Data: map[string]string{
						"installation": `{"spec":{"config":{"version":"1.0.0"}}}`, // Old version
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, cli client.Client) {
				var cms corev1.ConfigMapList
				err := cli.List(context.Background(), &cms, client.InNamespace("embedded-cluster"))
				require.NoError(t, err)
				assert.Len(t, cms.Items, 2)

				// Verify both configmaps exist with correct data
				for _, cm := range cms.Items {
					assert.Equal(t, "embedded-cluster", cm.Labels["replicated.com/installation"])
					assert.Equal(t, "ec-install", cm.Labels["replicated.com/disaster-recovery"])
					if cm.Name == "install-1" {
						assert.Contains(t, cm.Data["installation"], `"version":"2.0.0"`)
						assert.NotContains(t, cm.Data["installation"], `"version":"1.0.0"`)
					} else {
						assert.Contains(t, cm.Data["installation"], `"version":"1.0.0"`)
					}
				}
			},
		},
		{
			name: "copies multiple installations to configmaps",
			installs: []ecv1beta1.Installation{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "install-1",
					},
					Spec: ecv1beta1.InstallationSpec{
						Config: &ecv1beta1.ConfigSpec{
							Version: "1.0.0",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "install-2",
					},
					Spec: ecv1beta1.InstallationSpec{
						Config: &ecv1beta1.ConfigSpec{
							Version: "2.0.0",
						},
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, cli client.Client) {
				var cms corev1.ConfigMapList
				err := cli.List(context.Background(), &cms, client.InNamespace("embedded-cluster"))
				require.NoError(t, err)
				assert.Len(t, cms.Items, 2)

				// Verify both configmaps exist with correct data
				for _, cm := range cms.Items {
					assert.Equal(t, "embedded-cluster", cm.Labels["replicated.com/installation"])
					assert.Equal(t, "ec-install", cm.Labels["replicated.com/disaster-recovery"])
					if cm.Name == "install-1" {
						assert.Contains(t, cm.Data["installation"], `"version":"1.0.0"`)
					} else {
						assert.Contains(t, cm.Data["installation"], `"version":"2.0.0"`)
					}
				}
			},
		},
		{
			name:        "handles no installations",
			installs:    []ecv1beta1.Installation{},
			expectError: false,
			validate: func(t *testing.T, cli client.Client) {
				var cms corev1.ConfigMapList
				err := cli.List(context.Background(), &cms, client.InNamespace("embedded-cluster"))
				require.NoError(t, err)
				assert.Empty(t, cms.Items)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, ecv1beta1.AddToScheme(scheme))
			require.NoError(t, corev1.AddToScheme(scheme))

			// Create fake client with installations and existing configmaps
			cli := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(append(
					installationsToRuntimeObjects(tt.installs),
					configMapsToRuntimeObjects(tt.existingConfigs)...,
				)...).
				Build()

			logf := func(format string, args ...any) {
				// No-op logger for testing
			}

			// Run the function
			err := copyInstallationsToConfigMaps(context.Background(), logf, cli)

			if tt.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Run validation
			tt.validate(t, cli)
		})
	}
}

func installationsToRuntimeObjects(installs []ecv1beta1.Installation) []client.Object {
	objects := make([]client.Object, len(installs))
	for i := range installs {
		objects[i] = &installs[i]
	}
	return objects
}

func configMapsToRuntimeObjects(configs []corev1.ConfigMap) []client.Object {
	objects := make([]client.Object, len(configs))
	for i := range configs {
		objects[i] = &configs[i]
	}
	return objects
}
