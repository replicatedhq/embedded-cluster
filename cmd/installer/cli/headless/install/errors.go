package install

import (
	"fmt"
	"strings"

	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
)

// formatAPIError formats an APIError for display to the user
func formatAPIError(apiErr *apitypes.APIError) string {
	if apiErr == nil {
		return ""
	}

	var buf strings.Builder

	// Write the main error message if present
	if apiErr.Message != "" {
		buf.WriteString(apiErr.Message)
	}

	// Write field errors
	if len(apiErr.Errors) > 0 {
		if buf.Len() > 0 {
			buf.WriteString(":\n")
		}
		for _, fieldErr := range apiErr.Errors {
			if fieldErr.Field != "" {
				fmt.Fprintf(&buf, "  - Field '%s': %s\n", fieldErr.Field, fieldErr.Message)
			} else {
				fmt.Fprintf(&buf, "  - %s\n", fieldErr.Message)
			}
		}
	}

	result := buf.String()
	// Remove trailing newline
	return strings.TrimSuffix(result, "\n")
}
