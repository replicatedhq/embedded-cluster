package validation

import (
	"fmt"
	"strings"
)

// ValidationError represents a validation failure that prevents an upgrade.
// These are expected errors that indicate the upgrade cannot proceed due to
// business rules (e.g., version downgrades, required releases).
// Internal/system errors (API failures, parsing errors) should NOT use this type.
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// NewRequiredReleasesError creates a ValidationError indicating that intermediate
// required releases must be installed before upgrading to the target version
func NewRequiredReleasesError(requiredVersions []string, targetVersion string) *ValidationError {
	return &ValidationError{
		Message: fmt.Sprintf("this upgrade requires installing intermediate version(s) first: %s. Please go through this upgrade path before upgrading to %s",
			strings.Join(requiredVersions, ", "), targetVersion),
	}
}

// NewAppVersionDowngradeError creates a ValidationError indicating that the target
// app version is older than the current version
func NewAppVersionDowngradeError(currentVersion, targetVersion string) *ValidationError {
	return &ValidationError{
		Message: fmt.Sprintf("downgrade detected: cannot upgrade from app version %s to older version %s", currentVersion, targetVersion),
	}
}

// NewECVersionDowngradeError creates a ValidationError indicating that the target
// Embedded Cluster version is older than the current version
func NewECVersionDowngradeError(currentVersion, targetVersion string) *ValidationError {
	return &ValidationError{
		Message: fmt.Sprintf("downgrade detected: cannot upgrade from Embedded Cluster version %s to older version %s", currentVersion, targetVersion),
	}
}

// NewK8sVersionSkipError creates a ValidationError indicating that the Kubernetes
// version upgrade skips a minor version, which is not supported by Kubernetes
func NewK8sVersionSkipError(currentVersion, targetVersion string) *ValidationError {
	return &ValidationError{
		Message: fmt.Sprintf("Kubernetes version skip detected: cannot upgrade from k8s %s to %s. Kubernetes only supports upgrading by one minor version at a time",
			currentVersion, targetVersion),
	}
}
