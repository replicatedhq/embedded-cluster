package host

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/sirupsen/logrus"
)

type InitializeForInstallOptions struct {
	LicenseFile  string
	AirgapBundle string
	DataDir      string
	PodCIDR      string
	ServiceCIDR  string
}

func InitializeForInstall(ctx context.Context, opts InitializeForInstallOptions) error {
	if err := MaterializeFiles(opts.AirgapBundle); err != nil {
		return fmt.Errorf("unable to materialize files: %w", err)
	}

	logrus.Debugf("copy license file to %s", opts.DataDir)
	if err := helpers.CopyFile(opts.LicenseFile, filepath.Join(opts.DataDir, "license.yaml"), 0400); err != nil {
		// We have decided not to report this error
		logrus.Warnf("Unable to copy license file to %s: %v", opts.DataDir, err)
	}

	logrus.Debugf("configuring sysctl")
	if err := ConfigureSysctl(); err != nil {
		logrus.Debugf("unable to configure sysctl: %v", err)
	}

	logrus.Debugf("configuring kernel modules")
	if err := ConfigureKernelModules(); err != nil {
		logrus.Debugf("unable to configure kernel modules: %v", err)
	}

	logrus.Debugf("configuring network manager")
	if err := ConfigureNetworkManager(ctx); err != nil {
		return fmt.Errorf("unable to configure network manager: %w", err)
	}

	logrus.Debugf("configuring firewalld")
	if err := ConfigureFirewalld(ctx, opts.PodCIDR, opts.ServiceCIDR); err != nil {
		logrus.Debugf("unable to configure firewalld: %v", err)
	}

	return nil
}
