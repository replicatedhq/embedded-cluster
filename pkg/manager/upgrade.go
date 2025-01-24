package manager

import (
	"context"

	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers/systemd"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
)

type UpgradeOptions struct {
	AppSlug         string `json:"appSlug"`
	VersionLabel    string `json:"versionLabel"`
	LicenseID       string `json:"licenseID"`
	LicenseEndpoint string `json:"licenseEndpoint"`
}

func Upgrade(ctx context.Context, opts UpgradeOptions) error {
	// path to the manager binary on the host
	binPath := runtimeconfig.PathToEmbeddedClusterBinary("manager")

	// TODO (@salah): airgap
	err := DownloadBinaryOnline(ctx, binPath, opts.LicenseID, opts.LicenseEndpoint, opts.VersionLabel)
	if err != nil {
		return errors.Wrap(err, "download manager binary")
	}

	// this is hacky but app slug is what determines the service name
	SetServiceName(opts.AppSlug)

	if err := systemd.Restart(ctx, UnitName()); err != nil {
		return errors.Wrap(err, "restart manager service")
	}

	return nil
}
