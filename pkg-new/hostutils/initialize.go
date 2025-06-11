package hostutils

import (
	"context"
	"fmt"
	"os"
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

func (h *HostUtils) ConfigureHost(ctx context.Context, rc runtimeconfig.RuntimeConfig, opts InitForInstallOptions) error {
	// update process env vars from the runtime config
	os.Setenv("KUBECONFIG", rc.PathToKubeConfig())
	os.Setenv("TMPDIR", rc.EmbeddedClusterTmpSubDir())

	// write the runtime config to disk
	if err := rc.WriteToDisk(); err != nil {
		return fmt.Errorf("unable to write runtime config to disk: %w", err)
	}

	// ensure correct permissions on the data directory
	if err := os.Chmod(rc.EmbeddedClusterHomeDirectory(), 0755); err != nil {
		// don't fail as there are cases where we can't change the permissions (bind mounts, selinux, etc...),
		// and we handle and surface those errors to the user later (host preflights, checking exec errors, etc...)
		h.logger.Debugf("unable to chmod embedded-cluster home dir: %s", err)
	}

	h.logger.Debugf("materializing files")
	if err := h.MaterializeFiles(rc, opts.AirgapBundle); err != nil {
		return fmt.Errorf("materialize files: %w", err)
	}

	if opts.LicenseFile != "" {
		h.logger.Debugf("copy license file to %s", rc.EmbeddedClusterHomeDirectory())
		if err := helpers.CopyFile(opts.LicenseFile, filepath.Join(rc.EmbeddedClusterHomeDirectory(), "license.yaml"), 0400); err != nil {
			// We have decided not to report this error
			h.logger.Warnf("copy license file to %s: %v", rc.EmbeddedClusterHomeDirectory(), err)
		}
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
