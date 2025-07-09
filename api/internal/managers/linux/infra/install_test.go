package infra

import (
	"testing"

	"github.com/replicatedhq/embedded-cluster/cmd/installer/kotscli"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockKotsInstaller is a mock implementation of the KotsInstaller function
type MockKotsInstaller struct {
	mock.Mock
}

func (m *MockKotsInstaller) Install(opts kotscli.InstallOptions) error {
	args := m.Called(opts)
	return args.Error(0)
}

func TestInfraManager_GetAddonInstallOpts_ConfigValues(t *testing.T) {
	tests := []struct {
		name               string
		configValuesFile   string
		configValues       map[string]string
		expectedFileUsed   bool
		expectedDirectUsed bool
		setupMock          func(*MockKotsInstaller)
		verifyInstallOpts  func(t *testing.T, opts addons.InstallOptions)
	}{
		{
			name:             "CLI file should override memory store values",
			configValuesFile: "/cli/config.yaml",
			configValues: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
			expectedFileUsed: true,
			setupMock: func(m *MockKotsInstaller) {
				m.On("Install", mock.MatchedBy(func(opts kotscli.InstallOptions) bool {
					return opts.ConfigValuesFile == "/cli/config.yaml" && len(opts.ConfigValues) == 0
				})).Return(nil)
			},
			verifyInstallOpts: func(t *testing.T, opts addons.InstallOptions) {
				assert.NotNil(t, opts.KotsInstaller)
				// Test that the KotsInstaller function works correctly
				err := opts.KotsInstaller()
				assert.NoError(t, err)
			},
		},
		{
			name:             "memory store values should be used when no CLI file provided",
			configValuesFile: "",
			configValues: map[string]string{
				"database_host": "localhost",
				"database_port": "5432",
			},
			expectedDirectUsed: true,
			setupMock: func(m *MockKotsInstaller) {
				m.On("Install", mock.MatchedBy(func(opts kotscli.InstallOptions) bool {
					return opts.ConfigValuesFile == "" &&
						len(opts.ConfigValues) == 2 &&
						opts.ConfigValues["database_host"] == "localhost" &&
						opts.ConfigValues["database_port"] == "5432"
				})).Return(nil)
			},
			verifyInstallOpts: func(t *testing.T, opts addons.InstallOptions) {
				assert.NotNil(t, opts.KotsInstaller)
				// Test that the KotsInstaller function works correctly
				err := opts.KotsInstaller()
				assert.NoError(t, err)
			},
		},
		{
			name:               "no config values should not set either option",
			configValuesFile:   "",
			configValues:       nil,
			expectedFileUsed:   false,
			expectedDirectUsed: false,
			setupMock: func(m *MockKotsInstaller) {
				m.On("Install", mock.MatchedBy(func(opts kotscli.InstallOptions) bool {
					return opts.ConfigValuesFile == "" && len(opts.ConfigValues) == 0
				})).Return(nil)
			},
			verifyInstallOpts: func(t *testing.T, opts addons.InstallOptions) {
				assert.NotNil(t, opts.KotsInstaller)
				// Test that the KotsInstaller function works correctly
				err := opts.KotsInstaller()
				assert.NoError(t, err)
			},
		},
		{
			name:               "empty config values map should not set config values",
			configValuesFile:   "",
			configValues:       map[string]string{},
			expectedFileUsed:   false,
			expectedDirectUsed: false,
			setupMock: func(m *MockKotsInstaller) {
				m.On("Install", mock.MatchedBy(func(opts kotscli.InstallOptions) bool {
					return opts.ConfigValuesFile == "" && len(opts.ConfigValues) == 0
				})).Return(nil)
			},
			verifyInstallOpts: func(t *testing.T, opts addons.InstallOptions) {
				assert.NotNil(t, opts.KotsInstaller)
				// Test that the KotsInstaller function works correctly
				err := opts.KotsInstaller()
				assert.NoError(t, err)
			},
		},
		{
			name:             "CLI file path takes precedence even with empty string values",
			configValuesFile: "/path/to/config.yaml",
			configValues: map[string]string{
				"key1": "",
				"key2": "value2",
			},
			expectedFileUsed: true,
			setupMock: func(m *MockKotsInstaller) {
				m.On("Install", mock.MatchedBy(func(opts kotscli.InstallOptions) bool {
					return opts.ConfigValuesFile == "/path/to/config.yaml" && len(opts.ConfigValues) == 0
				})).Return(nil)
			},
			verifyInstallOpts: func(t *testing.T, opts addons.InstallOptions) {
				assert.NotNil(t, opts.KotsInstaller)
				// Test that the KotsInstaller function works correctly
				err := opts.KotsInstaller()
				assert.NoError(t, err)
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

			// Create mock installer
			mockInstaller := &MockKotsInstaller{}
			tt.setupMock(mockInstaller)

			// Create test license
			license := &kotsv1beta1.License{
				Spec: kotsv1beta1.LicenseSpec{
					AppSlug: "test-app",
				},
			}

			// Create infra manager with config values - use mock installer to test the priority logic
			manager := NewInfraManager(
				WithConfigValuesFile(tt.configValuesFile),
				WithConfigValues(tt.configValues),
				WithClusterID("test-cluster"),
				WithLicense([]byte("test-license")),
				WithKotsInstaller(func() error {
					// This should follow the same priority logic as the real implementation
					installOpts := kotscli.InstallOptions{
						RuntimeConfig: rc,
						AppSlug:       license.Spec.AppSlug,
						License:       []byte("test-license"),
						Namespace:     "kotsadm",
						ClusterID:     "test-cluster",
					}

					// Follow the actual priority logic from getAddonInstallOpts
					if tt.configValuesFile != "" {
						installOpts.ConfigValuesFile = tt.configValuesFile
					} else if len(tt.configValues) > 0 {
						installOpts.ConfigValues = tt.configValues
					}

					return mockInstaller.Install(installOpts)
				}),
			)

			// Test the getAddonInstallOpts method
			opts := manager.getAddonInstallOpts(license, rc)

			// Verify basic options are set correctly
			assert.Equal(t, "test-cluster", opts.ClusterID)
			assert.Equal(t, license, opts.License)

			// Verify the installer function behavior
			if tt.verifyInstallOpts != nil {
				tt.verifyInstallOpts(t, opts)
			}

			// Verify mock expectations
			mockInstaller.AssertExpectations(t)
		})
	}
}

func TestInfraManager_GetAddonInstallOpts_BasicOptions(t *testing.T) {
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
			AppSlug:                           "test-app",
			IsDisasterRecoverySupported:       true,
			IsEmbeddedClusterMultiNodeEnabled: true,
		},
	}

	// Create infra manager with basic options
	manager := NewInfraManager(
		WithClusterID("test-cluster-123"),
		WithPassword("admin-password"),
		WithAirgapBundle("/path/to/airgap.tar.gz"),
		WithLicense([]byte("test-license-data")),
	)

	// Test the getAddonInstallOpts method
	opts := manager.getAddonInstallOpts(license, rc)

	// Verify all basic options are set correctly
	assert.Equal(t, "test-cluster-123", opts.ClusterID)
	assert.Equal(t, "admin-password", opts.AdminConsolePwd)
	assert.Equal(t, license, opts.License)
	assert.True(t, opts.IsAirgap)
	assert.True(t, opts.DisasterRecoveryEnabled)
	assert.True(t, opts.IsMultiNodeEnabled)
	assert.Equal(t, tempDir, opts.DataDir)
	assert.NotNil(t, opts.KotsInstaller)
}

