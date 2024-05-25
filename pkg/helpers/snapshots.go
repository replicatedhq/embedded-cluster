package helpers

import (
	"fmt"
)

func DisasterRecoveryEnabled(licenseFile string) (bool, error) {
	if licenseFile == "" {
		return false, nil
	}

	lic, err := ParseLicense(licenseFile)
	if err != nil {
		return false, fmt.Errorf("failed to parse license: %w", err)
	}

	// if the license does not have disaster recovery enabled, return false
	if !lic.Spec.IsDisasterRecoverySupported {
		return false, nil
	}

	return true, nil
}
