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

func newReplicatedAPIClient(license *licensewrapper.LicenseWrapper, clusterID string) (replicatedapi.Client, error) {
	// Pass the wrapper directly - the API client now handles both v1beta1 and v1beta2
	return replicatedapi.NewClient(
		replicatedAppURL(),
		license,
		release.GetReleaseData(),
		replicatedapi.WithClusterID(clusterID),
	)
}

func syncLicense(ctx context.Context, client replicatedapi.Client, license *licensewrapper.LicenseWrapper) (*licensewrapper.LicenseWrapper, []byte, error) {
	logrus.Debug("Syncing license")

	updatedLicense, licenseBytes, err := client.SyncLicense(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("get latest license: %w", err)
	}

	if license != nil {
		oldSeq := license.GetLicenseSequence()
		newSeq := updatedLicense.GetLicenseSequence()
		if newSeq != oldSeq {
			logrus.Debugf("License synced successfully (sequence %d -> %d)", oldSeq, newSeq)
		} else {
			logrus.Debug("License is already up to date")
		}
	}

	// Return wrapper directly - already wrapped by SyncLicense
	return updatedLicense, licenseBytes, nil
}
