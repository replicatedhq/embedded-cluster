package installation

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"testing"

	"github.com/replicatedhq/embedded-cluster/api/internal/utils"
	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/hostutils"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/registry"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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
				LocalArtifactMirrorPort: 8888,
				DataDirectory:           "/var/lib/embedded-cluster",
			},
			expectedErr: false,
		},
		{
			name: "missing network interface",
			config: types.LinuxInstallationConfig{
				GlobalCIDR:              "10.0.0.0/16",
				LocalArtifactMirrorPort: 8888,
				DataDirectory:           "/var/lib/embedded-cluster",
			},
			expectedErr: true,
		},
		{
			name: "missing global CIDR and pod/service CIDRs",
			config: types.LinuxInstallationConfig{
				NetworkInterface:        "eth0",
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
				DataDirectory:    "/var/lib/embedded-cluster",
			},
			expectedErr: true,
		},
		{
			name: "missing data directory",
			config: types.LinuxInstallationConfig{
				GlobalCIDR:              "10.0.0.0/16",
				NetworkInterface:        "eth0",
				LocalArtifactMirrorPort: 8888,
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
				DataDirectory:           testDataDir,
				LocalArtifactMirrorPort: ecv1beta1.DefaultLocalArtifactMirrorPort,
				NetworkInterface:        "eth0",
				GlobalCIDR:              ecv1beta1.DefaultNetworkCIDR,
			},
		},
		{
			name: "partial config",
			inputConfig: types.LinuxInstallationConfig{
				DataDirectory: "/custom/dir",
			},
			expectedConfig: types.LinuxInstallationConfig{
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

			err := manager.setConfigDefaults(&tt.inputConfig, mockRC)
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
		err := manager.setConfigDefaults(&config, mockRC)
		assert.NoError(t, err)

		// Network interface should remain empty when detection fails
		assert.Empty(t, config.NetworkInterface)
		// DataDirectory should still be set from RuntimeConfig
		assert.Equal(t, testDataDir, config.DataDirectory)
	})
}