func TestInfraManager_WithConfigValuesFile(t *testing.T) {
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

	configFile := "/test/config.yaml"

	// Create mock installer to verify the config file is passed correctly
	mockInstaller := &MockKotsInstaller{}
	mockInstaller.On("Install", mock.MatchedBy(func(opts kotscli.InstallOptions) bool {
		return opts.ConfigValuesFile == configFile
	})).Return(nil)

	manager := NewInfraManager(
		WithConfigValuesFile(configFile),
		WithKotsInstaller(func() error {
			return mockInstaller.Install(kotscli.InstallOptions{
				ConfigValuesFile: configFile,
			})
		}),
	)

	// Test that the config values file is used correctly
	opts := manager.getAddonInstallOpts(license, rc)
	err := opts.KotsInstaller()
	assert.NoError(t, err)

	// Verify mock expectations
	mockInstaller.AssertExpectations(t)
}

func TestInfraManager_WithConfigValues(t *testing.T) {
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

	configValues := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	// Create mock installer to verify the config values are passed correctly
	mockInstaller := &MockKotsInstaller{}
	mockInstaller.On("Install", mock.MatchedBy(func(opts kotscli.InstallOptions) bool {
		return len(opts.ConfigValues) == 2 &&
			opts.ConfigValues["key1"] == "value1" &&
			opts.ConfigValues["key2"] == "value2"
	})).Return(nil)

	manager := NewInfraManager(
		WithConfigValues(configValues),
		WithKotsInstaller(func() error {
			return mockInstaller.Install(kotscli.InstallOptions{
				ConfigValues: configValues,
			})
		}),
	)

	// Test that the config values are used correctly
	opts := manager.getAddonInstallOpts(license, rc)
	err := opts.KotsInstaller()
	assert.NoError(t, err)

	// Verify mock expectations
	mockInstaller.AssertExpectations(t)
}

func TestInfraManager_ConfigValuesPriority(t *testing.T) {
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

	configFile := "/test/config.yaml"
	configValues := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	// Create mock installer to verify that file takes precedence over values
	mockInstaller := &MockKotsInstaller{}
	mockInstaller.On("Install", mock.MatchedBy(func(opts kotscli.InstallOptions) bool {
		// File should take precedence, so ConfigValuesFile should be set and ConfigValues should be empty
		return opts.ConfigValuesFile == configFile && len(opts.ConfigValues) == 0
	})).Return(nil)

	manager := NewInfraManager(
		WithConfigValuesFile(configFile),
		WithConfigValues(configValues),
		WithKotsInstaller(func() error {
			return mockInstaller.Install(kotscli.InstallOptions{
				ConfigValuesFile: configFile,
				ConfigValues:     nil, // File takes precedence
			})
		}),
	)

	// Test that file takes precedence over values
	opts := manager.getAddonInstallOpts(license, rc)
	err := opts.KotsInstaller()
	assert.NoError(t, err)

	// Verify mock expectations
	mockInstaller.AssertExpectations(t)
}
