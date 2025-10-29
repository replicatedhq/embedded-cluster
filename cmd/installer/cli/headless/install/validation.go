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

// ValidateHeadlessInstallFlags validates that all required flags are present for headless
// installation and that the config values file exists and is readable.
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

	if flags.ConfigValues != "" {
		// Check if file exists
		if _, err := os.Stat(flags.ConfigValues); err != nil {
			if os.IsNotExist(err) {
				errors = append(errors, fmt.Sprintf("config values file not found: %s", flags.ConfigValues))
			} else {
				errors = append(errors, fmt.Sprintf("failed to access config values file: %v", err))
			}
		} else {
			// Parse the config values file
			_, err := helpers.ParseConfigValues(flags.ConfigValues)
			if err != nil {
				errors = append(errors, fmt.Sprintf("failed to parse config values file: %v", err))
			}
		}
	}

	return errors
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
