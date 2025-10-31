package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/replicatedhq/embedded-cluster/pkg/lint"
	"github.com/spf13/cobra"
)

// LintCmd creates a hidden command for linting embedded cluster configuration files
func LintCmd(ctx context.Context) *cobra.Command {
	var verbose bool

	cmd := &cobra.Command{
		Use:    "lint [flags] [file...]",
		Short:  "Lint embedded cluster configuration files",
		Hidden: true, // Hidden command as requested
		Long: `Lint embedded cluster configuration files to validate:
- Port specifications in unsupportedOverrides that are already supported
- Custom domains against the Replicated app's configured domains

Environment variables required:
- REPLICATED_API_TOKEN: Authentication token for Replicated API
- REPLICATED_API_ORIGIN: API endpoint (e.g., https://api.replicated.com/vendor)
- REPLICATED_APP: Application ID or slug`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get environment variables
			apiToken := os.Getenv("REPLICATED_API_TOKEN")
			apiOrigin := os.Getenv("REPLICATED_API_ORIGIN")
			appID := os.Getenv("REPLICATED_APP")

			// Create validator with verbose flag
			validator := lint.NewValidator(apiToken, apiOrigin, appID)
			validator.SetVerbose(verbose)

			// Track if any validation failed (only errors cause failure, not warnings)
			hasErrors := false
			totalWarnings := 0

			// Validate each file
			for _, file := range args {
				fmt.Printf("Linting %s...\n", file)

				result, err := validator.ValidateFile(file)
				if err != nil {
					fmt.Fprintf(os.Stderr, "ERROR: Failed to validate %s: %v\n", file, err)
					hasErrors = true
					continue
				}

				// Display warnings
				for _, warning := range result.Warnings {
					fmt.Fprintf(os.Stderr, "WARNING: %s: %s\n", file, warning)
					totalWarnings++
				}

				// Display errors
				for _, validationErr := range result.Errors {
					fmt.Fprintf(os.Stderr, "ERROR: %s: %s\n", file, validationErr)
					hasErrors = true
				}

				// Display result
				if len(result.Errors) == 0 && len(result.Warnings) == 0 {
					fmt.Printf("✓ %s passed validation\n", file)
				} else if len(result.Errors) == 0 && len(result.Warnings) > 0 {
					fmt.Printf("⚠ %s passed with %d warning(s)\n", file, len(result.Warnings))
				} else {
					fmt.Printf("✗ %s failed validation\n", file)
				}
			}

			// Only fail if there are errors (not warnings)
			if hasErrors {
				return fmt.Errorf("validation failed with errors")
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output showing API endpoints and detailed information")

	return cmd
}
