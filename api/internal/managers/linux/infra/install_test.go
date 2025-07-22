package infra

import (
	"os"
	"testing"

	"github.com/replicatedhq/embedded-cluster/cmd/installer/kotscli"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	kyaml "sigs.k8s.io/yaml"
)

// MockKotsCLIInstaller is a mock implementation of the KotsCLIInstaller interface
type MockKotsCLIInstaller struct {
	mock.Mock
}

func (m *MockKotsCLIInstaller) Install(opts kotscli.InstallOptions) error {
	return m.Called(opts).Error(0)
}

func TestInfraManager_getAddonInstallOpts(t *testing.T) {
	tests := []struct {
		name              string
		configValues      kotsv1beta1.ConfigValues
		setupMock         func(m *MockKotsCLIInstaller)
		verifyInstallOpts func(t *testing.T, opts addons.InstallOptions)
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
			setupMock: func(m *MockKotsCLIInstaller) {
				m.On("Install", mock.MatchedBy(func(opts kotscli.InstallOptions) bool {
					if opts.ConfigValuesFile == "" {
						return false
					}
					b, err := os.ReadFile(opts.ConfigValuesFile)
					if err != nil {
						return false
					}
					var cv kotsv1beta1.ConfigValues
					if err := kyaml.Unmarshal(b, &cv); err != nil {
						return false
					}
					if cv.Spec.Values["key1"].Value != "value1" || cv.Spec.Values["key2"].Value != "value2" {
						return false
					}
					return true
				})).Return(nil)
			},
			verifyInstallOpts: func(t *testing.T, opts addons.InstallOptions) {
				assert.NotNil(t, opts.KotsInstaller)
				// Test that the KotsInstaller function works correctly
				err := opts.KotsInstaller()
				assert.NoError(t, err)
			},
		},

		// Basic options tests
		{
			name:         "basic options should be set correctly",
			configValues: kotsv1beta1.ConfigValues{},
			setupMock: func(m *MockKotsCLIInstaller) {
				m.On("Install", mock.MatchedBy(func(opts kotscli.InstallOptions) bool {
					return opts.AppSlug == "test-app" &&
						opts.License != nil &&
						opts.Namespace == "kotsadm" &&
						opts.ClusterID == "test-cluster" &&
						opts.AirgapBundle == "" &&
						opts.ConfigValuesFile != "" &&
						opts.ReplicatedAppEndpoint == "https://replicated.app"
				})).Return(nil)
			},
			verifyInstallOpts: func(t *testing.T, opts addons.InstallOptions) {
				assert.NotNil(t, opts.KotsInstaller)
				assert.NotNil(t, opts.ClusterID)
				assert.NotNil(t, opts.License)
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
			mockInstaller := &MockKotsCLIInstaller{}
			tt.setupMock(mockInstaller)

			// Create test license
			license := &kotsv1beta1.License{
				Spec: kotsv1beta1.LicenseSpec{
					AppSlug: "test-app",
				},
			}

			// Create infra manager with CLI config file - use mock installer to test the priority logic
			manager := NewInfraManager(
				WithClusterID("test-cluster"),
				WithLicense([]byte("test-license")),
				WithKotsCLIInstaller(mockInstaller),
			)

			// Test the getAddonInstallOpts method with configValues passed as parameter
			opts := manager.getAddonInstallOpts(license, rc, tt.configValues)

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
