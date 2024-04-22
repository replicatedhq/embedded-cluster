package helpers

import (
	"fmt"

	"github.com/replicatedhq/embedded-cluster/pkg/release"
)

func SnapshotsEnabled(licenseFile string) (bool, error) {
	lic, err := ParseLicense(licenseFile)
	if err != nil {
		return false, fmt.Errorf("failed to parse license: %w", err)
	}

	// if the license does not have snapshots enabled, return false
	if !lic.Spec.IsSnapshotSupported {
		return false, nil
	}

	rel, err := release.GetEmbeddedClusterConfig()
	if err != nil {
		return false, fmt.Errorf("failed to get embedded cluster config: %w", err)
	}

	// if the Velero addon is not enabled, return false
	if rel == nil || rel.Spec.Extensions.Builtin == nil || rel.Spec.Extensions.Builtin.Velero == nil || !rel.Spec.Extensions.Builtin.Velero.Enabled {
		return false, nil
	}

	return true, nil
}
