package helpers

import (
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
)

func DisasterRecoveryEnabled(license *kotsv1beta1.License) (bool, error) {
	if license == nil {
		return false, nil
	}

	// if the license does not have disaster recovery enabled, return false
	if !license.Spec.IsDisasterRecoverySupported {
		return false, nil
	}

	return true, nil
}
