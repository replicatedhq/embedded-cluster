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
	// Create temporary directory for test files
	tmpDir := t.TempDir()

	// Create a valid config file
	validConfigFile := filepath.Join(tmpDir, "valid-config.yaml")
	validConfigContent := `apiVersion: kots.io/v1beta1
kind: ConfigValues
metadata:
  name: test-config
spec:
  values:
    database_host:
      value: "postgres.example.com"
    database_password:
      value: "secretpassword"`
	err := os.WriteFile(validConfigFile, []byte(validConfigContent), 0644)
	require.NoError(t, err)

	// Create an invalid YAML file
	invalidConfigFile := filepath.Join(tmpDir, "invalid-config.yaml")
	invalidConfigContent := `this is not valid: yaml: content [`
	err = os.WriteFile(invalidConfigFile, []byte(invalidConfigContent), 0644)
	require.NoError(t, err)

	tests := []struct {
		name          string
		flags         HeadlessInstallFlags
		expectErrors  bool
		errorContains []string
	}{
		{
			name: "valid flags with valid config file",
			flags: HeadlessInstallFlags{
				ConfigValues:         validConfigFile,
				AdminConsolePassword: "password123",
				Target:               string(apitypes.InstallTargetLinux),
			},
			expectErrors: false,
		},
		{
			name: "missing config values flag",
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
				ConfigValues:         validConfigFile,
				AdminConsolePassword: "",
				Target:               string(apitypes.InstallTargetLinux),
			},
			expectErrors:  true,
			errorContains: []string{"--admin-console-password flag is required"},
		},
		{
			name: "unsupported target",
			flags: HeadlessInstallFlags{
				ConfigValues:         validConfigFile,
				AdminConsolePassword: "password123",
				Target:               string(apitypes.InstallTargetKubernetes),
			},
			expectErrors:  true,
			errorContains: []string{"headless installation only supports --target=linux"},
		},
		{
			name: "config file not found",
			flags: HeadlessInstallFlags{
				ConfigValues:         "/nonexistent/config.yaml",
				AdminConsolePassword: "password123",
				Target:               string(apitypes.InstallTargetLinux),
			},
			expectErrors:  true,
			errorContains: []string{"config values file not found"},
		},
		{
			name: "invalid YAML in config file",
			flags: HeadlessInstallFlags{
				ConfigValues:         invalidConfigFile,
				AdminConsolePassword: "password123",
				Target:               string(apitypes.InstallTargetLinux),
			},
			expectErrors:  true,
			errorContains: []string{"failed to parse config values"},
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
		{
			name: "multiple errors - missing password and bad target",
			flags: HeadlessInstallFlags{
				ConfigValues:         validConfigFile,
				AdminConsolePassword: "",
				Target:               string(apitypes.InstallTargetKubernetes),
			},
			expectErrors:  true,
			errorContains: []string{"--admin-console-password flag is required", "headless installation only supports --target=linux"},
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

func TestValidateHeadlessInstallFlags_EmptyConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	emptyConfigFile := filepath.Join(tmpDir, "empty-config.yaml")
	emptyConfigContent := `apiVersion: kots.io/v1beta1
kind: ConfigValues
metadata:
  name: test-config`
	err := os.WriteFile(emptyConfigFile, []byte(emptyConfigContent), 0644)
	require.NoError(t, err)

	flags := HeadlessInstallFlags{
		ConfigValues:         emptyConfigFile,
		AdminConsolePassword: "password123",
		Target:               string(apitypes.InstallTargetLinux),
	}

	errors := ValidateHeadlessInstallFlags(flags)
	// Empty config file should still be valid (no spec.values is okay)
	assert.Empty(t, errors, "expected no validation errors for empty config file")
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
