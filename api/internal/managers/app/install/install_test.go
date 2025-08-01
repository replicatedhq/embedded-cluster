package install

import (
	"context"
	"os"
	"testing"

	kotscli "github.com/replicatedhq/embedded-cluster/cmd/installer/kotscli"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	kyaml "sigs.k8s.io/yaml"
)

func TestAppInstallManager_Install(t *testing.T) {
	tests := []struct {
		name         string
		configValues kotsv1beta1.ConfigValues
		setupMock    func(t *testing.T, m *MockKotsCLIInstaller)
		expectError  bool
	}{
		{
			name: "Config values should be passed to the installer",
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
			setupMock: func(t *testing.T, m *MockKotsCLIInstaller) {
				m.On("Install", mock.MatchedBy(func(opts kotscli.InstallOptions) bool {
					// Verify basic install options
					if opts.AppSlug != "test-app" {
						t.Logf("AppSlug mismatch: expected 'test-app', got '%s'", opts.AppSlug)
						return false
					}
					if opts.License == nil {
						t.Logf("License is nil")
						return false
					}
					if opts.Namespace != "kotsadm" {
						t.Logf("Namespace mismatch: expected 'kotsadm', got '%s'", opts.Namespace)
						return false
					}
					if opts.ClusterID != "test-cluster" {
						t.Logf("ClusterID mismatch: expected 'test-cluster', got '%s'", opts.ClusterID)
						return false
					}
					if opts.AirgapBundle != "test-airgap.tar.gz" {
						t.Logf("AirgapBundle mismatch: expected 'test-airgap.tar.gz', got '%s'", opts.AirgapBundle)
						return false
					}
					if opts.ReplicatedAppEndpoint == "" {
						t.Logf("ReplicatedAppEndpoint is empty")
						return false
					}
					if opts.ConfigValuesFile == "" {
						t.Logf("ConfigValuesFile is empty")
						return false
					}

					// Verify config values file content
					b, err := os.ReadFile(opts.ConfigValuesFile)
					if err != nil {
						t.Logf("Failed to read config values file: %v", err)
						return false
					}
					var cv kotsv1beta1.ConfigValues
					if err := kyaml.Unmarshal(b, &cv); err != nil {
						t.Logf("Failed to unmarshal config values: %v", err)
						return false
					}
					if cv.Spec.Values["key1"].Value != "value1" {
						t.Logf("Config value key1 mismatch: expected 'value1', got '%s'", cv.Spec.Values["key1"].Value)
						return false
					}
					if cv.Spec.Values["key2"].Value != "value2" {
						t.Logf("Config value key2 mismatch: expected 'value2', got '%s'", cv.Spec.Values["key2"].Value)
						return false
					}
					return true
				})).Return(nil)
			},
			expectError: false,
		},
		{
			name:         "Install error should be propagated",
			configValues: kotsv1beta1.ConfigValues{},
			setupMock: func(t *testing.T, m *MockKotsCLIInstaller) {
				m.On("Install", mock.Anything).Return(assert.AnError)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test license
			license := &kotsv1beta1.License{
				Spec: kotsv1beta1.LicenseSpec{
					AppSlug: "test-app",
				},
			}
			licenseBytes, err := kyaml.Marshal(license)
			assert.NoError(t, err)

			// Create test release data with minimal ChannelRelease
			releaseData := &release.ReleaseData{
				ChannelRelease: &release.ChannelRelease{
					DefaultDomains: release.Domains{
						ReplicatedAppDomain: "replicated.app",
					},
				},
			}

			// Create mock installer
			mockInstaller := &MockKotsCLIInstaller{}
			tt.setupMock(t, mockInstaller)

			// Create app install manager
			manager, err := NewAppInstallManager(
				WithLicense(licenseBytes),
				WithClusterID("test-cluster"),
				WithAirgapBundle("test-airgap.tar.gz"),
				WithReleaseData(releaseData),
				WithKotsCLI(mockInstaller),
			)
			assert.NoError(t, err)

			// Test the Install method
			err = manager.Install(context.Background(), tt.configValues)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Verify mock expectations
			mockInstaller.AssertExpectations(t)
		})
	}
}

func TestAppInstallManager_createConfigValuesFile(t *testing.T) {
	manager := &appInstallManager{}

	configValues := kotsv1beta1.ConfigValues{
		Spec: kotsv1beta1.ConfigValuesSpec{
			Values: map[string]kotsv1beta1.ConfigValue{
				"testKey": {
					Value: "testValue",
				},
			},
		},
	}

	filename, err := manager.createConfigValuesFile(configValues)
	assert.NoError(t, err)
	assert.NotEmpty(t, filename)

	// Verify file exists and contains correct content
	data, err := os.ReadFile(filename)
	assert.NoError(t, err)

	var unmarshaled kotsv1beta1.ConfigValues
	err = kyaml.Unmarshal(data, &unmarshaled)
	assert.NoError(t, err)
	assert.Equal(t, "testValue", unmarshaled.Spec.Values["testKey"].Value)

	// Clean up
	os.Remove(filename)
}
