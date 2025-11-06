package validation

import (
	"context"
	"fmt"
	"regexp"
	"strconv"

	"github.com/Masterminds/semver/v3"
	"github.com/replicatedhq/embedded-cluster/pkg-new/replicatedapi"
	"github.com/replicatedhq/embedded-cluster/pkg/airgap"
  "github.com/replicatedhq/kotskinds/pkg/licensewrapper"
)

// k8sBuildRegex holds the regex pattern we use for the build portion of our EC version - i.e. 2.11.3+k8s-1.33
var k8sBuildRegex = regexp.MustCompile(`k8s-(\d+\.\d+)`)

// UpgradableOptions holds configuration for validating release deployability
type UpgradableOptions struct {
	CurrentAppVersion  string
	CurrentAppSequence int64
	CurrentECVersion   string
	TargetAppVersion   string
	TargetAppSequence  int64
	TargetECVersion    string
	License            *licensewrapper.LicenseWrapper
	requiredReleases   []string
}

// WithAirgapRequiredReleases extracts the required releases from airgap metadata to be used for validation
func (opts *UpgradableOptions) WithAirgapRequiredReleases(metadata *airgap.AirgapMetadata) error {
	if metadata == nil || metadata.AirgapInfo == nil {
		return fmt.Errorf("airgap metadata is required for validating airgap required releases")
	}

	// RequiredReleases are in descending order, we need to iterate through the required releases of the target release until we find releases lower than the current installed release
	requiredReleases := metadata.AirgapInfo.Spec.RequiredReleases
	if len(requiredReleases) > 0 {
		// Extract version labels from required releases
		for _, release := range requiredReleases {
			sequence, err := strconv.ParseInt(release.UpdateCursor, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse airgap spec required release update cursor %s: %w", release.UpdateCursor, err)
			}
			// We've hit a release that is less than or equal to the current installed release, we can stop
			if sequence <= opts.CurrentAppSequence {
				return nil
			}
			opts.requiredReleases = append(opts.requiredReleases, release.VersionLabel)
		}
	}
	return nil
}

// WithOnlineRequiredReleases fetches the pending releases from the current app sequence and extracts the required releases until the target app sequence
func (opts *UpgradableOptions) WithOnlineRequiredReleases(ctx context.Context, replAPIClient replicatedapi.Client) error {
	if opts.License == nil {
		return fmt.Errorf("license is required to check online upgrade required releases")
	}
	options := &replicatedapi.PendingReleasesOptions{
		IsSemverSupported: opts.License.IsSemverRequired(),
		SortOrder:         replicatedapi.SortOrderAscending,
	}
	// Get pending releases from the current app sequence in asceding order
	pendingReleases, err := replAPIClient.GetPendingReleases(ctx, opts.License.GetChannelID(), opts.CurrentAppSequence, options)
	if err != nil {
		return fmt.Errorf("failed to get pending releases while checking required releases for upgrade: %w", err)
	}
	if pendingReleases != nil {
		opts.handlePendingReleases(pendingReleases.ChannelReleases)
	}
	return nil
}

// handlePendingReleases processes the pending releases to extract required releases between current and target sequences
func (opts *UpgradableOptions) handlePendingReleases(pendingReleases []replicatedapi.ChannelRelease) {
	// Find required releases between current and target sequence
	for _, release := range pendingReleases {
		// Releases are in asceding order, we've hit the target sequence so we can break
		if release.ChannelSequence == opts.TargetAppSequence {
			break
		}
		if release.IsRequired {
			opts.requiredReleases = append(opts.requiredReleases, release.VersionLabel)
		}
	}
}

// ValidateIsReleaseUpgradable validates that a target release can be safely deployed
func ValidateIsReleaseUpgradable(ctx context.Context, opts UpgradableOptions) error {
	// Check 1: App version downgrade
	if err := validateAppVersionDowngrade(opts); err != nil {
		return err
	}

	// Check 2: Required releases
	if err := validateRequiredReleases(ctx, opts); err != nil {
		return err
	}

	// Check 3: EC version downgrade
	if err := validateECVersionDowngrade(opts); err != nil {
		return err
	}

	// Check 4: K8s minor version skip and downgrade
	if err := validateK8sVersion(opts); err != nil {
		return err
	}

	return nil
}