func TestGetDefaults(t *testing.T) {
	tests := []struct {
		name             string
		setupMocks       func(*utils.MockNetUtils)
		setupEnv         func(t *testing.T)
		expectedDefaults types.LinuxInstallationConfig
		expectedErr      bool
	}{
		{
			name: "successful defaults with network interface detection and no proxy env vars",
			setupMocks: func(mockNetUtils *utils.MockNetUtils) {
				mockNetUtils.On("DetermineBestNetworkInterface").Return("eth0", nil)
			},
			setupEnv: func(t *testing.T) {
				// Ensure proxy environment variables are not set
				t.Setenv("HTTP_PROXY", "")
				t.Setenv("http_proxy", "")
				t.Setenv("HTTPS_PROXY", "")
				t.Setenv("https_proxy", "")
				t.Setenv("NO_PROXY", "")
				t.Setenv("no_proxy", "")
			},
			expectedDefaults: types.LinuxInstallationConfig{
				DataDirectory:           "/test/data/dir",
				LocalArtifactMirrorPort: ecv1beta1.DefaultLocalArtifactMirrorPort,
				GlobalCIDR:              ecv1beta1.DefaultNetworkCIDR,
				NetworkInterface:        "eth0",
				HTTPProxy:               "",
				HTTPSProxy:              "",
				NoProxy:                 "",
			},
			expectedErr: false,
		},
		{
			name: "successful defaults with proxy environment variables set",
			setupMocks: func(mockNetUtils *utils.MockNetUtils) {
				mockNetUtils.On("DetermineBestNetworkInterface").Return("eth0", nil)
			},
			setupEnv: func(t *testing.T) {
				// Set proxy environment variables
				t.Setenv("HTTP_PROXY", "http://proxy.example.com:3128")
				t.Setenv("HTTPS_PROXY", "https://proxy.example.com:3128")
				t.Setenv("NO_PROXY", "localhost,127.0.0.1")
			},
			expectedDefaults: types.LinuxInstallationConfig{
				DataDirectory:           "/test/data/dir",
				LocalArtifactMirrorPort: ecv1beta1.DefaultLocalArtifactMirrorPort,
				GlobalCIDR:              ecv1beta1.DefaultNetworkCIDR,
				NetworkInterface:        "eth0",
				HTTPProxy:               "http://proxy.example.com:3128",
				HTTPSProxy:              "https://proxy.example.com:3128",
				NoProxy:                 "localhost,127.0.0.1",
			},
			expectedErr: false,
		},
		{
			name: "successful defaults with lowercase proxy environment variables",
			setupMocks: func(mockNetUtils *utils.MockNetUtils) {
				mockNetUtils.On("DetermineBestNetworkInterface").Return("eth0", nil)
			},
			setupEnv: func(t *testing.T) {
				// Set lowercase proxy environment variables (higher precedence)
				t.Setenv("http_proxy", "http://lower-proxy.example.com:8080")
				t.Setenv("https_proxy", "https://lower-proxy.example.com:8080")
				t.Setenv("no_proxy", "localhost,127.0.0.1,.example.com")
				// Also set uppercase ones to verify lowercase takes precedence
				t.Setenv("HTTP_PROXY", "http://upper-proxy.example.com:3128")
				t.Setenv("HTTPS_PROXY", "https://upper-proxy.example.com:3128")
				t.Setenv("NO_PROXY", "localhost,127.0.0.1")
			},
			expectedDefaults: types.LinuxInstallationConfig{
				DataDirectory:           "/test/data/dir",
				LocalArtifactMirrorPort: ecv1beta1.DefaultLocalArtifactMirrorPort,
				GlobalCIDR:              ecv1beta1.DefaultNetworkCIDR,
				NetworkInterface:        "eth0",
				HTTPProxy:               "http://lower-proxy.example.com:8080",
				HTTPSProxy:              "https://lower-proxy.example.com:8080",
				NoProxy:                 "localhost,127.0.0.1,.example.com",
			},
			expectedErr: false,
		},
		{
			name: "network interface detection fails with proxy env vars",
			setupMocks: func(mockNetUtils *utils.MockNetUtils) {
				mockNetUtils.On("DetermineBestNetworkInterface").Return("", errors.New("network detection failed"))
			},
			setupEnv: func(t *testing.T) {
				t.Setenv("HTTP_PROXY", "http://proxy.example.com:3128")
				t.Setenv("HTTPS_PROXY", "https://proxy.example.com:3128")
				t.Setenv("NO_PROXY", "localhost,127.0.0.1")
			},
			expectedDefaults: types.LinuxInstallationConfig{
				DataDirectory:           "/test/data/dir",
				LocalArtifactMirrorPort: ecv1beta1.DefaultLocalArtifactMirrorPort,
				GlobalCIDR:              ecv1beta1.DefaultNetworkCIDR,
				NetworkInterface:        "", // Should be empty when detection fails
				HTTPProxy:               "http://proxy.example.com:3128",
				HTTPSProxy:              "https://proxy.example.com:3128",
				NoProxy:                 "localhost,127.0.0.1",
			},
			expectedErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment variables
			if tt.setupEnv != nil {
				tt.setupEnv(t)
			}

			// Create a mock RuntimeConfig
			mockRC := &runtimeconfig.MockRuntimeConfig{}
			testDataDir := "/test/data/dir"
			mockRC.On("EmbeddedClusterHomeDirectory").Return(testDataDir)

			// Create mock NetUtils
			mockNetUtils := &utils.MockNetUtils{}
			tt.setupMocks(mockNetUtils)

			// Create manager with mocks
			manager := NewInstallationManager(WithNetUtils(mockNetUtils))

			// Call GetDefaults
			defaults, err := manager.GetDefaults(mockRC)

			// Assertions
			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedDefaults, defaults)
			}

			// Verify mock expectations
			mockRC.AssertExpectations(t)
			mockNetUtils.AssertExpectations(t)
		})
	}
}

