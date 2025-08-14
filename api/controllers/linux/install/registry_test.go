package install

import (
	"testing"

	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstallController_detectRegistrySettings(t *testing.T) {
	tests := []struct {
		name         string
		airgapBundle string
		serviceCIDR  string
		license      *kotsv1beta1.License
		wantSettings *types.RegistrySettings
	}{
		{
			name:         "online install should return empty settings",
			airgapBundle: "", // Not airgap
			serviceCIDR:  "10.96.0.0/12",
			license:      &kotsv1beta1.License{Spec: kotsv1beta1.LicenseSpec{AppSlug: "my-app"}},
			wantSettings: &types.RegistrySettings{
				HasLocalRegistry:    false,
				Host:               "",
				Namespace:          "",
				Address:            "",
				ImagePullSecretName: "",
			},
		},
		{
			name:         "airgap install without license should return settings without namespace",
			airgapBundle: "/path/to/bundle.airgap",
			serviceCIDR:  "10.96.0.0/12",
			license:      nil,
			wantSettings: &types.RegistrySettings{
				HasLocalRegistry:    true,
				Host:               "10.96.0.11:5000", // GetRegistryClusterIP should return this
				Namespace:          "",
				Address:            "10.96.0.11:5000",
				ImagePullSecretName: "",
			},
		},
		{
			name:         "airgap install with license should return full settings",
			airgapBundle: "/path/to/bundle.airgap",
			serviceCIDR:  "10.96.0.0/12",
			license:      &kotsv1beta1.License{Spec: kotsv1beta1.LicenseSpec{AppSlug: "my-app"}},
			wantSettings: &types.RegistrySettings{
				HasLocalRegistry:    true,
				Host:               "10.96.0.11:5000", // GetRegistryClusterIP should return this
				Namespace:          "my-app",
				Address:            "10.96.0.11:5000/my-app",
				ImagePullSecretName: "my-app-registry",
			},
		},
		{
			name:         "airgap install with empty app slug should return settings without namespace",
			airgapBundle: "/path/to/bundle.airgap",
			serviceCIDR:  "10.96.0.0/12",
			license:      &kotsv1beta1.License{Spec: kotsv1beta1.LicenseSpec{AppSlug: ""}},
			wantSettings: &types.RegistrySettings{
				HasLocalRegistry:    true,
				Host:               "10.96.0.11:5000",
				Namespace:          "",
				Address:            "10.96.0.11:5000",
				ImagePullSecretName: "",
			},
		},
		{
			name:         "airgap install without runtime config should return partial settings",
			airgapBundle: "/path/to/bundle.airgap",
			serviceCIDR:  "", // No runtime config
			license:      &kotsv1beta1.License{Spec: kotsv1beta1.LicenseSpec{AppSlug: "my-app"}},
			wantSettings: &types.RegistrySettings{
				HasLocalRegistry:    true,
				Host:               "", // No host because no runtime config
				Namespace:          "my-app",
				Address:            "/my-app", // Address will be "/namespace" when host is empty
				ImagePullSecretName: "my-app-registry",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller := &InstallController{
				airgapBundle: tt.airgapBundle,
			}

			// Set up runtime config if service CIDR is provided
			if tt.serviceCIDR != "" {
				mockRC := &runtimeconfig.MockRuntimeConfig{}
				networkSpec := ecv1beta1.NetworkSpec{
					ServiceCIDR: tt.serviceCIDR,
				}
				// Mock the SetNetworkSpec call
				mockRC.On("SetNetworkSpec", networkSpec).Return()
				mockRC.SetNetworkSpec(networkSpec)
				
				// Mock the ServiceCIDR() call to return the expected value
				mockRC.On("ServiceCIDR").Return(tt.serviceCIDR)
				controller.rc = mockRC
			}

			result := controller.detectRegistrySettings(tt.license)
			require.NotNil(t, result)
			assert.Equal(t, tt.wantSettings, result)
		})
	}
}