// validateRequiredReleases checks if any required releases are being skipped
func validateRequiredReleases(ctx context.Context, opts UpgradableOptions) error {
	if len(opts.requiredReleases) > 0 {
		return NewRequiredReleasesError(opts.requiredReleases, opts.TargetAppVersion)
	}

	return nil
}

// validateAppVersionDowngrade checks if the target app version is older than the current version
func validateAppVersionDowngrade(opts UpgradableOptions) error {
	// If using semver than compare using it
	if opts.License.IsSemverRequired() {
		currentVer, err := semver.NewVersion(opts.CurrentAppVersion)
		if err != nil {
			return fmt.Errorf("failed to parse current app version %s: %w", opts.CurrentAppVersion, err)
		}
		targetVer, err := semver.NewVersion(opts.TargetAppVersion)
		if err != nil {
			return fmt.Errorf("failed to parse target app version %s: %w", opts.TargetAppVersion, err)
		}
		if targetVer.LessThan(currentVer) {
			return NewAppVersionDowngradeError(opts.CurrentAppVersion, opts.TargetAppVersion)
		}
		return nil
	}

	// Use app sequence as fallback
	if opts.CurrentAppSequence > opts.TargetAppSequence {
		return NewAppVersionDowngradeError(opts.CurrentAppVersion, opts.TargetAppVersion)
	}

	return nil
}

// validateECVersionDowngrade checks if the target EC version is older than the current version
func validateECVersionDowngrade(opts UpgradableOptions) error {
	current, err := semver.NewVersion(opts.CurrentECVersion)
	if err != nil {
		return fmt.Errorf("failed to parse current EC version %s: %w", opts.CurrentECVersion, err)
	}

	target, err := semver.NewVersion(opts.TargetECVersion)
	if err != nil {
		return fmt.Errorf("failed to parse target EC version %s: %w", opts.TargetECVersion, err)
	}

	if target.LessThan(current) {
		return NewECVersionDowngradeError(opts.CurrentECVersion, opts.TargetECVersion)
	}

	return nil
}

// validateK8sVersion checks if the K8s version skips a minor version or downgrades
func validateK8sVersion(opts UpgradableOptions) error {
	// Parse the EC version format to extract K8s version: "2.12.0+k8s-1.33-*"
	currentK8s, err := getK8sVersion(opts.CurrentECVersion)
	if err != nil {
		return fmt.Errorf("failed to extract k8s version from current version %s: %w", opts.CurrentECVersion, err)
	}

	targetK8s, err := getK8sVersion(opts.TargetECVersion)
	if err != nil {
		return fmt.Errorf("failed to extract k8s version from target version %s: %w", opts.TargetECVersion, err)
	}

	// Check if minor version is being skipped
	if targetK8s.Minor() > currentK8s.Minor()+1 {
		return NewK8sVersionSkipError(
			currentK8s.String(),
			targetK8s.String(),
		)
	}

	// Check if K8s version is being downgraded
	if targetK8s.LessThan(currentK8s) {
		return NewK8sVersionDowngrade(
			currentK8s.String(),
			targetK8s.String(),
		)
	}

	return nil
}

// getK8sVersion parses an EC version string in the format "2.12.0+k8s-1.33-*"
// and returns the K8s version
func getK8sVersion(version string) (*semver.Version, error) {
	// Parse the EC version format to extract K8s version: "2.12.0+k8s-1.33-*"
	ecVersion, err := semver.NewVersion(version)
	if err != nil {
		return nil, fmt.Errorf("failed to parse EC version %s: %w", version, err)
	}

	// Parse the build portion of the semver version +k8s-<version> and extract it
	matches := k8sBuildRegex.FindStringSubmatch(ecVersion.Metadata())
	if len(matches) != 2 {
		return nil, fmt.Errorf("invalid EC version format: expected 'X.Y.Z+k8s-A.B-*', got %s", version)
	}

	// Parse k8s version
	k8sVersion, err := semver.NewVersion(matches[1])
	if err != nil {
		return nil, fmt.Errorf("failed to parse k8s version %s: %w", k8sVersion, err)
	}

	return k8sVersion, nil
}
