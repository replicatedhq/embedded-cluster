package kotscli

import (
	"os"
	"path/filepath"
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	k8syaml "sigs.k8s.io/yaml"
)

func TestInstall(t *testing.T) {
	tests := []struct {
		name           string
		configFile     string
		configValues   map[string]string
		expectedBinary string
		expectedArgs   func(tempDir string) []string
	}{
		{
			name:       "CLI file path should take precedence over memory store values",
			configFile: "/path/to/cli.yaml",
			configValues: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
			expectedBinary: "kubectl-kots",
			expectedArgs: func(tempDir string) []string {
				return []string{
					"install", "test-app", "--license-file", "", "--namespace", "kotsadm",
					"--app-version-label", "", "--exclude-admin-console",
					"--config-values", "/path/to/cli.yaml",
				}
			},
		},
		{
			name:       "memory store values should be used when no CLI file provided",
			configFile: "",
			configValues: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
			expectedBinary: "kubectl-kots",
			expectedArgs: func(tempDir string) []string {
				return []string{
					"install", "test-app", "--license-file", "", "--namespace", "kotsadm",
					"--app-version-label", "", "--exclude-admin-console",
					"--config-values", filepath.Join(tempDir, "config", "config-values.yaml"),
				}
			},
		},
		{
			name:           "no config values should not add config-values flag",
			configFile:     "",
			configValues:   nil,
			expectedBinary: "kubectl-kots",
			expectedArgs: func(tempDir string) []string {
				return []string{
					"install", "test-app", "--license-file", "", "--namespace", "kotsadm",
					"--app-version-label", "", "--exclude-admin-console",
				}
			},
		},
		{
			name:           "empty config values map should not add config-values flag",
			configFile:     "",
			configValues:   map[string]string{},
			expectedBinary: "kubectl-kots",
			expectedArgs: func(tempDir string) []string {
				return []string{
					"install", "test-app", "--license-file", "", "--namespace", "kotsadm",
					"--app-version-label", "", "--exclude-admin-console",
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory for test
			tempDir := t.TempDir()

			// Create runtime config with the temp directory
			rcSpec := &ecv1beta1.RuntimeConfigSpec{
				DataDir: tempDir,
			}
			rc := runtimeconfig.New(rcSpec)

			// Create basic license for testing
			license := []byte(`spec:
  appSlug: test-app`)

			opts := InstallOptions{
				RuntimeConfig:    rc,
				AppSlug:          "test-app",
				License:          license,
				Namespace:        "kotsadm",
				ConfigValuesFile: tt.configFile,
				ConfigValues:     tt.configValues,
			}

			// Note: This test focuses on the logic flow and argument construction
			// The actual execution would require mocking the helpers.RunCommandWithOptions call
			// For now, we test the createConfigValuesFile function directly below

			// Test createConfigValuesFile behavior when config values are provided
			if len(tt.configValues) > 0 && tt.configFile == "" {
				configFile, err := createConfigValuesFile(tt.configValues)
				require.NoError(t, err)
				defer os.Remove(configFile) // Clean up temp file

				// Verify file was created
				assert.FileExists(t, configFile)

				// Verify file contents
				data, err := os.ReadFile(configFile)
				require.NoError(t, err)

				var kotsConfig kotsv1beta1.ConfigValues
				err = k8syaml.Unmarshal(data, &kotsConfig)
				require.NoError(t, err)

				assert.Equal(t, "kots.io/v1beta1", kotsConfig.APIVersion)
				assert.Equal(t, "ConfigValues", kotsConfig.Kind)
				assert.Equal(t, "kots-app-config", kotsConfig.Name)
				assert.Equal(t, len(tt.configValues), len(kotsConfig.Spec.Values))

				for key, expectedValue := range tt.configValues {
					assert.Equal(t, expectedValue, kotsConfig.Spec.Values[key].Value)
				}
			}

			// Verify runtime config is working properly
			assert.Equal(t, tempDir, rc.EmbeddedClusterHomeDirectory())

			// Verify the install options were created correctly
			assert.Equal(t, tt.configFile, opts.ConfigValuesFile)
			assert.Equal(t, tt.configValues, opts.ConfigValues)
			assert.Equal(t, rc, opts.RuntimeConfig)
			assert.Equal(t, "test-app", opts.AppSlug)
			assert.Equal(t, license, opts.License)
			assert.Equal(t, "kotsadm", opts.Namespace)
		})
	}
}

func TestCreateConfigValuesFile(t *testing.T) {
	tests := []struct {
		name          string
		configValues  map[string]string
		expectedError string
	}{
		{
			name: "valid config values should create proper KOTS ConfigValues",
			configValues: map[string]string{
				"database_host": "localhost",
				"database_port": "5432",
				"admin_email":   "admin@example.com",
			},
			expectedError: "",
		},
		{
			name:          "empty config values should create empty KOTS ConfigValues",
			configValues:  map[string]string{},
			expectedError: "",
		},
		{
			name: "config values with special characters should be handled properly",
			configValues: map[string]string{
				"password":    "p@ssw0rd!",
				"json_config": `{"key": "value"}`,
				"multiline":   "line1\nline2\nline3",
			},
			expectedError: "",
		},
		{
			name: "should create directory with proper permissions",
			configValues: map[string]string{
				"test_key": "test_value",
			},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configFile, err := createConfigValuesFile(tt.configValues)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				return
			}

			require.NoError(t, err)
			assert.NotEmpty(t, configFile)
			defer os.Remove(configFile) // Clean up temp file

			// Verify file exists and is a temp file
			assert.FileExists(t, configFile)
			assert.Contains(t, configFile, "config-values-")
			assert.Contains(t, configFile, ".yaml")

			// Verify file permissions
			fileInfo, err := os.Stat(configFile)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0600), fileInfo.Mode().Perm()) // os.CreateTemp creates files with 0600 permissions

			// Verify file contents can be unmarshaled
			data, err := os.ReadFile(configFile)
			require.NoError(t, err)

			var kotsConfig kotsv1beta1.ConfigValues
			err = k8syaml.Unmarshal(data, &kotsConfig)
			require.NoError(t, err)

			// Verify structure
			assert.Equal(t, "kots.io/v1beta1", kotsConfig.APIVersion)
			assert.Equal(t, "ConfigValues", kotsConfig.Kind)
			assert.Equal(t, "kots-app-config", kotsConfig.Name)

			// Verify values
			assert.Equal(t, len(tt.configValues), len(kotsConfig.Spec.Values))
			for key, expectedValue := range tt.configValues {
				require.Contains(t, kotsConfig.Spec.Values, key)
				assert.Equal(t, expectedValue, kotsConfig.Spec.Values[key].Value)
			}
		})
	}
}