func TestConfigSetAndGet(t *testing.T) {
	manager := NewInstallationManager()

	// Test writing config values
	configToWrite := types.LinuxInstallationConfig{
		DataDirectory:           "/var/lib/embedded-cluster",
		LocalArtifactMirrorPort: 8888,
		NetworkInterface:        "eth0",
		GlobalCIDR:              "10.0.0.0/16",
	}

	err := manager.SetConfigValues(configToWrite)
	assert.NoError(t, err)

	// Test reading user values back
	readValues, err := manager.GetConfigValues()
	assert.NoError(t, err)

	// Verify the user values match
	assert.Equal(t, configToWrite.DataDirectory, readValues.DataDirectory)
	assert.Equal(t, configToWrite.LocalArtifactMirrorPort, readValues.LocalArtifactMirrorPort)
	assert.Equal(t, configToWrite.NetworkInterface, readValues.NetworkInterface)
	assert.Equal(t, configToWrite.GlobalCIDR, readValues.GlobalCIDR)

	// Test reading resolved config (should have defaults applied)
	// Create a mock RuntimeConfig for the GetConfig method
	mockRC := &runtimeconfig.MockRuntimeConfig{}
	mockRC.On("EmbeddedClusterHomeDirectory").Return("/test/data/dir")

	resolvedConfig, err := manager.GetConfig(mockRC)
	assert.NoError(t, err)

	// Verify the resolved config has user values
	assert.Equal(t, configToWrite.DataDirectory, resolvedConfig.DataDirectory)
	assert.Equal(t, configToWrite.LocalArtifactMirrorPort, resolvedConfig.LocalArtifactMirrorPort)
	assert.Equal(t, configToWrite.NetworkInterface, resolvedConfig.NetworkInterface)
	assert.Equal(t, configToWrite.GlobalCIDR, resolvedConfig.GlobalCIDR)

	// Verify mock expectations
	mockRC.AssertExpectations(t)
}

// TestComputeCIDRs tests the CIDR computation logic
func TestComputeCIDRs(t *testing.T) {
	tests := []struct {
		name        string
		globalCIDR  string
		expectedPod string
		expectedSvc string
		expectedErr bool
	}{
		{
			name:        "valid cidr 10.0.0.0/16",
			globalCIDR:  "10.0.0.0/16",
			expectedPod: "10.0.0.0/17",
			expectedSvc: "10.0.128.0/17",
			expectedErr: false,
		},
		{
			name:        "valid cidr 192.168.0.0/16",
			globalCIDR:  "192.168.0.0/16",
			expectedPod: "192.168.0.0/17",
			expectedSvc: "192.168.128.0/17",
			expectedErr: false,
		},
		{
			name:        "no global cidr",
			globalCIDR:  "",
			expectedPod: "", // Should remain unchanged
			expectedSvc: "", // Should remain unchanged
			expectedErr: false,
		},
		{
			name:        "invalid cidr",
			globalCIDR:  "not-a-cidr",
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewInstallationManager()

			config := types.LinuxInstallationConfig{
				GlobalCIDR: tt.globalCIDR,
			}

			err := manager.computeCIDRs(&config)

			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedPod, config.PodCIDR)
				assert.Equal(t, tt.expectedSvc, config.ServiceCIDR)
			}
		})
	}
}

