package installation

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
)

// MockNetUtils is a mock implementation of utils.NetUtils
type MockNetUtils struct {
	mock.Mock
}

func (m *MockNetUtils) DetermineBestNetworkInterface() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockNetUtils) ListValidNetworkInterfaces() ([]string, error) {
	args := m.Called()
	return []string{args.String(0)}, args.Error(1)
}

func TestValidateConfig(t *testing.T) {
	// Create test cases for validation
	tests := []struct {
		name        string
		config      *types.InstallationConfig
		expectedErr bool
	}{
		{
			name: "valid config with global CIDR",
			config: &types.InstallationConfig{
				GlobalCIDR:              "10.0.0.0/16",
				NetworkInterface:        "eth0",
				AdminConsolePort:        8800,
				LocalArtifactMirrorPort: 8888,
				DataDirectory:           "/var/lib/embedded-cluster",
			},
			expectedErr: false,
		},
		{
			name: "valid config with pod and service CIDRs",
			config: &types.InstallationConfig{
				PodCIDR:                 "10.0.0.0/17",
				ServiceCIDR:             "10.0.128.0/17",
				NetworkInterface:        "eth0",
				AdminConsolePort:        8800,
				LocalArtifactMirrorPort: 8888,
				DataDirectory:           "/var/lib/embedded-cluster",
			},
			expectedErr: false,
		},
		{
			name: "missing network interface",
			config: &types.InstallationConfig{
				GlobalCIDR:              "10.0.0.0/16",
				AdminConsolePort:        8800,
				LocalArtifactMirrorPort: 8888,
				DataDirectory:           "/var/lib/embedded-cluster",
			},
			expectedErr: true,
		},
		{
			name: "missing global CIDR and pod/service CIDRs",
			config: &types.InstallationConfig{
				NetworkInterface:        "eth0",
				AdminConsolePort:        8800,
				LocalArtifactMirrorPort: 8888,
				DataDirectory:           "/var/lib/embedded-cluster",
			},
			expectedErr: true,
		},
		{
			name: "missing pod CIDR when no global CIDR",
			config: &types.InstallationConfig{
				ServiceCIDR:             "10.0.128.0/17",
				NetworkInterface:        "eth0",
				AdminConsolePort:        8800,
				LocalArtifactMirrorPort: 8888,
				DataDirectory:           "/var/lib/embedded-cluster",
			},
			expectedErr: true,
		},
		{
			name: "missing service CIDR when no global CIDR",
			config: &types.InstallationConfig{
				PodCIDR:                 "10.0.0.0/17",
				NetworkInterface:        "eth0",
				AdminConsolePort:        8800,
				LocalArtifactMirrorPort: 8888,
				DataDirectory:           "/var/lib/embedded-cluster",
			},
			expectedErr: true,
		},
		{
			name: "invalid global CIDR",
			config: &types.InstallationConfig{
				GlobalCIDR:              "10.0.0.0/24", // Not a /16
				NetworkInterface:        "eth0",
				AdminConsolePort:        8800,
				LocalArtifactMirrorPort: 8888,
				DataDirectory:           "/var/lib/embedded-cluster",
			},
			expectedErr: true,
		},
		{
			name: "missing admin console port",
			config: &types.InstallationConfig{
				GlobalCIDR:              "10.0.0.0/16",
				NetworkInterface:        "eth0",
				LocalArtifactMirrorPort: 8888,
				DataDirectory:           "/var/lib/embedded-cluster",
			},
			expectedErr: true,
		},
		{
			name: "missing local artifact mirror port",
			config: &types.InstallationConfig{
				GlobalCIDR:       "10.0.0.0/16",
				NetworkInterface: "eth0",
				AdminConsolePort: 8800,
				DataDirectory:    "/var/lib/embedded-cluster",
			},
			expectedErr: true,
		},
		{
			name: "missing data directory",
			config: &types.InstallationConfig{
				GlobalCIDR:              "10.0.0.0/16",
				NetworkInterface:        "eth0",
				AdminConsolePort:        8800,
				LocalArtifactMirrorPort: 8888,
			},
			expectedErr: true,
		},
		{
			name: "same ports for admin console and artifact mirror",
			config: &types.InstallationConfig{
				GlobalCIDR:              "10.0.0.0/16",
				NetworkInterface:        "eth0",
				AdminConsolePort:        8800,
				LocalArtifactMirrorPort: 8800, // Same as admin console
				DataDirectory:           "/var/lib/embedded-cluster",
			},
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewInstallationManager()
			err := manager.ValidateConfig(tt.config)

			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateStatus(t *testing.T) {
	tests := []struct {
		name        string
		status      *types.InstallationStatus
		expectedErr bool
	}{
		{
			name: "valid status - pending",
			status: &types.InstallationStatus{
				State:       types.InstallationStatePending,
				Description: "Installation pending",
				LastUpdated: time.Now(),
			},
			expectedErr: false,
		},
		{
			name: "valid status - running",
			status: &types.InstallationStatus{
				State:       types.InstallationStateRunning,
				Description: "Installation in progress",
				LastUpdated: time.Now(),
			},
			expectedErr: false,
		},
		{
			name: "valid status - succeeded",
			status: &types.InstallationStatus{
				State:       types.InstallationStateSucceeded,
				Description: "Installation completed successfully",
				LastUpdated: time.Now(),
			},
			expectedErr: false,
		},
		{
			name: "valid status - failed",
			status: &types.InstallationStatus{
				State:       types.InstallationStateFailed,
				Description: "Installation failed",
				LastUpdated: time.Now(),
			},
			expectedErr: false,
		},
		{
			name:        "nil status",
			status:      nil,
			expectedErr: true,
		},
		{
			name: "invalid state",
			status: &types.InstallationStatus{
				State:       "Invalid",
				Description: "Invalid state",
				LastUpdated: time.Now(),
			},
			expectedErr: true,
		},
		{
			name: "missing description",
			status: &types.InstallationStatus{
				State:       types.InstallationStateRunning,
				Description: "",
				LastUpdated: time.Now(),
			},
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewInstallationManager()
			err := manager.ValidateStatus(tt.status)

			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSetDefaults(t *testing.T) {
	// Create a mock for network utilities
	mockNetUtils := &MockNetUtils{}
	mockNetUtils.On("DetermineBestNetworkInterface").Return("eth0", nil)

	tests := []struct {
		name           string
		inputConfig    *types.InstallationConfig
		expectedConfig *types.InstallationConfig
	}{
		{
			name:        "empty config",
			inputConfig: &types.InstallationConfig{},
			expectedConfig: &types.InstallationConfig{
				AdminConsolePort:        ecv1beta1.DefaultAdminConsolePort,
				DataDirectory:           ecv1beta1.DefaultDataDir,
				LocalArtifactMirrorPort: ecv1beta1.DefaultLocalArtifactMirrorPort,
				NetworkInterface:        "eth0",
				GlobalCIDR:              ecv1beta1.DefaultNetworkCIDR,
			},
		},
		{
			name: "partial config",
			inputConfig: &types.InstallationConfig{
				AdminConsolePort: 9000,
				DataDirectory:    "/custom/dir",
			},
			expectedConfig: &types.InstallationConfig{
				AdminConsolePort:        9000,
				DataDirectory:           "/custom/dir",
				LocalArtifactMirrorPort: ecv1beta1.DefaultLocalArtifactMirrorPort,
				NetworkInterface:        "eth0",
				GlobalCIDR:              ecv1beta1.DefaultNetworkCIDR,
			},
		},
		{
			name: "config with pod and service CIDRs",
			inputConfig: &types.InstallationConfig{
				PodCIDR:     "10.1.0.0/17",
				ServiceCIDR: "10.1.128.0/17",
			},
			expectedConfig: &types.InstallationConfig{
				AdminConsolePort:        ecv1beta1.DefaultAdminConsolePort,
				DataDirectory:           ecv1beta1.DefaultDataDir,
				LocalArtifactMirrorPort: ecv1beta1.DefaultLocalArtifactMirrorPort,
				NetworkInterface:        "eth0",
				PodCIDR:                 "10.1.0.0/17",
				ServiceCIDR:             "10.1.128.0/17",
			},
		},
		{
			name: "config with global CIDR",
			inputConfig: &types.InstallationConfig{
				GlobalCIDR: "192.168.0.0/16",
			},
			expectedConfig: &types.InstallationConfig{
				AdminConsolePort:        ecv1beta1.DefaultAdminConsolePort,
				DataDirectory:           ecv1beta1.DefaultDataDir,
				LocalArtifactMirrorPort: ecv1beta1.DefaultLocalArtifactMirrorPort,
				NetworkInterface:        "eth0",
				GlobalCIDR:              "192.168.0.0/16",
			},
		},
		{
			name: "config with proxy settings",
			inputConfig: &types.InstallationConfig{
				HTTPProxy:  "http://proxy.example.com:3128",
				HTTPSProxy: "https://proxy.example.com:3128",
			},
			expectedConfig: &types.InstallationConfig{
				AdminConsolePort:        ecv1beta1.DefaultAdminConsolePort,
				DataDirectory:           ecv1beta1.DefaultDataDir,
				LocalArtifactMirrorPort: ecv1beta1.DefaultLocalArtifactMirrorPort,
				NetworkInterface:        "eth0",
				GlobalCIDR:              ecv1beta1.DefaultNetworkCIDR,
				HTTPProxy:               "http://proxy.example.com:3128",
				HTTPSProxy:              "https://proxy.example.com:3128",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewInstallationManager(WithNetUtils(mockNetUtils))

			err := manager.SetDefaults(tt.inputConfig)
			assert.NoError(t, err)

			assert.NotNil(t, tt.inputConfig)
			assert.Equal(t, tt.expectedConfig, tt.inputConfig)
		})
	}

	// Test when network interface detection fails
	t.Run("network interface detection fails", func(t *testing.T) {
		failingMockNetUtils := &MockNetUtils{}
		failingMockNetUtils.On("DetermineBestNetworkInterface").Return("", errors.New("failed to detect network interface"))

		manager := NewInstallationManager(WithNetUtils(failingMockNetUtils))

		config := &types.InstallationConfig{}
		err := manager.SetDefaults(config)
		assert.NoError(t, err)

		// Network interface should remain empty when detection fails
		assert.Empty(t, config.NetworkInterface)
	})
}

func TestReadWriteOperations(t *testing.T) {
	manager := NewInstallationManager()

	// Test Config operations
	t.Run("config operations", func(t *testing.T) {
		// Test writing a config
		configToWrite := types.InstallationConfig{
			AdminConsolePort:        8800,
			DataDirectory:           "/var/lib/embedded-cluster",
			LocalArtifactMirrorPort: 8888,
			NetworkInterface:        "eth0",
			GlobalCIDR:              "10.0.0.0/16",
		}

		err := manager.WriteConfig(configToWrite)
		assert.NoError(t, err)

		// Test reading it back
		readConfig, err := manager.ReadConfig()
		assert.NoError(t, err)
		assert.NotNil(t, readConfig)

		// Verify the values match
		assert.Equal(t, configToWrite.AdminConsolePort, readConfig.AdminConsolePort)
		assert.Equal(t, configToWrite.DataDirectory, readConfig.DataDirectory)
		assert.Equal(t, configToWrite.LocalArtifactMirrorPort, readConfig.LocalArtifactMirrorPort)
		assert.Equal(t, configToWrite.NetworkInterface, readConfig.NetworkInterface)
		assert.Equal(t, configToWrite.GlobalCIDR, readConfig.GlobalCIDR)
	})

	// Test Status operations
	t.Run("status operations", func(t *testing.T) {
		// Test writing a status
		statusToWrite := types.InstallationStatus{
			State:       types.InstallationStateRunning,
			Description: "Installation in progress",
			LastUpdated: time.Now().UTC().Truncate(time.Second), // Truncate to avoid time precision issues
		}

		err := manager.WriteStatus(statusToWrite)
		assert.NoError(t, err)

		// Test reading it back
		readStatus, err := manager.ReadStatus()
		assert.NoError(t, err)
		assert.NotNil(t, readStatus)

		// Verify the values match
		assert.Equal(t, statusToWrite.State, readStatus.State)
		assert.Equal(t, statusToWrite.Description, readStatus.Description)

		// Compare time with string format to avoid precision issues
		expectedTime := statusToWrite.LastUpdated.Format(time.RFC3339)
		actualTime := readStatus.LastUpdated.Format(time.RFC3339)
		assert.Equal(t, expectedTime, actualTime)
	})
}
