package cli

import (
	"context"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/pkg-new/replicatedapi"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/kotskinds/pkg/licensewrapper"
	"github.com/sirupsen/logrus"
)

func replicatedAppURL() string {
	domains := getDomains()
	return netutils.MaybeAddHTTPS(domains.ReplicatedAppDomain)
}

func proxyRegistryURL() string {
	domains := getDomains()
	return netutils.MaybeAddHTTPS(domains.ProxyRegistryDomain)
}

func newReplicatedAPIClient(license licensewrapper.LicenseWrapper, clusterID string) (replicatedapi.Client, error) {
	// Extract the underlying v1beta1 license for the API client
	// The API client only supports v1beta1 licenses
	// For v1beta2 licenses, we use the V1 field which contains the converted v1beta1 representation
	underlyingLicense := license.V1

	return replicatedapi.NewClient(
		replicatedAppURL(),
		underlyingLicense,
		release.GetReleaseData(),
		replicatedapi.WithClusterID(clusterID),
	)
}

func syncLicense(ctx context.Context, client replicatedapi.Client, license licensewrapper.LicenseWrapper) (licensewrapper.LicenseWrapper, []byte, error) {
	logrus.Debug("Syncing license")

	updatedLicense, licenseBytes, err := client.SyncLicense(ctx)
	if err != nil {
		return licensewrapper.LicenseWrapper{}, nil, fmt.Errorf("get latest license: %w", err)
	}

	oldSeq := license.GetLicenseSequence()
	newSeq := updatedLicense.Spec.LicenseSequence
	if newSeq != oldSeq {
		logrus.Debugf("License synced successfully (sequence %d -> %d)", oldSeq, newSeq)
	} else {
		logrus.Debug("License is already up to date")
	}

	// Wrap the updated license - it comes back as v1beta1
	return licensewrapper.LicenseWrapper{V1: updatedLicense}, licenseBytes, nil
}
