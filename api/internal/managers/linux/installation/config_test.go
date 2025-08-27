package installation

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/replicatedhq/embedded-cluster/api/internal/store/linux/installation"
	"github.com/replicatedhq/embedded-cluster/api/internal/utils"
	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/hostutils"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/registry"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	kyaml "sigs.k8s.io/yaml"
)

func TestValidateConfig(t *testing.T) {
	// Create test cases for validation
	tests := []struct {
		name        string
		rc          runtimeconfig.RuntimeConfig
		config      types.LinuxInstallationConfig
		expectedErr bool
	}{
		{
			name: "valid config with global CIDR",
			config: types.LinuxInstallationConfig{
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
			config: types.LinuxInstallationConfig{
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
			config: types.LinuxInstallationConfig{
				GlobalCIDR:              "10.0.0.0/16",
				AdminConsolePort:        8800,
				LocalArtifactMirrorPort: 8888,
				DataDirectory:           "/var/lib/embedded-cluster",
			},
			expectedErr: true,
		},
		{
			name: "missing global CIDR and pod/service CIDRs",
			config: types.LinuxInstallationConfig{
				NetworkInterface:        "eth0",
				AdminConsolePort:        8800,
				LocalArtifactMirrorPort: 8888,
				DataDirectory:           "/var/lib/embedded-cluster",
			},
			expectedErr: true,
		},
		{
			name: "missing pod CIDR when no global CIDR",
			config: types.LinuxInstallationConfig{
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
			config: types.LinuxInstallationConfig{
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
			config: types.LinuxInstallationConfig{
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
			config: types.LinuxInstallationConfig{
				GlobalCIDR:              "10.0.0.0/16",
				NetworkInterface:        "eth0",
				LocalArtifactMirrorPort: 8888,
				DataDirectory:           "/var/lib/embedded-cluster",
			},
			expectedErr: true,
		},
		{
			name: "missing local artifact mirror port",
			config: types.LinuxInstallationConfig{
				GlobalCIDR:       "10.0.0.0/16",
				NetworkInterface: "eth0",
				AdminConsolePort: 8800,
				DataDirectory:    "/var/lib/embedded-cluster",
			},
			expectedErr: true,
		},
		{
			name: "missing data directory",
			config: types.LinuxInstallationConfig{
				GlobalCIDR:              "10.0.0.0/16",
				NetworkInterface:        "eth0",
				AdminConsolePort:        8800,
				LocalArtifactMirrorPort: 8888,
			},
			expectedErr: true,
		},
		{
			name: "same ports for admin console and artifact mirror",
			config: types.LinuxInstallationConfig{
				GlobalCIDR:              "10.0.0.0/16",
				NetworkInterface:        "eth0",
				AdminConsolePort:        8800,
				LocalArtifactMirrorPort: 8800, // Same as admin console
				DataDirectory:           "/var/lib/embedded-cluster",
			},
			expectedErr: true,
		},
		{
			name: "same ports for admin console and manager",
			rc: func() runtimeconfig.RuntimeConfig {
				rc := runtimeconfig.New(nil)
				rc.SetManagerPort(8800)
				return rc
			}(),
			config: types.LinuxInstallationConfig{
				GlobalCIDR:              "10.0.0.0/16",
				NetworkInterface:        "eth0",
				AdminConsolePort:        8800,
				LocalArtifactMirrorPort: 8888,
				DataDirectory:           "/var/lib/embedded-cluster",
			},
			expectedErr: true,
		},
		{
			name: "same ports for artifact mirror and manager",
			rc: func() runtimeconfig.RuntimeConfig {
				rc := runtimeconfig.New(nil)
				rc.SetManagerPort(8888)
				return rc
			}(),
			config: types.LinuxInstallationConfig{
				GlobalCIDR:              "10.0.0.0/16",
				NetworkInterface:        "eth0",
				AdminConsolePort:        8800,
				LocalArtifactMirrorPort: 8888,
				DataDirectory:           "/var/lib/embedded-cluster",
			},
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rc runtimeconfig.RuntimeConfig
			if tt.rc != nil {
				rc = tt.rc
			} else {
				rc = runtimeconfig.New(nil)
			}
			rc.SetDataDir(t.TempDir())

			manager := NewInstallationManager()

			err := manager.ValidateConfig(tt.config, rc.ManagerPort())

			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSetConfigDefaults(t *testing.T) {
	// Create a mock for network utilities
	mockNetUtils := &utils.MockNetUtils{}
	mockNetUtils.On("DetermineBestNetworkInterface").Return("eth0", nil)

	// Create a mock RuntimeConfig
	mockRC := &runtimeconfig.MockRuntimeConfig{}
	testDataDir := "/test/data/dir"
	mockRC.On("EmbeddedClusterHomeDirectory").Return(testDataDir)

	tests := []struct {
		name           string
		inputConfig    types.LinuxInstallationConfig
		expectedConfig types.LinuxInstallationConfig
	}{
		{
			name:        "empty config",
			inputConfig: types.LinuxInstallationConfig{},
			expectedConfig: types.LinuxInstallationConfig{
				AdminConsolePort:        ecv1beta1.DefaultAdminConsolePort,
				DataDirectory:           testDataDir,
				LocalArtifactMirrorPort: ecv1beta1.DefaultLocalArtifactMirrorPort,
				NetworkInterface:        "eth0",
				GlobalCIDR:              ecv1beta1.DefaultNetworkCIDR,
			},
		},
		{
			name: "partial config",
			inputConfig: types.LinuxInstallationConfig{
				AdminConsolePort: 9000,
				DataDirectory:    "/custom/dir",
			},
			expectedConfig: types.LinuxInstallationConfig{
				AdminConsolePort:        9000,
				DataDirectory:           "/custom/dir",
				LocalArtifactMirrorPort: ecv1beta1.DefaultLocalArtifactMirrorPort,
				NetworkInterface:        "eth0",
				GlobalCIDR:              ecv1beta1.DefaultNetworkCIDR,
			},
		},
		{
			name: "config with pod and service CIDRs",
			inputConfig: types.LinuxInstallationConfig{
				PodCIDR:     "10.1.0.0/17",
				ServiceCIDR: "10.1.128.0/17",
			},
			expectedConfig: types.LinuxInstallationConfig{
				AdminConsolePort:        ecv1beta1.DefaultAdminConsolePort,
				DataDirectory:           testDataDir,
				LocalArtifactMirrorPort: ecv1beta1.DefaultLocalArtifactMirrorPort,
				NetworkInterface:        "eth0",
				PodCIDR:                 "10.1.0.0/17",
				ServiceCIDR:             "10.1.128.0/17",
			},
		},
		{
			name: "config with global CIDR",
			inputConfig: types.LinuxInstallationConfig{
				GlobalCIDR: "192.168.0.0/16",
			},
			expectedConfig: types.LinuxInstallationConfig{
				AdminConsolePort:        ecv1beta1.DefaultAdminConsolePort,
				DataDirectory:           testDataDir,
				LocalArtifactMirrorPort: ecv1beta1.DefaultLocalArtifactMirrorPort,
				NetworkInterface:        "eth0",
				GlobalCIDR:              "192.168.0.0/16",
			},
		},
		{
			name: "config with proxy settings",
			inputConfig: types.LinuxInstallationConfig{
				HTTPProxy:  "http://proxy.example.com:3128",
				HTTPSProxy: "https://proxy.example.com:3128",
			},
			expectedConfig: types.LinuxInstallationConfig{
				AdminConsolePort:        ecv1beta1.DefaultAdminConsolePort,
				DataDirectory:           testDataDir,
				LocalArtifactMirrorPort: ecv1beta1.DefaultLocalArtifactMirrorPort,
				NetworkInterface:        "eth0",
				GlobalCIDR:              ecv1beta1.DefaultNetworkCIDR,
				HTTPProxy:               "http://proxy.example.com:3128",
				HTTPSProxy:              "https://proxy.example.com:3128",
			},
		},
		{
			name: "config with existing data directory should preserve it",
			inputConfig: types.LinuxInstallationConfig{
				DataDirectory: "/existing/custom/path",
			},
			expectedConfig: types.LinuxInstallationConfig{
				AdminConsolePort:        ecv1beta1.DefaultAdminConsolePort,
				DataDirectory:           "/existing/custom/path",
				LocalArtifactMirrorPort: ecv1beta1.DefaultLocalArtifactMirrorPort,
				NetworkInterface:        "eth0",
				GlobalCIDR:              ecv1beta1.DefaultNetworkCIDR,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewInstallationManager(WithNetUtils(mockNetUtils))

			err := manager.SetConfigDefaults(&tt.inputConfig, mockRC)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedConfig, tt.inputConfig)
		})
	}

	// Test when network interface detection fails
	t.Run("network interface detection fails", func(t *testing.T) {
		failingMockNetUtils := &utils.MockNetUtils{}
		failingMockNetUtils.On("DetermineBestNetworkInterface").Return("", errors.New("failed to detect network interface"))

		manager := NewInstallationManager(WithNetUtils(failingMockNetUtils))

		config := types.LinuxInstallationConfig{}
		err := manager.SetConfigDefaults(&config, mockRC)
		assert.NoError(t, err)

		// Network interface should remain empty when detection fails
		assert.Empty(t, config.NetworkInterface)
		// DataDirectory should still be set from RuntimeConfig
		assert.Equal(t, testDataDir, config.DataDirectory)
	})
}

func TestConfigSetAndGet(t *testing.T) {
	manager := NewInstallationManager()

	// Test writing a config
	configToWrite := types.LinuxInstallationConfig{
		AdminConsolePort:        8800,
		DataDirectory:           "/var/lib/embedded-cluster",
		LocalArtifactMirrorPort: 8888,
		NetworkInterface:        "eth0",
		GlobalCIDR:              "10.0.0.0/16",
	}

	err := manager.SetConfig(configToWrite)
	assert.NoError(t, err)

	// Test reading it back
	readConfig, err := manager.GetConfig()
	assert.NoError(t, err)

	// Verify the values match
	assert.Equal(t, configToWrite.AdminConsolePort, readConfig.AdminConsolePort)
	assert.Equal(t, configToWrite.DataDirectory, readConfig.DataDirectory)
	assert.Equal(t, configToWrite.LocalArtifactMirrorPort, readConfig.LocalArtifactMirrorPort)
	assert.Equal(t, configToWrite.NetworkInterface, readConfig.NetworkInterface)
	assert.Equal(t, configToWrite.GlobalCIDR, readConfig.GlobalCIDR)
}

func TestConfigureHost(t *testing.T) {
	tests := []struct {
		name        string
		rc          runtimeconfig.RuntimeConfig
		setupMocks  func(*hostutils.MockHostUtils, *installation.MockStore)
		expectedErr bool
	}{
		{
			name: "successful configuration",
			rc: func() runtimeconfig.RuntimeConfig {
				rc := runtimeconfig.New(&ecv1beta1.RuntimeConfigSpec{
					DataDir: "/var/lib/embedded-cluster",
					Network: ecv1beta1.NetworkSpec{
						PodCIDR:     "10.0.0.0/16",
						ServiceCIDR: "10.1.0.0/16",
					},
				})
				return rc
			}(),
			setupMocks: func(hum *hostutils.MockHostUtils, im *installation.MockStore) {
				mock.InOrder(
					im.On("SetStatus", mock.MatchedBy(func(status types.Status) bool { return status.State == types.StateRunning })).Return(nil),
					hum.On("ConfigureHost", mock.Anything,
						mock.MatchedBy(func(rc runtimeconfig.RuntimeConfig) bool {
							return rc.EmbeddedClusterHomeDirectory() == "/var/lib/embedded-cluster" &&
								rc.PodCIDR() == "10.0.0.0/16" &&
								rc.ServiceCIDR() == "10.1.0.0/16"
						}),
						hostutils.InitForInstallOptions{
							License:      []byte("metadata:\n  name: test-license"),
							AirgapBundle: "bundle.tar",
						}).Return(nil),
					im.On("SetStatus", mock.MatchedBy(func(status types.Status) bool { return status.State == types.StateSucceeded })).Return(nil),
				)
			},
			expectedErr: false,
		},
		{
			name: "configure installation fails",
			rc: func() runtimeconfig.RuntimeConfig {
				rc := runtimeconfig.New(&ecv1beta1.RuntimeConfigSpec{
					DataDir: "/var/lib/embedded-cluster",
				})
				return rc
			}(),
			setupMocks: func(hum *hostutils.MockHostUtils, im *installation.MockStore) {
				mock.InOrder(
					im.On("SetStatus", mock.MatchedBy(func(status types.Status) bool { return status.State == types.StateRunning })).Return(nil),
					hum.On("ConfigureHost", mock.Anything,
						mock.MatchedBy(func(rc runtimeconfig.RuntimeConfig) bool {
							return rc.EmbeddedClusterHomeDirectory() == "/var/lib/embedded-cluster"
						}),
						hostutils.InitForInstallOptions{
							License:      []byte("metadata:\n  name: test-license"),
							AirgapBundle: "bundle.tar",
						},
					).Return(errors.New("configuration failed")),
					im.On("SetStatus", mock.MatchedBy(func(status types.Status) bool { return status.State == types.StateFailed })).Return(nil),
				)
			},
			expectedErr: true,
		},
		{
			name: "set running status fails",
			rc: func() runtimeconfig.RuntimeConfig {
				rc := runtimeconfig.New(&ecv1beta1.RuntimeConfigSpec{
					DataDir: "/var/lib/embedded-cluster",
				})
				return rc
			}(),
			setupMocks: func(hum *hostutils.MockHostUtils, im *installation.MockStore) {
				mock.InOrder(
					im.On("SetStatus", mock.Anything).Return(errors.New("failed to set status")),
				)
			},
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := tt.rc

			// Create mocks
			mockHostUtils := &hostutils.MockHostUtils{}
			mockStore := &installation.MockStore{}

			// Setup mocks
			tt.setupMocks(mockHostUtils, mockStore)

			// Create manager with mocks
			manager := NewInstallationManager(
				WithHostUtils(mockHostUtils),
				WithInstallationStore(mockStore),
				WithLicense([]byte("metadata:\n  name: test-license")),
				WithAirgapBundle("bundle.tar"),
			)

			// Run the test
			err := manager.ConfigureHost(context.Background(), rc)

			// Assertions
			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Verify all mock expectations were met
			mockStore.AssertExpectations(t)
			mockHostUtils.AssertExpectations(t)
		})
	}
}

func TestCalculateRegistrySettings(t *testing.T) {
	// Helper to create a test license
	createTestLicense := func(licenseID, appSlug string) []byte {
		license := kotsv1beta1.License{
			Spec: kotsv1beta1.LicenseSpec{
				LicenseID: licenseID,
				AppSlug:   appSlug,
			},
		}
		licenseBytes, _ := kyaml.Marshal(license)
		return licenseBytes
	}

	// Helper to create test release data
	createTestReleaseData := func(appSlug string, domains *ecv1beta1.Domains) *release.ReleaseData {
		releaseData := &release.ReleaseData{
			ChannelRelease: &release.ChannelRelease{
				AppSlug: appSlug,
			},
		}
		if domains != nil {
			releaseData.EmbeddedClusterConfig = &ecv1beta1.Config{
				Spec: ecv1beta1.ConfigSpec{
					Domains: *domains,
				},
			}
		}
		return releaseData
	}

	// Helper to create runtime config
	createTestRuntimeConfig := func() runtimeconfig.RuntimeConfig {
		return runtimeconfig.New(&ecv1beta1.RuntimeConfigSpec{
			Network: ecv1beta1.NetworkSpec{
				ServiceCIDR: "10.96.0.0/12",
			},
		})
	}

	tests := []struct {
		name           string
		license        []byte
		releaseData    *release.ReleaseData
		airgapBundle   string
		expectedResult *types.RegistrySettings
		expectedError  string
	}{
		{
			name:         "online mode with default domains",
			license:      createTestLicense("test-license-123", "test-app"),
			releaseData:  createTestReleaseData("test-app", nil),
			airgapBundle: "", // Online mode
			expectedResult: &types.RegistrySettings{
				HasLocalRegistry:     false,
				ImagePullSecretName:  "test-app-registry",
				ImagePullSecretValue: base64.StdEncoding.EncodeToString([]byte(`{"auths":{"proxy.replicated.com":{"username": "LICENSE_ID", "password": "test-license-123"},"registry.replicated.com":{"username": "LICENSE_ID", "password": "test-license-123"}}}`)),
			},
		},
		{
			name:    "online mode with custom domains",
			license: createTestLicense("custom-license-456", "custom-app"),
			releaseData: createTestReleaseData("custom-app", &ecv1beta1.Domains{
				ProxyRegistryDomain:      "custom-proxy.example.com",
				ReplicatedRegistryDomain: "custom-registry.example.com",
			}),
			airgapBundle: "", // Online mode
			expectedResult: &types.RegistrySettings{
				HasLocalRegistry:     false,
				ImagePullSecretName:  "custom-app-registry",
				ImagePullSecretValue: base64.StdEncoding.EncodeToString([]byte(`{"auths":{"custom-proxy.example.com":{"username": "LICENSE_ID", "password": "custom-license-456"},"custom-registry.example.com":{"username": "LICENSE_ID", "password": "custom-license-456"}}}`)),
			},
		},
		{
			name:          "online mode missing license",
			license:       nil,
			releaseData:   createTestReleaseData("test-app", nil),
			airgapBundle:  "", // Online mode
			expectedError: "license is required for online registry settings",
		},
		{
			name:          "online mode empty license",
			license:       []byte{},
			releaseData:   createTestReleaseData("test-app", nil),
			airgapBundle:  "", // Online mode
			expectedError: "license is required for online registry settings",
		},
		{
			name:          "online mode invalid license format",
			license:       []byte("invalid yaml"),
			releaseData:   createTestReleaseData("test-app", nil),
			airgapBundle:  "", // Online mode
			expectedError: "parse license:",
		},
		{
			name:          "online mode missing release data",
			license:       createTestLicense("test-license", "test-app"),
			releaseData:   nil,
			airgapBundle:  "", // Online mode
			expectedError: "release data with app slug is required for registry settings",
		},
		{
			name:    "online mode missing app slug",
			license: createTestLicense("test-license", "test-app"),
			releaseData: &release.ReleaseData{
				ChannelRelease: &release.ChannelRelease{
					AppSlug: "", // Empty app slug
				},
			},
			airgapBundle:  "", // Online mode
			expectedError: "release data with app slug is required for registry settings",
		},
		{
			name:         "airgap mode",
			license:      createTestLicense("test-license", "test-app"),
			releaseData:  createTestReleaseData("test-app", nil),
			airgapBundle: "test-bundle.tar",
			expectedResult: &types.RegistrySettings{
				HasLocalRegistry:       true,
				LocalRegistryHost:      "10.96.0.11:5000",
				LocalRegistryAddress:   "10.96.0.11:5000/test-app",
				LocalRegistryNamespace: "test-app",
				ImagePullSecretName:    "test-app-registry",
				ImagePullSecretValue:   base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(`{"auths":{"10.96.0.11:5000":{"username": "embedded-cluster", "password": "%s"}}}`, registry.GetRegistryPassword()))),
			},
		},
		{
			name:          "airgap mode missing release data",
			license:       createTestLicense("test-license", "test-app"),
			releaseData:   nil,
			airgapBundle:  "test-bundle.tar",
			expectedError: "release data with app slug is required for registry settings",
		},
		{
			name:    "airgap mode missing app slug",
			license: createTestLicense("test-license", "test-app"),
			releaseData: &release.ReleaseData{
				ChannelRelease: &release.ChannelRelease{
					AppSlug: "", // Empty app slug
				},
			},
			airgapBundle:  "test-bundle.tar",
			expectedError: "release data with app slug is required for registry settings",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := createTestRuntimeConfig()

			manager := NewInstallationManager(
				WithLicense(tt.license),
				WithReleaseData(tt.releaseData),
				WithAirgapBundle(tt.airgapBundle),
			)

			result, err := manager.CalculateRegistrySettings(context.Background(), rc)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedResult, result)
			}
		})
	}
}
