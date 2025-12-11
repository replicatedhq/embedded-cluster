package install

import (
	"encoding/base64"
	"os"
	"testing"

	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	metadatafake "k8s.io/client-go/metadata/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_namespaceReconciler_reconcile(t *testing.T) {
	dockerConfigJSON := `{"auths":{"registry.example.com":{"auth":"dXNlcjpwYXNz"}}}`

	appSlug := "test-app"
	versionLabel := "1.0.0"

	tests := []struct {
		name               string
		applicationYAML    string
		registrySettings   *types.RegistrySettings
		withCABundle       bool
		existingNamespaces []string
		existingSecrets    []corev1.Secret
		existingConfigMaps []corev1.ConfigMap

		wantNamespaces        []string
		wantCreatedNs         []string
		wantSecretInNs        []string
		wantNoSecretInNs      []string
		wantCAConfigmapInNs   []string
		wantNoCAConfigmapInNs []string
		wantErr               bool
	}{
		{
			name:            "no application - only app namespace",
			applicationYAML: "",
			registrySettings: &types.RegistrySettings{
				ImagePullSecretName:  "test-secret",
				ImagePullSecretValue: base64.StdEncoding.EncodeToString([]byte(dockerConfigJSON)),
			},
			existingNamespaces:    []string{appSlug},
			wantNamespaces:        []string{appSlug},
			wantCreatedNs:         []string{},
			wantSecretInNs:        []string{appSlug},
			wantNoCAConfigmapInNs: []string{appSlug},
		},
		{
			name: "application with no additional namespaces",
			applicationYAML: `apiVersion: kots.io/v1beta1
kind: Application
metadata:
  name: test-app
spec:
  title: Test App`,
			registrySettings: &types.RegistrySettings{
				ImagePullSecretName:  "test-secret",
				ImagePullSecretValue: base64.StdEncoding.EncodeToString([]byte(dockerConfigJSON)),
			},
			existingNamespaces:    []string{appSlug},
			wantNamespaces:        []string{appSlug},
			wantCreatedNs:         []string{},
			wantSecretInNs:        []string{appSlug},
			wantNoCAConfigmapInNs: []string{appSlug},
		},
		{
			name: "application with additional namespaces",
			applicationYAML: `apiVersion: kots.io/v1beta1
kind: Application
metadata:
  name: test-app
spec:
  title: Test App
  additionalNamespaces:
    - app-ns-1
    - app-ns-2`,
			registrySettings: &types.RegistrySettings{
				ImagePullSecretName:  "test-secret",
				ImagePullSecretValue: base64.StdEncoding.EncodeToString([]byte(dockerConfigJSON)),
			},
			existingNamespaces:    []string{appSlug},
			wantNamespaces:        []string{appSlug, "app-ns-1", "app-ns-2"},
			wantCreatedNs:         []string{"app-ns-1", "app-ns-2"},
			wantSecretInNs:        []string{appSlug, "app-ns-1", "app-ns-2"},
			wantNoCAConfigmapInNs: []string{appSlug, "app-ns-1", "app-ns-2"},
		},
		{
			name: "application with wildcard namespace - now skipped with warning",
			applicationYAML: `apiVersion: kots.io/v1beta1
kind: Application
metadata:
  name: test-app
spec:
  title: Test App
  additionalNamespaces:
    - "*"`,
			registrySettings: &types.RegistrySettings{
				ImagePullSecretName:  "test-secret",
				ImagePullSecretValue: base64.StdEncoding.EncodeToString([]byte(dockerConfigJSON)),
			},
			existingNamespaces:    []string{appSlug, "existing-ns-1", "existing-ns-2"},
			wantNamespaces:        []string{appSlug}, // "*" is now skipped
			wantCreatedNs:         []string{},
			wantSecretInNs:        []string{appSlug}, // Only appSlug gets the secret
			wantNoSecretInNs:      []string{"existing-ns-1", "existing-ns-2"},
			wantNoCAConfigmapInNs: []string{appSlug, "existing-ns-1", "existing-ns-2"},
		},
		{
			name: "no registry settings - no secrets created",
			applicationYAML: `apiVersion: kots.io/v1beta1
kind: Application
metadata:
  name: test-app
spec:
  title: Test App
  additionalNamespaces:
    - app-ns`,
			registrySettings:      nil,
			existingNamespaces:    []string{appSlug},
			wantNamespaces:        []string{appSlug, "app-ns"},
			wantCreatedNs:         []string{"app-ns"},
			wantSecretInNs:        []string{},
			wantNoSecretInNs:      []string{appSlug, "app-ns"},
			wantNoCAConfigmapInNs: []string{appSlug, "app-ns"},
		},
		{
			name: "with CA bundle path - configmaps created",
			applicationYAML: `apiVersion: kots.io/v1beta1
kind: Application
metadata:
  name: test-app
spec:
  title: Test App
  additionalNamespaces:
    - app-ns`,
			registrySettings: &types.RegistrySettings{
				ImagePullSecretName:  "test-secret",
				ImagePullSecretValue: base64.StdEncoding.EncodeToString([]byte(dockerConfigJSON)),
			},
			withCABundle:        true,
			existingNamespaces:  []string{appSlug},
			wantNamespaces:      []string{appSlug, "app-ns"},
			wantCreatedNs:       []string{"app-ns"},
			wantSecretInNs:      []string{appSlug, "app-ns"},
			wantCAConfigmapInNs: []string{appSlug, "app-ns"},
		},
		{
			name: "updates existing secret with different data",
			applicationYAML: `apiVersion: kots.io/v1beta1
kind: Application
metadata:
  name: test-app
spec:
  title: Test App`,
			registrySettings: &types.RegistrySettings{
				ImagePullSecretName:  "test-secret",
				ImagePullSecretValue: base64.StdEncoding.EncodeToString([]byte(dockerConfigJSON)),
			},
			existingNamespaces: []string{appSlug},
			existingSecrets: []corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-secret",
						Namespace: appSlug,
					},
					Type: corev1.SecretTypeDockerConfigJson,
					Data: map[string][]byte{
						".dockerconfigjson": []byte(`{"auths":{"old.registry.com":{}}}`),
					},
				},
			},
			wantNamespaces: []string{appSlug},
			wantCreatedNs:  []string{},
			wantSecretInNs: []string{appSlug},
		},
		{
			name: "updates existing CA configmap with different data",
			applicationYAML: `apiVersion: kots.io/v1beta1
kind: Application
metadata:
  name: test-app
spec:
  title: Test App`,
			registrySettings: &types.RegistrySettings{
				ImagePullSecretName:  "test-secret",
				ImagePullSecretValue: base64.StdEncoding.EncodeToString([]byte(dockerConfigJSON)),
			},
			withCABundle:       true,
			existingNamespaces: []string{appSlug},
			existingConfigMaps: []corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kotsadm-private-cas",
						Namespace: appSlug,
						Annotations: map[string]string{
							"replicated.com/cas-checksum": "old-checksum",
						},
					},
					Data: map[string]string{
						"ca_0.crt": "old-ca-content",
					},
				},
			},
			wantNamespaces:      []string{appSlug},
			wantCreatedNs:       []string{},
			wantSecretInNs:      []string{appSlug},
			wantCAConfigmapInNs: []string{appSlug},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ENABLE_V3", "1")

			// Set up release data
			releaseData := map[string][]byte{
				"channelrelease.yaml": []byte("# channel release object\nappSlug: test-app"),
			}
			if tt.applicationYAML != "" {
				releaseData["application.yaml"] = []byte(tt.applicationYAML)
			}
			err := release.SetReleaseDataForTests(releaseData)
			require.NoError(t, err)

			// Build fake client with existing namespaces, secrets, and configmaps
			builder := fake.NewClientBuilder().WithScheme(scheme.Scheme)
			for _, nsName := range tt.existingNamespaces {
				ns := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{Name: nsName},
				}
				builder = builder.WithObjects(ns)
			}
			for i := range tt.existingSecrets {
				builder = builder.WithObjects(&tt.existingSecrets[i])
			}
			for i := range tt.existingConfigMaps {
				builder = builder.WithObjects(&tt.existingConfigMaps[i])
			}
			fakeKcli := builder.Build()

			// Create fake metadata client
			fakeMcli := metadatafake.NewSimpleMetadataClient(metadatafake.NewTestScheme())

			// Handle temp CA file
			var hostCABundlePath string
			if tt.withCABundle {
				tmpFile, err := os.CreateTemp("", "ca-bundle-*.crt")
				require.NoError(t, err)
				defer os.Remove(tmpFile.Name())
				_, err = tmpFile.WriteString("-----BEGIN CERTIFICATE-----\ntest-ca-content\n-----END CERTIFICATE-----")
				require.NoError(t, err)
				tmpFile.Close()
				hostCABundlePath = tmpFile.Name()
			}

			// Create the reconciler
			reconciler, err := newNamespaceReconciler(
				t.Context(),
				fakeKcli,
				fakeMcli,
				tt.registrySettings,
				hostCABundlePath,
				appSlug,
				versionLabel,
				logger.NewDiscardLogger(),
			)
			require.NoError(t, err)
			require.NotNil(t, reconciler)

			// Verify namespaces to be reconciled
			assert.Equal(t, tt.wantNamespaces, reconciler.namespaces)

			// Run the reconciler
			err = reconciler.reconcile(t.Context())
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Verify namespaces were created
			for _, nsName := range tt.wantCreatedNs {
				ns := &corev1.Namespace{}
				err := fakeKcli.Get(t.Context(), client.ObjectKey{Name: nsName}, ns)
				require.NoError(t, err, "namespace %s should be created", nsName)
			}

			// Verify secrets were created in expected namespaces
			for _, nsName := range tt.wantSecretInNs {
				secret := &corev1.Secret{}
				err := fakeKcli.Get(t.Context(), client.ObjectKey{
					Namespace: nsName,
					Name:      tt.registrySettings.ImagePullSecretName,
				}, secret)
				require.NoError(t, err, "secret should exist in namespace %s", nsName)
				assert.Equal(t, corev1.SecretTypeDockerConfigJson, secret.Type)
				assert.Equal(t, dockerConfigJSON, string(secret.Data[".dockerconfigjson"]))
			}

			// Verify CA configmaps were created in expected namespaces
			for _, nsName := range tt.wantCAConfigmapInNs {
				configMap := &corev1.ConfigMap{}
				err := fakeKcli.Get(t.Context(), client.ObjectKey{
					Namespace: nsName,
					Name:      "kotsadm-private-cas",
				}, configMap)
				require.NoError(t, err, "CA configmap should exist in namespace %s", nsName)
				assert.Contains(t, configMap.Data["ca_0.crt"], "test-ca-content")
			}

			// Verify secrets were NOT created in namespaces where they shouldn't be
			for _, nsName := range tt.wantNoSecretInNs {
				secret := &corev1.Secret{}
				err := fakeKcli.Get(t.Context(), client.ObjectKey{
					Namespace: nsName,
					Name:      "test-secret",
				}, secret)
				assert.Error(t, err, "secret should not exist in namespace %s", nsName)
			}

			// Verify CA configmaps were NOT created in namespaces where they shouldn't be
			for _, nsName := range tt.wantNoCAConfigmapInNs {
				configMap := &corev1.ConfigMap{}
				err := fakeKcli.Get(t.Context(), client.ObjectKey{
					Namespace: nsName,
					Name:      "kotsadm-private-cas",
				}, configMap)
				assert.Error(t, err, "CA configmap should not exist in namespace %s", nsName)
			}
		})
	}
}
