package integration

import (
	"context"
	"strings"
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/registry"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"
)

func TestHA_HTTPSecretInjected(t *testing.T) {
	const (
		serviceCIDR   = "10.96.0.0/12"
		httpSecretVal = "test-http-secret-value-abcdefghijklmnop"
		registryNS    = "registry"

		// registry-http-secret is our stable k8s secret (created by ensureHTTPSecret).
		// Its value is read in GenerateHelmValues and passed as secrets.haSharedSecret
		// to the chart, which renders it into the chart-managed registry-secret.
		// The deployment env var REGISTRY_HTTP_SECRET references registry-secret, not
		// registry-http-secret directly.
		httpSecretName  = "registry-http-secret"
		chartSecretName = "registry-secret"
		chartSecretKey  = "haSharedSecret"
	)

	kcli := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      httpSecretName,
				Namespace: registryNS,
			},
			Data: map[string][]byte{
				"secret": []byte(httpSecretVal),
			},
		},
	).Build()

	addon := &registry.Registry{
		IsHA:        true,
		ServiceCIDR: serviceCIDR,
	}

	values, err := addon.GenerateHelmValues(context.Background(), kcli, ecv1beta1.Domains{}, nil)
	require.NoError(t, err, "GenerateHelmValues should not return an error")

	hcli, err := helm.NewClient(helm.HelmOptions{HelmPath: "helm"})
	require.NoError(t, err, "NewClient should not return an error")

	manifests, err := hcli.Render(context.Background(), helm.InstallOptions{
		ReleaseName:  addon.ReleaseName(),
		ChartPath:    addon.ChartLocation(ecv1beta1.Domains{}),
		ChartVersion: registry.Metadata.Version,
		Values:       values,
		Namespace:    addon.Namespace(),
	})
	require.NoError(t, err, "Render should not return an error")
	require.NotEmpty(t, manifests, "Render should return manifests")

	var deployment *appsv1.Deployment
	for _, manifest := range manifests {
		if strings.Contains(string(manifest), "# Source: docker-registry/templates/deployment.yaml") {
			err := yaml.Unmarshal(manifest, &deployment)
			require.NoError(t, err, "Failed to unmarshal deployment")
			break
		}
	}
	require.NotNil(t, deployment, "registry deployment should be present in rendered manifests")

	var httpSecretEnv *corev1.EnvVar
	for _, env := range deployment.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "REGISTRY_HTTP_SECRET" {
			httpSecretEnv = &env
			break
		}
	}

	if assert.NotNil(t, httpSecretEnv, "REGISTRY_HTTP_SECRET env var should be present") {
		if assert.NotNil(t, httpSecretEnv.ValueFrom, "REGISTRY_HTTP_SECRET should use valueFrom") {
			if assert.NotNil(t, httpSecretEnv.ValueFrom.SecretKeyRef, "REGISTRY_HTTP_SECRET should use secretKeyRef") {
				assert.Equal(t, chartSecretName, httpSecretEnv.ValueFrom.SecretKeyRef.Name)
				assert.Equal(t, chartSecretKey, httpSecretEnv.ValueFrom.SecretKeyRef.Key)
			}
		}
	}

	// Verify registry-secret manifest contains the expected value
	var registrySecret *corev1.Secret
	for _, manifest := range manifests {
		if strings.Contains(string(manifest), "# Source: docker-registry/templates/secret.yaml") {
			err := yaml.Unmarshal(manifest, &registrySecret)
			require.NoError(t, err, "Failed to unmarshal registry secret")
			break
		}
	}
	if assert.NotNil(t, registrySecret, "registry-secret should be present in rendered manifests") {
		assert.Equal(t, httpSecretVal, string(registrySecret.Data[chartSecretKey]),
			"%s in %s should match the value from %s", chartSecretKey, chartSecretName, httpSecretName)
	}
}
