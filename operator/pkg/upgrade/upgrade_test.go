package upgrade

import (
	"context"
	"testing"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestUpdateClusterConfig(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, k0sv1beta1.AddToScheme(scheme))

	tests := []struct {
		name          string
		currentConfig *k0sv1beta1.ClusterConfig
		installation  *ecv1beta1.Installation
		validate      func(t *testing.T, updatedConfig *k0sv1beta1.ClusterConfig)
	}{
		{
			name: "updates images with proxy registry domain",
			currentConfig: &k0sv1beta1.ClusterConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "k0s",
					Namespace: "kube-system",
				},
			},
			installation: &ecv1beta1.Installation{
				Spec: ecv1beta1.InstallationSpec{
					Config: &ecv1beta1.ConfigSpec{
						Domains: ecv1beta1.Domains{
							ProxyRegistryDomain: "registry.com",
						},
					},
				},
			},
			validate: func(t *testing.T, updatedConfig *k0sv1beta1.ClusterConfig) {
				assert.Contains(t, updatedConfig.Spec.Images.CoreDNS.Image, "registry.com/")
				assert.Contains(t, updatedConfig.Spec.Images.Calico.Node.Image, "registry.com/")
				assert.Contains(t, updatedConfig.Spec.Images.Calico.CNI.Image, "registry.com/")
				assert.Contains(t, updatedConfig.Spec.Images.Calico.KubeControllers.Image, "registry.com/")
				assert.Contains(t, updatedConfig.Spec.Images.MetricsServer.Image, "registry.com/")
				assert.Contains(t, updatedConfig.Spec.Images.KubeProxy.Image, "registry.com/")
				assert.Contains(t, updatedConfig.Spec.Images.Pause.Image, "registry.com/")
				assert.Contains(t, updatedConfig.Spec.Network.NodeLocalLoadBalancing.EnvoyProxy.Image.Image, "registry.com/")
			},
		},
		{
			name: "updates node local load balancing when different",
			currentConfig: &k0sv1beta1.ClusterConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "k0s",
					Namespace: "kube-system",
				},
				Spec: &k0sv1beta1.ClusterSpec{
					Network: &k0sv1beta1.Network{
						NodeLocalLoadBalancing: &k0sv1beta1.NodeLocalLoadBalancing{
							Enabled: true,
							Type:    k0sv1beta1.NllbTypeEnvoyProxy,
							EnvoyProxy: &k0sv1beta1.EnvoyProxy{
								Image: &k0sv1beta1.ImageSpec{
									Image:   "some-image",
									Version: "some-version",
								},
							},
						},
					},
				},
			},
			installation: &ecv1beta1.Installation{
				Spec: ecv1beta1.InstallationSpec{
					Config: &ecv1beta1.ConfigSpec{
						Domains: ecv1beta1.Domains{
							ProxyRegistryDomain: "registry.com",
						},
					},
				},
			},
			validate: func(t *testing.T, updatedConfig *k0sv1beta1.ClusterConfig) {
				assert.True(t, updatedConfig.Spec.Network.NodeLocalLoadBalancing.Enabled)
				assert.Equal(t, k0sv1beta1.NllbTypeEnvoyProxy, updatedConfig.Spec.Network.NodeLocalLoadBalancing.Type)
				assert.Contains(t, updatedConfig.Spec.Network.NodeLocalLoadBalancing.EnvoyProxy.Image.Image, "registry.com/")
			},
		},
		{
			name: "does not enable node local load balancing when nil",
			currentConfig: &k0sv1beta1.ClusterConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "k0s",
					Namespace: "kube-system",
				},
				Spec: &k0sv1beta1.ClusterSpec{
					Network: &k0sv1beta1.Network{
						NodeLocalLoadBalancing: nil,
					},
				},
			},
			installation: &ecv1beta1.Installation{
				Spec: ecv1beta1.InstallationSpec{
					Config: &ecv1beta1.ConfigSpec{
						Domains: ecv1beta1.Domains{
							ProxyRegistryDomain: "registry.com",
						},
					},
				},
			},
			validate: func(t *testing.T, updatedConfig *k0sv1beta1.ClusterConfig) {
				assert.False(t, updatedConfig.Spec.Network.NodeLocalLoadBalancing.Enabled)
				assert.Equal(t, k0sv1beta1.NllbTypeEnvoyProxy, updatedConfig.Spec.Network.NodeLocalLoadBalancing.Type)
				assert.Contains(t, updatedConfig.Spec.Network.NodeLocalLoadBalancing.EnvoyProxy.Image.Image, "registry.com/")
			},
		},
		{
			name: "applies unsupported vendor k0s overrides",
			currentConfig: &k0sv1beta1.ClusterConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "k0s",
					Namespace: "kube-system",
				},
				Spec: &k0sv1beta1.ClusterSpec{
					Network: &k0sv1beta1.Network{
						ServiceCIDR: "10.96.0.0/12",
					},
					Telemetry: &k0sv1beta1.ClusterTelemetry{
						Enabled: true,
					},
				},
			},
			installation: &ecv1beta1.Installation{
				Spec: ecv1beta1.InstallationSpec{
					Config: &ecv1beta1.ConfigSpec{
						Domains: ecv1beta1.Domains{
							ProxyRegistryDomain: "registry.com",
						},
						UnsupportedOverrides: ecv1beta1.UnsupportedOverrides{
							K0s: `
config:
  spec:
    telemetry:
      enabled: false
    workerProfiles:
    - name: ip-forward
      values:
        allowedUnsafeSysctls:
        - net.ipv4.ip_forward
`,
						},
					},
				},
			},
			validate: func(t *testing.T, updatedConfig *k0sv1beta1.ClusterConfig) {
				// Verify that the unsupported override was applied to the telemetry config
				assert.Equal(t, false, updatedConfig.Spec.Telemetry.Enabled)

				// Verify that the unsupported override was applied to the worker profiles
				require.Len(t, updatedConfig.Spec.WorkerProfiles, 1)
				assert.Equal(t, "ip-forward", updatedConfig.Spec.WorkerProfiles[0].Name)
				assert.Equal(t, &runtime.RawExtension{Raw: []byte(`{"allowedUnsafeSysctls":["net.ipv4.ip_forward"]}`)}, updatedConfig.Spec.WorkerProfiles[0].Config)

				// Verify that other changes were not made
				assert.Equal(t, "10.96.0.0/12", updatedConfig.Spec.Network.ServiceCIDR)
				// Verify that supported changes (like image registries) are still applied
				assert.Contains(t, updatedConfig.Spec.Images.CoreDNS.Image, "registry.com/")
			},
		},
		{
			name: "applies unsupported end user k0s overrides",
			currentConfig: &k0sv1beta1.ClusterConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "k0s",
					Namespace: "kube-system",
				},
				Spec: &k0sv1beta1.ClusterSpec{
					Network: &k0sv1beta1.Network{
						ServiceCIDR: "10.96.0.0/12",
					},
				},
			},
			installation: &ecv1beta1.Installation{
				Spec: ecv1beta1.InstallationSpec{
					Config: &ecv1beta1.ConfigSpec{
						Domains: ecv1beta1.Domains{
							ProxyRegistryDomain: "registry.com",
						},
					},
					EndUserK0sConfigOverrides: `
config:
  spec:
    workerProfiles:
    - name: another-profile
      values:
        allowedUnsafeSysctls:
        - net.ipv4.ip_forward
`,
				},
			},
			validate: func(t *testing.T, updatedConfig *k0sv1beta1.ClusterConfig) {
				// Verify that the unsupported override was applied to the worker profiles
				require.Len(t, updatedConfig.Spec.WorkerProfiles, 1)
				assert.Equal(t, "another-profile", updatedConfig.Spec.WorkerProfiles[0].Name)
				assert.Equal(t, &runtime.RawExtension{Raw: []byte(`{"allowedUnsafeSysctls":["net.ipv4.ip_forward"]}`)}, updatedConfig.Spec.WorkerProfiles[0].Config)

				// Verify that other changes were not made
				assert.Equal(t, "10.96.0.0/12", updatedConfig.Spec.Network.ServiceCIDR)
				// Verify that supported changes (like image registries) are still applied
				assert.Contains(t, updatedConfig.Spec.Images.CoreDNS.Image, "registry.com/")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.currentConfig).
				Build()

			err := updateClusterConfig(context.Background(), cli, tt.installation)
			require.NoError(t, err)

			var updatedConfig k0sv1beta1.ClusterConfig
			err = cli.Get(context.Background(), client.ObjectKey{Name: "k0s", Namespace: "kube-system"}, &updatedConfig)
			require.NoError(t, err)

			tt.validate(t, &updatedConfig)
		})
	}
}
