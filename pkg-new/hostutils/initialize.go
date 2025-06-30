package hostutils

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
)

type InitForInstallOptions struct {
	License      []byte
	AirgapBundle string
}

func (h *HostUtils) ConfigureHost(ctx context.Context, rc runtimeconfig.RuntimeConfig, opts InitForInstallOptions) error {
	h.logger.Debugf("writing runtime config to disk")
	if err := rc.WriteToDisk(); err != nil {
		return fmt.Errorf("write runtime config to disk: %w", err)
	}

	h.logger.Debugf("setting permissions on %s", rc.EmbeddedClusterHomeDirectory())
	if err := os.Chmod(rc.EmbeddedClusterHomeDirectory(), 0755); err != nil {
		// don't fail as there are cases where we can't change the permissions (bind mounts, selinux, etc...),
		// and we handle and surface those errors to the user later (host preflights, checking exec errors, etc...)
		h.logger.Debugf("unable to chmod embedded-cluster home dir: %s", err)
	}

	h.logger.Debugf("materializing files")
	if err := h.MaterializeFiles(rc, opts.AirgapBundle); err != nil {
		return fmt.Errorf("materialize files: %w", err)
	}

	if opts.License != nil {
		h.logger.Debugf("write license file to %s", rc.EmbeddedClusterHomeDirectory())
		if err := os.WriteFile(filepath.Join(rc.EmbeddedClusterHomeDirectory(), "license.yaml"), opts.License, 0400); err != nil {
			h.logger.Warnf("unable to write license file to %s: %v", rc.EmbeddedClusterHomeDirectory(), err)
		}
	}

	h.logger.Debugln("checking for semanage binary in $PATH")
	if _, err := exec.LookPath("semanage"); err != nil {
		h.logger.Debugln("semanage not found")
	} else {

		// Set selinux fcontext for embedded-cluster binary directory to bin_t
		out, err := exec.Command("semanage", "fcontext", "-a", "-s", "system_u", "-t", "bin_t", rc.EmbeddedClusterBinsSubDir()+"(/.*)?").CombinedOutput()
		if err != nil {
			h.logger.Debugf("unable to set contexts on binary directory: %v", err)
			h.logger.Debugln(out)
		}

		// Relabel whole embedded-cluster data directory since it's created with unconfined_u
		out, err = exec.Command("restorecon", "-RvF", rc.EmbeddedClusterHomeDirectory()).CombinedOutput()
		if err != nil {
			h.logger.Debugf("unable to run restorecon: %v", err)
			h.logger.Debugln(out)
		}
	}

	h.logger.Debugf("configuring sysctl")
	if err := h.ConfigureSysctl(); err != nil {
		h.logger.Debugf("unable to configure sysctl: %v", err)
	}

	h.logger.Debugf("configuring kernel modules")
	if err := h.ConfigureKernelModules(); err != nil {
		h.logger.Debugf("unable to configure kernel modules: %v", err)
	}

	h.logger.Debugf("configuring network manager")
	if err := h.ConfigureNetworkManager(ctx, rc); err != nil {
		return fmt.Errorf("configure network manager: %w", err)
	}

	h.logger.Debugf("configuring firewalld")
	if err := h.ConfigureFirewalld(ctx, rc.PodCIDR(), rc.ServiceCIDR()); err != nil {
		h.logger.Debugf("unable to configure firewalld: %v", err)
	}

	return nil
}