func TestConfigureHost(t *testing.T) {
	tests := []struct {
		name        string
		rc          runtimeconfig.RuntimeConfig
		setupMocks  func(*hostutils.MockHostUtils)
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
			setupMocks: func(hum *hostutils.MockHostUtils) {
				mock.InOrder(
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
			setupMocks: func(hum *hostutils.MockHostUtils) {
				mock.InOrder(
					hum.On("ConfigureHost", mock.Anything,
						mock.MatchedBy(func(rc runtimeconfig.RuntimeConfig) bool {
							return rc.EmbeddedClusterHomeDirectory() == "/var/lib/embedded-cluster"
						}),
						hostutils.InitForInstallOptions{
							License:      []byte("metadata:\n  name: test-license"),
							AirgapBundle: "bundle.tar",
						},
					).Return(errors.New("configuration failed")),
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

			// Setup mocks
			tt.setupMocks(mockHostUtils)

			// Create manager with mocks
			manager := NewInstallationManager(
				WithHostUtils(mockHostUtils),
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
			mockHostUtils.AssertExpectations(t)
		})
	}
}

// Helper to create a test license
func createTestLicense(licenseID, appSlug string) []byte {
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
func createTestReleaseData(appSlug string, domains *ecv1beta1.Domains) *release.ReleaseData {
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
func createTestRuntimeConfig() runtimeconfig.RuntimeConfig {
	return runtimeconfig.New(&ecv1beta1.RuntimeConfigSpec{
		Network: ecv1beta1.NetworkSpec{
			ServiceCIDR: "10.96.0.0/12",
		},
	})
}

func TestCalculateRegistrySettings(t *testing.T) {

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
				LocalRegistryUsername:  "embedded-cluster",
				LocalRegistryPassword:  registry.GetRegistryPassword(),
				ImagePullSecretName:    "test-app-registry",
				ImagePullSecretValue: func() string {
					authString := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("embedded-cluster:%s", registry.GetRegistryPassword())))
					return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(`{"auths":{"10.96.0.11:5000":{"username": "embedded-cluster", "password": "%s", "auth": "%s"}}}`, registry.GetRegistryPassword(), authString)))
				}(),
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

func TestGetRegistrySettings(t *testing.T) {
	tests := []struct {
		name              string
		license           []byte
		releaseData       *release.ReleaseData
		airgapBundle      string
		setupCluster      func() client.Client
		expectedResult    *types.RegistrySettings
		expectedError     string
		skipEnableV3Unset bool
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
			name:         "airgap mode with valid registry-creds secret",
			license:      createTestLicense("test-license", "test-app"),
			releaseData:  createTestReleaseData("test-app", nil),
			airgapBundle: "test-bundle.tar",
			setupCluster: func() client.Client {
				// Create a fake kubernetes client with the registry-creds secret
				authString := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("embedded-cluster:%s", registry.GetRegistryPassword())))
				dockerConfigJSON := fmt.Sprintf(`{"auths":{"10.96.0.11:5000":{"username": "embedded-cluster", "password": "%s", "auth": "%s"}}}`,
					registry.GetRegistryPassword(), authString)

				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "registry-creds",
						Namespace: "kotsadm",
					},
					Type: corev1.SecretTypeDockerConfigJson,
					Data: map[string][]byte{
						".dockerconfigjson": []byte(dockerConfigJSON),
					},
				}

				return fake.NewClientBuilder().WithObjects(secret).Build()
			},
			expectedResult: &types.RegistrySettings{
				HasLocalRegistry:       true,
				LocalRegistryHost:      "10.96.0.11:5000",
				LocalRegistryAddress:   "10.96.0.11:5000/test-app",
				LocalRegistryNamespace: "test-app",
				LocalRegistryUsername:  "embedded-cluster",
				LocalRegistryPassword:  registry.GetRegistryPassword(),
				ImagePullSecretName:    "test-app-registry",
				ImagePullSecretValue: func() string {
					authString := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("embedded-cluster:%s", registry.GetRegistryPassword())))
					dockerConfigJSON := fmt.Sprintf(`{"auths":{"10.96.0.11:5000":{"username": "embedded-cluster", "password": "%s", "auth": "%s"}}}`,
						registry.GetRegistryPassword(), authString)
					return base64.StdEncoding.EncodeToString([]byte(dockerConfigJSON))
				}(),
			},
		},
		{
			name:         "airgap mode with missing registry-creds secret",
			license:      createTestLicense("test-license", "test-app"),
			releaseData:  createTestReleaseData("test-app", nil),
			airgapBundle: "test-bundle.tar",
			setupCluster: func() client.Client {
				return fake.NewClientBuilder().Build()
			},
			expectedError: "get registry-creds secret:",
		},
		{
			name:         "airgap mode with invalid secret type",
			license:      createTestLicense("test-license", "test-app"),
			releaseData:  createTestReleaseData("test-app", nil),
			airgapBundle: "test-bundle.tar",
			setupCluster: func() client.Client {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "registry-creds",
						Namespace: "kotsadm",
					},
					Type: corev1.SecretTypeOpaque, // Wrong type
					Data: map[string][]byte{
						".dockerconfigjson": []byte(`{}`),
					},
				}
				return fake.NewClientBuilder().WithObjects(secret).Build()
			},
			expectedError: "registry-creds secret is not of type kubernetes.io/dockerconfigjson",
		},
		{
			name:         "airgap mode with missing dockerconfigjson",
			license:      createTestLicense("test-license", "test-app"),
			releaseData:  createTestReleaseData("test-app", nil),
			airgapBundle: "test-bundle.tar",
			setupCluster: func() client.Client {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "registry-creds",
						Namespace: "kotsadm",
					},
					Type: corev1.SecretTypeDockerConfigJson,
					Data: map[string][]byte{}, // Missing .dockerconfigjson
				}
				return fake.NewClientBuilder().WithObjects(secret).Build()
			},
			expectedError: "registry-creds secret missing .dockerconfigjson data",
		},
		{
			name:         "airgap mode with invalid json in dockerconfigjson",
			license:      createTestLicense("test-license", "test-app"),
			releaseData:  createTestReleaseData("test-app", nil),
			airgapBundle: "test-bundle.tar",
			setupCluster: func() client.Client {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "registry-creds",
						Namespace: "kotsadm",
					},
					Type: corev1.SecretTypeDockerConfigJson,
					Data: map[string][]byte{
						".dockerconfigjson": []byte("invalid json"),
					},
				}
				return fake.NewClientBuilder().WithObjects(secret).Build()
			},
			expectedError: "parse dockerconfigjson:",
		},
		{
			name:         "airgap mode with missing embedded-cluster username",
			license:      createTestLicense("test-license", "test-app"),
			releaseData:  createTestReleaseData("test-app", nil),
			airgapBundle: "test-bundle.tar",
			setupCluster: func() client.Client {
				dockerConfigJSON := `{"auths":{"registry.example.com":{"username": "other-user", "password": "other-pass"}}}`
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "registry-creds",
						Namespace: "kotsadm",
					},
					Type: corev1.SecretTypeDockerConfigJson,
					Data: map[string][]byte{
						".dockerconfigjson": []byte(dockerConfigJSON),
					},
				}
				return fake.NewClientBuilder().WithObjects(secret).Build()
			},
			expectedError: "embedded-cluster username not found in registry-creds secret",
		},
		{
			name:         "airgap mode missing release data",
			license:      createTestLicense("test-license", "test-app"),
			releaseData:  nil,
			airgapBundle: "test-bundle.tar",
			// No need to setup cluster - validation happens before cluster access
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
			airgapBundle: "test-bundle.tar",
			// No need to setup cluster - validation happens before cluster access
			expectedError: "release data with app slug is required for registry settings",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Ensure ENABLE_V3 is not set so KotsadmNamespace returns "kotsadm"
			if !tt.skipEnableV3Unset {
				t.Setenv("ENABLE_V3", "")
			}

			rc := createTestRuntimeConfig()

			var kcli client.Client
			if tt.setupCluster != nil {
				kcli = tt.setupCluster()
			}

			manager := NewInstallationManager(
				WithLicense(tt.license),
				WithReleaseData(tt.releaseData),
				WithAirgapBundle(tt.airgapBundle),
				WithKubeClient(kcli),
			)

			result, err := manager.GetRegistrySettings(context.Background(), rc)

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
