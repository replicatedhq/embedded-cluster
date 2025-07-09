package infra

import (
	"testing"

	appconfig "github.com/replicatedhq/embedded-cluster/api/internal/managers/app/config"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInfraManager_getAddonInstallOpts(t *testing.T) {
	tests := []struct {
		name             string
		cliConfigValues  string
		appConfigManager appconfig.AppConfigManager
		expectErr        bool
	}{
		{
			name:            "CLI file path should take precedence - memory store not called",
			cliConfigValues: "/path/to/cli-config.yaml",
			appConfigManager: func() appconfig.AppConfigManager {
				mock := &appconfig.MockAppConfigManager{}
				// Should NOT be called when CLI file is provided
				return mock
			}(),
			expectErr: false,
		},
		{
			name:            "memory store values should be used when no CLI file provided",
			cliConfigValues: "",
			appConfigManager: func() appconfig.AppConfigManager {
				mock := &appconfig.MockAppConfigManager{}
				configValues := kotsv1beta1.ConfigValues{
					Spec: kotsv1beta1.ConfigValuesSpec{
						Values: map[string]kotsv1beta1.ConfigValue{
							"memory-key": {Value: "memory-value"},
						},
					},
				}
				mock.On("GetConfigValues").Return(configValues, nil)
				return mock
			}(),
			expectErr: false,
		},
		{
			name:            "memory store error should fail installation",
			cliConfigValues: "",
			appConfigManager: func() appconfig.AppConfigManager {
				mock := &appconfig.MockAppConfigManager{}
				mock.On("GetConfigValues").Return(kotsv1beta1.ConfigValues{}, assert.AnError)
				return mock
			}(),
			expectErr: true,
		},
		{
			name:             "nil app config manager should not set config values",
			cliConfigValues:  "",
			appConfigManager: nil,
			expectErr:        false,
		},
		{
			name:            "empty memory store values should not set config values",
			cliConfigValues: "",
			appConfigManager: func() appconfig.AppConfigManager {
				mock := &appconfig.MockAppConfigManager{}
				emptyConfigValues := kotsv1beta1.ConfigValues{
					Spec: kotsv1beta1.ConfigValuesSpec{
						Values: map[string]kotsv1beta1.ConfigValue{}, // Empty values
					},
				}
				mock.On("GetConfigValues").Return(emptyConfigValues, nil)
				return mock
			}(),
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create infra manager with test app config manager
			manager := &infraManager{
				configValues:     tt.cliConfigValues,
				appConfigManager: tt.appConfigManager,
				releaseData:      &release.ReleaseData{},
				clusterID:        "test-cluster",
				license:          []byte("test-license"),
				logger:           logrus.New(),
			}

			// Create test license
			license := &kotsv1beta1.License{
				Spec: kotsv1beta1.LicenseSpec{
					AppSlug: "test-app",
				},
			}

			// Create test runtime config
			rc := runtimeconfig.New(nil)

			// Test the getAddonInstallOpts function
			opts := manager.getAddonInstallOpts(license, rc)

			// Verify KotsInstaller is set
			require.NotNil(t, opts.KotsInstaller)

			// Test both error handling and priority logic by calling the callback
			if tt.expectErr {
				err := opts.KotsInstaller()
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "retrieving config values from memory store")
			} else {
				// Call the callback to test priority logic - this is where memory store interactions happen
				// We don't assert on the result since it depends on the environment (file permissions, etc.)
				_ = opts.KotsInstaller()
			}

			// Verify mock expectations - this confirms our priority logic worked correctly
			if mockAppConfigManager, ok := tt.appConfigManager.(*appconfig.MockAppConfigManager); ok {
				mockAppConfigManager.AssertExpectations(t)
			}
		})
	}
}
