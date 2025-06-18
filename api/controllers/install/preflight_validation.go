package install

import (
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/types"
)

// ValidatePreflightStatus validates if infrastructure setup should proceed based on preflight status
func ValidatePreflightStatus(preflightStatus *types.HostPreflights, ignoreFailures bool) error {
	if preflightStatus == nil {
		return fmt.Errorf("preflight status not available")
	}

	// Check if preflights have any failures
	hasFailures := preflightStatus.Output != nil && len(preflightStatus.Output.Fail) > 0

	// If no failures, allow setup
	if !hasFailures {
		return nil
	}

	// Preflights failed - check if we can ignore them
	if !ignoreFailures {
		return fmt.Errorf("Preflight checks failed")
	}

	// Client wants to ignore failures - check if CLI flag allows it
	if !preflightStatus.AllowIgnoreHostPreflights {
		return fmt.Errorf("Cannot ignore preflight failures without --ignore-host-preflights flag")
	}

	// Client wants to ignore failures and CLI flag allows it
	return nil
}
