package infra

import (
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/kotskinds/pkg/licensewrapper"
	"github.com/stretchr/testify/assert"
)

func TestInfraManager_getAddonInstallOpts(t *testing.T) {
	tests := []struct {
		name              string
		configValues      kotsv1beta1.ConfigValues
		verifyInstallOpts func(t *testing.T, opts addons.InstallOptions)
	}{
		{
			name: "Config values should be passed correctly",
			configValues: kotsv1beta1.ConfigValues{
				Spec: kotsv1beta1.ConfigValuesSpec{
					Values: map[string]kotsv1beta1.ConfigValue{
						"key1": {
							Value: "value1",
						},
						"key2": {
							Value: "value2",
						},
					},
				},
			},
			verifyInstallOpts: func(t *testing.T, opts addons.InstallOptions) {
				assert.Equal(t, "test-cluster", opts.ClusterID)
				assert.NotNil(t, opts.License)
			},
		},
		{
			name:         "basic options should be set correctly",
			configValues: kotsv1beta1.ConfigValues{},
			verifyInstallOpts: func(t *testing.T, opts addons.InstallOptions) {
				assert.Equal(t, "test-cluster", opts.ClusterID)
				assert.NotNil(t, opts.License)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory for test
			tempDir := t.TempDir()

			// Create runtime config
			rcSpec := &ecv1beta1.RuntimeConfigSpec{
				DataDir: tempDir,
			}
			rc := runtimeconfig.New(rcSpec)

			// Create test license
			license := &kotsv1beta1.License{
				Spec: kotsv1beta1.LicenseSpec{
					AppSlug: "test-app",
				},
			}

			// Wrap the license
			wrappedLicense := &licensewrapper.LicenseWrapper{
				V1: license,
			}

			// Create infra manager
			manager := NewInfraManager(
				WithClusterID("test-cluster"),
				WithLicense([]byte("spec:\n  licenseID: test-license\n")),
			)

			// Test the getAddonInstallOpts method with configValues passed as parameter
			opts, err := manager.getAddonInstallOpts(t.Context(), wrappedLicense, rc)
			assert.NoError(t, err)

			// Verify the install options
			tt.verifyInstallOpts(t, opts)
		})
	}
}
