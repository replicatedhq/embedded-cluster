package validation

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/replicatedhq/embedded-cluster/pkg-new/replicatedapi"
	"github.com/replicatedhq/embedded-cluster/pkg/airgap"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
)

// k8sBuildRegex holds the regex pattern we use for the build portion of our EC version - i.e. 2.11.3+k8s-1.33
var k8sBuildRegex = regexp.MustCompile(`k8s-(\d+\.\d+)`)

// UpgradableOptions holds configuration for validating release deployability
type UpgradableOptions struct {
	IsAirgap           bool
	CurrentAppVersion  string
	CurrentAppSequence int64
	CurrentECVersion   string
	TargetAppVersion   string
	TargetAppSequence  int64
	TargetECVersion    string
	License            *kotsv1beta1.License
	AirgapMetadata     *airgap.AirgapMetadata
	ReplicatedAPI      replicatedapi.Client
	ChannelID          string
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

	// Check 4: K8s minor version skip
	if err := validateK8sVersion(opts); err != nil {
		return err
	}

	return nil
}

// validateRequiredReleases checks if any required releases are being skipped
func validateRequiredReleases(ctx context.Context, opts UpgradableOptions) error {
	requiredVersions := []string{}
	if opts.IsAirgap {
		// For airgap, check RequiredReleases field in the airgap metadata
		if opts.AirgapMetadata == nil || opts.AirgapMetadata.AirgapInfo == nil {
			return fmt.Errorf("airgap metadata is required for airgap validation")
		}

		requiredReleases := opts.AirgapMetadata.AirgapInfo.Spec.RequiredReleases
		if len(requiredReleases) > 0 {
			// Extract version labels from required releases
			for _, release := range requiredReleases {
				requiredVersions = append(requiredVersions, release.VersionLabel)
			}
		}
	} else {
		// For online, call the API
		if opts.ReplicatedAPI == nil {
			return fmt.Errorf("replicated API client is required for online validation")
		}

		if opts.ChannelID == "" {
			return fmt.Errorf("channel ID is required for online validation")
		}

		options := &replicatedapi.PendingReleasesOptions{
			IsSemverSupported: opts.License.Spec.IsSemverRequired,
			SortOrder:         replicatedapi.SortOrderAscending,
		}
		// Get pending releases from the current app seqeuence in asceding order
		pendingReleases, err := opts.ReplicatedAPI.GetPendingReleases(ctx, opts.ChannelID, opts.CurrentAppSequence, options)
		if err != nil {
			return fmt.Errorf("failed to fetch pending releases: %w", err)
		}

		// Find required releases between current and target sequence
		for _, release := range pendingReleases.ChannelReleases {
			// Releases are in asceding order, we've hit the target sequence so we can break
			if release.ChannelSequence == opts.TargetAppSequence {
				break
			}
			if release.IsRequired {
				requiredVersions = append(requiredVersions, release.VersionLabel)
			}
		}
	}

	if len(requiredVersions) > 0 {
		return fmt.Errorf("this upgrade requires installing intermediate version(s) first: %s. Please go through this upgrade path before upgrading to %s",
			strings.Join(requiredVersions, ", "), opts.TargetAppVersion)
	}

	return nil
}

// validateAppVersionDowngrade checks if the target app version is older than the current version
func validateAppVersionDowngrade(opts UpgradableOptions) error {
	// If using semver than compare using it
	if opts.License.Spec.IsSemverRequired {
		currentVer, err := semver.NewVersion(opts.CurrentAppVersion)
		if err != nil {
			return err
		}
		targetVer, err := semver.NewVersion(opts.TargetAppVersion)
		if err != nil {
			return err
		}
		if targetVer.LessThan(currentVer) {
			return fmt.Errorf("downgrade detected: cannot upgrade from app version %s to older version %s", opts.CurrentAppVersion, opts.TargetAppVersion)
		}
		return nil
	}

	// Use app sequence as fallback
	if opts.CurrentAppSequence > opts.TargetAppSequence {
		return fmt.Errorf("downgrade detected: cannot upgrade from app version %s to older version %s", opts.CurrentAppVersion, opts.TargetAppVersion)
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
		return fmt.Errorf("downgrade detected: cannot upgrade from Embedded Cluster version %s to older version %s", opts.CurrentECVersion, opts.TargetECVersion)
	}

	return nil
}

// validateK8sVersion checks if the K8s version skips a minor version
func validateK8sVersion(opts UpgradableOptions) error {
	// Parse the EC version format to extract K8s version: "2.12.0+k8s-1.33-*"
	currentK8s, err := getK8sVersion(opts.CurrentECVersion)
	if err != nil {
		return fmt.Errorf("failed to extract k8s version from curretn version %s: %w", opts.CurrentECVersion, err)
	}

	targetK8s, err := getK8sVersion(opts.TargetECVersion)
	if err != nil {
		return fmt.Errorf("failed to extract k8s version from target version %s: %w", opts.CurrentECVersion, err)
	}

	// Check if minor version is being skipped
	if targetK8s.Minor() > currentK8s.Minor()+1 {
		return fmt.Errorf("Kubernetes version skip detected: cannot upgrade from k8s %d.%d to %d.%d. Kubernetes only supports upgrading by one minor version at a time",
			currentK8s.Major(), currentK8s.Minor(), targetK8s.Major(), targetK8s.Minor())
	}

	return nil
}

// getK8sVersion parses an EC version string in the format "2.12.0+k8s-1.33-*"
// and returns the K8s version
func getK8sVersion(version string) (*semver.Version, error) {
	// Parse the EC version format to extract K8s version: "2.12.0+k8s-1.33-*"
	ecVersion, err := semver.NewVersion(version)
	if err != nil {
		return nil, fmt.Errorf("failed to parse EC version %s: %w", ecVersion, err)
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
