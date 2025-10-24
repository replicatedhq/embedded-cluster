package cli

import (
	"context"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/pkg-new/replicatedapi"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
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

func newReplicatedAPIClient(license *kotsv1beta1.License, clusterID string) (replicatedapi.Client, error) {
	return replicatedapi.NewClient(
		replicatedAppURL(),
		license,
		release.GetReleaseData(),
		replicatedapi.WithClusterID(clusterID),
	)
}

func syncLicense(ctx context.Context, client replicatedapi.Client, license *kotsv1beta1.License) (*kotsv1beta1.License, []byte, error) {
	logrus.Debug("Syncing license")

	updatedLicense, licenseBytes, err := client.SyncLicense(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("get latest license: %w", err)
	}

	if updatedLicense.Spec.LicenseSequence != license.Spec.LicenseSequence {
		logrus.Debugf("License synced successfully (sequence %d -> %d)",
			license.Spec.LicenseSequence,
			updatedLicense.Spec.LicenseSequence)
	} else {
		logrus.Debug("License is already up to date")
	}

	return updatedLicense, licenseBytes, nil
}
