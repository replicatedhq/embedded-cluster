package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateHeadlessInstallFlags(t *testing.T) {
	tests := []struct {
		name          string
		flags         HeadlessInstallFlags
		expectErrors  bool
		errorContains []string
	}{
		{
			name: "valid flags",
			flags: HeadlessInstallFlags{
				ConfigValues:         "/path/to/config.yaml",
				AdminConsolePassword: "password123",
				Target:               string(apitypes.InstallTargetLinux),
			},
			expectErrors: false,
		},
		{
			name: "missing config values",
			flags: HeadlessInstallFlags{
				ConfigValues:         "",
				AdminConsolePassword: "password123",
				Target:               string(apitypes.InstallTargetLinux),
			},
			expectErrors:  true,
			errorContains: []string{"--config-values flag is required"},
		},
		{
			name: "missing admin console password",
			flags: HeadlessInstallFlags{
				ConfigValues:         "/path/to/config.yaml",
				AdminConsolePassword: "",
				Target:               string(apitypes.InstallTargetLinux),
			},
			expectErrors:  true,
			errorContains: []string{"--admin-console-password flag is required"},
		},
		{
			name: "unsupported target",
			flags: HeadlessInstallFlags{
				ConfigValues:         "/path/to/config.yaml",
				AdminConsolePassword: "password123",
				Target:               string(apitypes.InstallTargetKubernetes),
			},
			expectErrors:  true,
			errorContains: []string{"headless installation only supports --target=linux"},
		},
		{
			name: "all flags missing",
			flags: HeadlessInstallFlags{
				ConfigValues:         "",
				AdminConsolePassword: "",
				Target:               "",
			},
			expectErrors:  true,
			errorContains: []string{"--config-values flag is required", "--admin-console-password flag is required", "headless installation only supports --target=linux"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := ValidateHeadlessInstallFlags(tt.flags)

			if tt.expectErrors {
				assert.NotEmpty(t, errors, "expected validation errors but got none")
				// Check that each expected error substring is found in at least one error message
				for _, expectedError := range tt.errorContains {
					found := false
					for _, err := range errors {
						if strings.Contains(err, expectedError) {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("expected error containing %q not found in errors: %v", expectedError, errors)
					}
				}
			} else {
				assert.Empty(t, errors, "expected no validation errors but got: %v", errors)
			}
		})
	}
}

func TestValidateAndLoadConfigValues(t *testing.T) {
	tests := []struct {
		name          string
		configContent string
		expectError   bool
		errorContains string
	}{
		{
			name: "valid config values",
			configContent: `apiVersion: kots.io/v1beta1
kind: ConfigValues
metadata:
  name: test-config
spec:
  values:
    database_host:
      value: "postgres.example.com"
    database_password:
      value: "secretpassword"`,
			expectError: false,
		},
		{
			name:          "invalid YAML",
			configContent: `this is not valid: yaml: content [`,
			expectError:   true,
			errorContains: "failed to parse config values",
		},
		{
			name: "empty config values",
			configContent: `apiVersion: kots.io/v1beta1
kind: ConfigValues
metadata:
  name: test-config`,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary file with the config content
			tmpDir := t.TempDir()
			configFile := filepath.Join(tmpDir, "config.yaml")
			err := os.WriteFile(configFile, []byte(tt.configContent), 0644)
			require.NoError(t, err)

			result, err := ValidateAndLoadConfigValues(configFile)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.True(t, result.IsValid)
				assert.NotNil(t, result.ConfigValues)
			}
		})
	}
}

func TestValidateAndLoadConfigValues_FileNotFound(t *testing.T) {
	result, err := ValidateAndLoadConfigValues("/nonexistent/config.yaml")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config values file not found")
	assert.False(t, result.IsValid)
	assert.Contains(t, result.ValidationErrors[0], "config values file not found")
}

func TestFormatValidationErrors(t *testing.T) {
	tests := []struct {
		name     string
		errors   []string
		expected string
	}{
		{
			name:     "no errors",
			errors:   []string{},
			expected: "",
		},
		{
			name:     "single error",
			errors:   []string{"error 1"},
			expected: "Validation failed:\n  - error 1\n",
		},
		{
			name:     "multiple errors",
			errors:   []string{"error 1", "error 2", "error 3"},
			expected: "Validation failed:\n  - error 1\n  - error 2\n  - error 3\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatValidationErrors(tt.errors)
			assert.Equal(t, tt.expected, result)
		})
	}
}
