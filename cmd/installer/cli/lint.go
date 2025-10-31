package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/replicatedhq/embedded-cluster/pkg/lint"
	"github.com/spf13/cobra"
)

// LintCmd creates a hidden command for linting embedded cluster configuration files
func LintCmd(ctx context.Context) *cobra.Command {
	var verbose bool
	var outputFormat string

	cmd := &cobra.Command{
		Use:    "lint [flags] [file...]",
		Short:  "Lint embedded cluster configuration files",
		Hidden: true, // Hidden command as requested
		Long: `Lint embedded cluster configuration files to validate:
- YAML syntax (duplicate keys, unclosed quotes, invalid structure)
- Port specifications in unsupportedOverrides that are already supported
- Custom domains against the Replicated app's configured domains

Environment variables (optional for custom domain validation):
- REPLICATED_API_TOKEN: Authentication token for Replicated API
- REPLICATED_API_ORIGIN: API endpoint (e.g., https://api.replicated.com/vendor)
- REPLICATED_APP: Application ID or slug`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate output format
			if outputFormat != "text" && outputFormat != "json" {
				return fmt.Errorf("invalid output format %q: must be 'text' or 'json'", outputFormat)
			}

			// Get environment variables
			apiToken := os.Getenv("REPLICATED_API_TOKEN")
			apiOrigin := os.Getenv("REPLICATED_API_ORIGIN")
			appID := os.Getenv("REPLICATED_APP")

			// Create validator with verbose flag
			validator := lint.NewValidator(apiToken, apiOrigin, appID)
			validator.SetVerbose(verbose && outputFormat != "json") // Disable verbose in JSON mode

			// For JSON output, accumulate all results
			var jsonResults lint.JSONOutput
			hasErrors := false

			// Validate each file
			for _, file := range args {
				if outputFormat != "json" {
					fmt.Printf("Linting %s...\n", file)
				}

				result, err := validator.ValidateFile(file)
				if err != nil {
					if outputFormat == "json" {
						// Add as a file with error
						jsonResults.Files = append(jsonResults.Files, lint.FileResult{
							Path:   file,
							Valid:  false,
							Errors: []lint.ValidationIssue{{Field: "", Message: err.Error()}},
						})
					} else {
						fmt.Fprintf(os.Stderr, "ERROR: Failed to validate %s: %v\n", file, err)
					}
					hasErrors = true
					continue
				}

				if outputFormat == "json" {
					// Add to JSON results
					jsonResults.Files = append(jsonResults.Files, result.ToJSON(file))
					if len(result.Errors) > 0 {
						hasErrors = true
					}
				} else {
					// Display warnings
					for _, warning := range result.Warnings {
						fmt.Fprintf(os.Stderr, "WARNING: %s: %s\n", file, warning)
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
			}

			// Output JSON if requested
			if outputFormat == "json" {
				output, err := json.MarshalIndent(jsonResults, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal JSON output: %w", err)
				}
				fmt.Println(string(output))
			}

			// Only fail if there are errors (not warnings)
			if hasErrors {
				return fmt.Errorf("validation failed with errors")
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output showing API endpoints and detailed information")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text or json")

	return cmd
}
