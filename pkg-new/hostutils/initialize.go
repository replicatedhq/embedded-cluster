package hostutils

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
)

type InitForInstallOptions struct {
	LicenseFile  string
	AirgapBundle string
	PodCIDR      string
	ServiceCIDR  string
}

func (h *HostUtils) ConfigureForInstall(ctx context.Context, rc runtimeconfig.RuntimeConfig, opts InitForInstallOptions) error {
	h.logger.Debugf("materializing files")
	if err := h.MaterializeFiles(rc, opts.AirgapBundle); err != nil {
		return fmt.Errorf("materialize files: %w", err)
	}

	h.logger.Debugf("copy license file to %s", rc.EmbeddedClusterHomeDirectory())
	if err := helpers.CopyFile(opts.LicenseFile, filepath.Join(rc.EmbeddedClusterHomeDirectory(), "license.yaml"), 0400); err != nil {
		// We have decided not to report this error
		h.logger.Warnf("copy license file to %s: %v", rc.EmbeddedClusterHomeDirectory(), err)
	}

	h.logger.Debugf("configuring sysctl")
	if err := h.ConfigureSysctl(); err != nil {
		h.logger.Debugf("configure sysctl: %v", err)
	}

	h.logger.Debugf("configuring kernel modules")
	if err := h.ConfigureKernelModules(); err != nil {
		h.logger.Debugf("configure kernel modules: %v", err)
	}

	h.logger.Debugf("configuring network manager")
	if err := h.ConfigureNetworkManager(ctx, rc); err != nil {
		return fmt.Errorf("configure network manager: %w", err)
	}

	h.logger.Debugf("configuring firewalld")
	if err := h.ConfigureFirewalld(ctx, opts.PodCIDR, opts.ServiceCIDR); err != nil {
		h.logger.Debugf("configure firewalld: %v", err)
	}

	return nil
}
