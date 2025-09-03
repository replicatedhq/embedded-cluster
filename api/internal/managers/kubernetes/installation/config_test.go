package installation

import (
	"testing"

	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/stretchr/testify/assert"
)

func TestValidateConfig(t *testing.T) {
	// Create test cases for validation
	tests := []struct {
		name        string
		managerPort int
		config      types.KubernetesInstallationConfig
		expectedErr bool
	}{
		{
			name:        "valid config with admin console port",
			managerPort: 8080,
			config: types.KubernetesInstallationConfig{
				AdminConsolePort: 8800,
			},
			expectedErr: false,
		},
		{
			name:        "missing admin console port",
			managerPort: 8080,
			config:      types.KubernetesInstallationConfig{},
			expectedErr: true,
		},
		{
			name:        "admin console port is zero",
			managerPort: 8080,
			config: types.KubernetesInstallationConfig{
				AdminConsolePort: 0,
			},
			expectedErr: true,
		},
		{
			name:        "same ports for admin console and manager",
			managerPort: 8800,
			config: types.KubernetesInstallationConfig{
				AdminConsolePort: 8800,
			},
			expectedErr: true,
		},
		{
			name:        "valid config with proxy settings",
			managerPort: 8080,
			config: types.KubernetesInstallationConfig{
				AdminConsolePort: 8800,
				HTTPProxy:        "http://proxy.example.com:3128",
				HTTPSProxy:       "https://proxy.example.com:3128",
				NoProxy:          "localhost,127.0.0.1",
			},
			expectedErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewInstallationManager()

			err := manager.ValidateConfig(tt.config, tt.managerPort)

			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSetConfigDefaults(t *testing.T) {
	tests := []struct {
		name           string
		inputConfig    types.KubernetesInstallationConfig
		expectedConfig types.KubernetesInstallationConfig
	}{
		{
			name:        "empty config",
			inputConfig: types.KubernetesInstallationConfig{},
			expectedConfig: types.KubernetesInstallationConfig{
				AdminConsolePort: ecv1beta1.DefaultAdminConsolePort,
			},
		},
		{
			name: "partial config with admin console port",
			inputConfig: types.KubernetesInstallationConfig{
				AdminConsolePort: 9000,
			},
			expectedConfig: types.KubernetesInstallationConfig{
				AdminConsolePort: 9000,
			},
		},
		{
			name: "config with proxy settings",
			inputConfig: types.KubernetesInstallationConfig{
				HTTPProxy:  "http://proxy.example.com:3128",
				HTTPSProxy: "https://proxy.example.com:3128",
			},
			expectedConfig: types.KubernetesInstallationConfig{
				AdminConsolePort: ecv1beta1.DefaultAdminConsolePort,
				HTTPProxy:        "http://proxy.example.com:3128",
				HTTPSProxy:       "https://proxy.example.com:3128",
			},
		},
		{
			name: "config with all proxy settings",
			inputConfig: types.KubernetesInstallationConfig{
				HTTPProxy:  "http://proxy.example.com:3128",
				HTTPSProxy: "https://proxy.example.com:3128",
				NoProxy:    "localhost,127.0.0.1",
			},
			expectedConfig: types.KubernetesInstallationConfig{
				AdminConsolePort: ecv1beta1.DefaultAdminConsolePort,
				HTTPProxy:        "http://proxy.example.com:3128",
				HTTPSProxy:       "https://proxy.example.com:3128",
				NoProxy:          "localhost,127.0.0.1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewInstallationManager()

			err := manager.setConfigDefaults(&tt.inputConfig)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedConfig, tt.inputConfig)
		})
	}
}

func TestSetConfigDefaultsNoEnvProxy(t *testing.T) {
	// Set proxy environment variables to simulate a proxy-enabled environment
	t.Setenv("HTTP_PROXY", "http://env-proxy.example.com:8080")
	t.Setenv("HTTPS_PROXY", "https://env-proxy.example.com:8080")
	t.Setenv("NO_PROXY", "localhost,127.0.0.1,.env-example.com")

	manager := NewInstallationManager()

	// Test with empty config - should only set admin console port default
	config := types.KubernetesInstallationConfig{}
	err := manager.setConfigDefaults(&config)
	assert.NoError(t, err)

	// Verify that only the admin console port is set as default
	expectedConfig := types.KubernetesInstallationConfig{
		AdminConsolePort: ecv1beta1.DefaultAdminConsolePort,
	}
	assert.Equal(t, expectedConfig, config)

	// Verify that proxy fields remain empty even though environment variables are set
	assert.Empty(t, config.HTTPProxy, "HTTPProxy should not be set from environment variable")
	assert.Empty(t, config.HTTPSProxy, "HTTPSProxy should not be set from environment variable")
	assert.Empty(t, config.NoProxy, "NoProxy should not be set from environment variable")
}

func TestGetDefaults(t *testing.T) {
	tests := []struct {
		name             string
		expectedDefaults types.KubernetesInstallationConfig
		expectedErr      bool
	}{
		{
			name: "successful defaults",
			expectedDefaults: types.KubernetesInstallationConfig{
				AdminConsolePort: ecv1beta1.DefaultAdminConsolePort,
			},
			expectedErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create manager
			manager := NewInstallationManager()

			// Call GetDefaults
			defaults, err := manager.GetDefaults()

			// Assertions
			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedDefaults, defaults)
			}
		})
	}
}

func TestConfigSetAndGet(t *testing.T) {
	manager := NewInstallationManager()

	// Test writing config values
	configToWrite := types.KubernetesInstallationConfig{
		AdminConsolePort: 8800,
		HTTPProxy:        "http://proxy.example.com:3128",
		HTTPSProxy:       "https://proxy.example.com:3128",
		NoProxy:          "localhost,127.0.0.1",
	}

	err := manager.SetConfigValues(configToWrite)
	assert.NoError(t, err)

	// Test reading user values back
	readValues, err := manager.GetConfigValues()
	assert.NoError(t, err)

	// Verify the user values match
	assert.Equal(t, configToWrite.AdminConsolePort, readValues.AdminConsolePort)
	assert.Equal(t, configToWrite.HTTPProxy, readValues.HTTPProxy)
	assert.Equal(t, configToWrite.HTTPSProxy, readValues.HTTPSProxy)
	assert.Equal(t, configToWrite.NoProxy, readValues.NoProxy)

	// Test reading resolved config (should have defaults applied)
	resolvedConfig, err := manager.GetConfig()
	assert.NoError(t, err)

	// Verify the resolved config has user values
	assert.Equal(t, configToWrite.AdminConsolePort, resolvedConfig.AdminConsolePort)
	assert.Equal(t, configToWrite.HTTPProxy, resolvedConfig.HTTPProxy)
	assert.Equal(t, configToWrite.HTTPSProxy, resolvedConfig.HTTPSProxy)
	assert.Equal(t, configToWrite.NoProxy, resolvedConfig.NoProxy)
}
