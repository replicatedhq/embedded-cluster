package helpers

import (
	"fmt"
)

func SnapshotsEnabled(licenseFile string) (bool, error) {
	if licenseFile == "" {
		return false, nil
	}

	lic, err := ParseLicense(licenseFile)
	if err != nil {
		return false, fmt.Errorf("failed to parse license: %w", err)
	}

	// if the license does not have snapshots enabled, return false
	if !lic.Spec.IsSnapshotSupported {
		return false, nil
	}

	return true, nil
}
