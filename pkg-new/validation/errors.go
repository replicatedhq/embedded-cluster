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

// NewCurrentReleaseFailedError creates a ValidationError indicating that the current
// installed release is required and it's in a failed state
func NewCurrentReleaseFailedError(currentVersion string, targetVersion string) *ValidationError {
	return &ValidationError{
		Message: fmt.Sprintf("this upgrade requires the current installed version %s to be installed successfully and its current status is failed. Please this version is installed correctly before upgrading to %s",
			currentVersion, targetVersion),
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

// NewK8sVersionDowngrade creates a ValidationError indicating that the Kubernetes
// version upgrade downgrades the kubernetes version used, which is not supported
func NewK8sVersionDowngrade(currentVersion, targetVersion string) *ValidationError {
	return &ValidationError{
		Message: fmt.Sprintf("Kubernetes version downgrade detected: cannot downgrade from k8s %s to %s. Kubernetes downgrades are not supported",
			currentVersion, targetVersion),
	}
}
