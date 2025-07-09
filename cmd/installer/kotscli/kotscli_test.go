package kotscli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	k8syaml "sigs.k8s.io/yaml"
)

func TestCreateConfigValuesFile(t *testing.T) {
	tests := []struct {
		name                    string
		configValues            *kotsv1beta1.ConfigValues
		setupFunc               func(string) // setup function to prepare test environment
		expectError             bool
		verifyDirectoryCreation bool
		expectedYAMLContent     func(string) string // function to generate expected YAML content
	}{
		{
			name: "valid config values should create file successfully",
			configValues: &kotsv1beta1.ConfigValues{
				Spec: kotsv1beta1.ConfigValuesSpec{
					Values: map[string]kotsv1beta1.ConfigValue{
						"test-key": {
							Value: "test-value",
						},
					},
				},
			},
			setupFunc:   func(tempDir string) {}, // no special setup needed
			expectError: false,
			expectedYAMLContent: func(tempDir string) string {
				return `apiVersion: kots.io/v1beta1
kind: ConfigValues
spec:
  values:
    test-key:
      value: test-value
status: {}
`
			},
		},
		{
			name: "empty config values should create empty file successfully",
			configValues: &kotsv1beta1.ConfigValues{
				Spec: kotsv1beta1.ConfigValuesSpec{
					Values: map[string]kotsv1beta1.ConfigValue{},
				},
			},
			setupFunc:   func(tempDir string) {}, // no special setup needed
			expectError: false,
			expectedYAMLContent: func(tempDir string) string {
				return `apiVersion: kots.io/v1beta1
kind: ConfigValues
spec:
  values: {}
status: {}
`
			},
		},
		{
			name: "should create config directory when it doesn't exist",
			configValues: &kotsv1beta1.ConfigValues{
				Spec: kotsv1beta1.ConfigValuesSpec{
					Values: map[string]kotsv1beta1.ConfigValue{
						"test": {Value: "value"},
					},
				},
			},
			setupFunc: func(tempDir string) {
				// Ensure config directory doesn't exist initially
				configDir := filepath.Join(tempDir, "config")
				os.RemoveAll(configDir)
			},
			expectError:             false,
			verifyDirectoryCreation: true,
			expectedYAMLContent: func(tempDir string) string {
				return `apiVersion: kots.io/v1beta1
kind: ConfigValues
spec:
  values:
    test:
      value: value
status: {}
`
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary directory for testing
			tempDir, err := os.MkdirTemp("", "kotscli-test-")
			require.NoError(t, err)
			defer os.RemoveAll(tempDir)

			// Run setup function if provided
			if tt.setupFunc != nil {
				tt.setupFunc(tempDir)
			}

			// Create a mock runtime config
			mockRC := &runtimeconfig.MockRuntimeConfig{}
			mockRC.On("EmbeddedClusterHomeDirectory").Return(tempDir)

			configDir := filepath.Join(tempDir, "config")

			// Verify directory doesn't exist initially if we're testing directory creation
			if tt.verifyDirectoryCreation {
				assert.NoDirExists(t, configDir)
			}

			filePath, err := createConfigValuesFile(tt.configValues, mockRC)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotEmpty(t, filePath)

			// Verify file was created
			assert.FileExists(t, filePath)

			// Verify directory was created
			assert.DirExists(t, configDir)

			// Verify file content
			content, err := os.ReadFile(filePath)
			require.NoError(t, err)

			// Check that the YAML has proper structure
			var parsedConfig kotsv1beta1.ConfigValues
			err = k8syaml.Unmarshal(content, &parsedConfig)
			require.NoError(t, err)

			// Verify the spec values match
			assert.Equal(t, tt.configValues.Spec.Values, parsedConfig.Spec.Values)

			// Verify the YAML content has proper apiVersion and kind at the top level
			contentStr := string(content)
			assert.Contains(t, contentStr, "apiVersion: kots.io/v1beta1")
			assert.Contains(t, contentStr, "kind: ConfigValues")

			// Verify file path structure
			expectedFile := filepath.Join(configDir, "config-values.yaml")
			assert.Equal(t, expectedFile, filePath)

			// Verify mock expectations
			mockRC.AssertExpectations(t)
		})
	}
}
