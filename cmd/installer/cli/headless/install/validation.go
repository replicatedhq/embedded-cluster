package install

import (
	"fmt"
	"os"

	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
)

// HeadlessInstallFlags contains the flags required for headless installation
type HeadlessInstallFlags struct {
	ConfigValues         string
	AdminConsolePassword string
	Target               string
}

// ValidationResult contains the results of headless install validation
type ValidationResult struct {
	ConfigValues     *kotsv1beta1.ConfigValues
	AppConfigValues  apitypes.AppConfigValues
	IsValid          bool
	ValidationErrors []string
}

// ValidateHeadlessInstallFlags validates that all required flags are present for headless installation
func ValidateHeadlessInstallFlags(flags HeadlessInstallFlags) []string {
	var errors []string

	if flags.ConfigValues == "" {
		errors = append(errors, "--config-values flag is required for headless installation")
	}

	if flags.AdminConsolePassword == "" {
		errors = append(errors, "--admin-console-password flag is required for headless installation")
	}

	if flags.Target != string(apitypes.InstallTargetLinux) {
		errors = append(errors, fmt.Sprintf("headless installation only supports --target=linux (got: %s)", flags.Target))
	}

	return errors
}

// ValidateAndLoadConfigValues validates the config values file and loads it
func ValidateAndLoadConfigValues(configValuesPath string) (*ValidationResult, error) {
	result := &ValidationResult{
		IsValid: true,
	}

	// Check if file exists
	if _, err := os.Stat(configValuesPath); err != nil {
		if os.IsNotExist(err) {
			result.IsValid = false
			result.ValidationErrors = append(result.ValidationErrors, fmt.Sprintf("config values file not found: %s", configValuesPath))
			return result, fmt.Errorf("config values file not found: %s", configValuesPath)
		}
		result.IsValid = false
		result.ValidationErrors = append(result.ValidationErrors, fmt.Sprintf("failed to access config values file: %v", err))
		return result, fmt.Errorf("failed to access config values file: %w", err)
	}

	// Parse the config values file
	kotsConfigValues, err := helpers.ParseConfigValues(configValuesPath)
	if err != nil {
		result.IsValid = false
		result.ValidationErrors = append(result.ValidationErrors, fmt.Sprintf("failed to parse config values: %v", err))
		return result, fmt.Errorf("failed to parse config values from %s: %w", configValuesPath, err)
	}

	if kotsConfigValues == nil {
		result.IsValid = false
		result.ValidationErrors = append(result.ValidationErrors, "config values file is empty or invalid")
		return result, fmt.Errorf("config values file is empty or invalid: %s", configValuesPath)
	}

	// Convert to AppConfigValues
	appConfigValues := apitypes.ConvertToAppConfigValues(kotsConfigValues)

	result.ConfigValues = kotsConfigValues
	result.AppConfigValues = appConfigValues

	return result, nil
}

// FormatValidationErrors formats validation errors for display
func FormatValidationErrors(errors []string) string {
	if len(errors) == 0 {
		return ""
	}

	result := "Validation failed:\n"
	for _, err := range errors {
		result += fmt.Sprintf("  - %s\n", err)
	}
	return result
}
