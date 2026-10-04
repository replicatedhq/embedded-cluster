package hostutils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/opencontainers/selinux/go-selinux"
	"github.com/replicatedhq/embedded-cluster/cmd/installer/goods"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
)

// selinuxModuleName is the name of our policy module, as declared by
// policy_module() in selinux/ec.te. semodule identifies it by this name.
const selinuxModuleName = "ec"

// ConfigureSELinux turns on container labeling, loads our policy module, and
// labels the data directory. Does nothing when selinux is not enabled.
//
// Failures are logged, not returned: a missing selinux tool should not fail an
// install. The selinux host preflights report it instead.
func (h *HostUtils) ConfigureSELinux(rc runtimeconfig.RuntimeConfig) error {
	if !selinux.GetEnabled() {
		h.logger.Debugln("selinux is not enabled, skipping selinux configuration")
		return nil
	}

	h.logger.Debugln("configuring selinux")

	if err := h.configureContainerdForSELinux(); err != nil {
		h.logger.Debugf("unable to configure containerd for selinux: %v", err)
	}

	if err := h.installSELinuxModule(rc); err != nil {
		// The module defines the labels, so relabeling without it would apply
		// the base policy's instead, undoing what a previous release set.
		h.logger.Debugf("unable to install selinux policy module: %v", err)
		return nil
	}

	if err := h.labelCustomDataDir(rc); err != nil {
		h.logger.Debugf("unable to label custom data directory: %v", err)
	}

	if err := h.relabelDataDir(rc); err != nil {
		h.logger.Debugf("unable to relabel the data directory: %v", err)
	}

	return nil
}

// installSELinuxModule writes the embedded policy module out and loads it.
func (h *HostUtils) installSELinuxModule(rc runtimeconfig.RuntimeConfig) error {
	pp, ok := goods.SELinuxPolicy()
	if !ok {
		return fmt.Errorf("no selinux policy module in this build")
	}

	if _, err := exec.LookPath("semodule"); err != nil {
		return fmt.Errorf("semodule not found in $PATH")
	}

	path := filepath.Join(rc.EmbeddedClusterTmpSubDir(), selinuxModuleName+".pp")
	if err := os.WriteFile(path, pp, 0600); err != nil {
		return fmt.Errorf("write policy module: %w", err)
	}
	defer os.Remove(path)

	// Reinstalls in place if already present, so this is safe on upgrade.
	if out, err := exec.Command("semodule", "-i", path).CombinedOutput(); err != nil {
		// Usually means container-selinux is absent, so the types the module
		// requires do not exist.
		return fmt.Errorf("semodule -i: %w: %s", err, string(out))
	}

	return nil
}

// labelCustomDataDir tells selinux to label a --data-dir directory the same as
// the default one. Our labels are compiled into the module and name only the
// default path, and generating policy on the host would need a compiler we
// cannot expect to be installed. Does nothing for the default path.
func (h *HostUtils) labelCustomDataDir(rc runtimeconfig.RuntimeConfig) error {
	dataDir := rc.EmbeddedClusterHomeDirectory()
	if dataDir == ecv1beta1.DefaultDataDir {
		return nil
	}

	if _, err := exec.LookPath("semanage"); err != nil {
		return fmt.Errorf("semanage not found in $PATH")
	}

	out, err := exec.Command("semanage", "fcontext", "-a", "-e", ecv1beta1.DefaultDataDir, dataDir).CombinedOutput()
	if err == nil {
		return nil
	}
	// Already set by a previous install on this host.
	if strings.Contains(string(out), "already defined") || strings.Contains(string(out), "already exists") {
		return nil
	}
	return fmt.Errorf("semanage fcontext -a -e: %w: %s", err, string(out))
}

// relabelDataDir applies the module's labels to files that already exist.
func (h *HostUtils) relabelDataDir(rc runtimeconfig.RuntimeConfig) error {
	if _, err := exec.LookPath("restorecon"); err != nil {
		return fmt.Errorf("restorecon not found in $PATH")
	}

	h.logger.Debugf("relabeling %s", rc.EmbeddedClusterHomeDirectory())
	if out, err := exec.Command("restorecon", "-RF", rc.EmbeddedClusterHomeDirectory()).CombinedOutput(); err != nil {
		return fmt.Errorf("restorecon: %w: %s", err, string(out))
	}

	return nil
}

// RemoveSELinuxModule removes the policy module and any custom data directory
// labeling, so reset does not leave them behind pointing at a directory that no
// longer exists. The containerd drop-in goes with /etc/k0s.
func (h *HostUtils) RemoveSELinuxModule(rc runtimeconfig.RuntimeConfig) error {
	if !selinux.GetEnabled() {
		h.logger.Debugln("selinux is not enabled, skipping selinux cleanup")
		return nil
	}

	h.logger.Debugln("removing selinux configuration")

	dataDir := rc.EmbeddedClusterHomeDirectory()
	if dataDir != ecv1beta1.DefaultDataDir {
		if _, err := exec.LookPath("semanage"); err == nil {
			out, err := exec.Command("semanage", "fcontext", "-d", "-e", ecv1beta1.DefaultDataDir, dataDir).CombinedOutput()
			if err != nil {
				h.logger.Debugf("unable to remove labeling for %s: %v: %s", dataDir, err, string(out))
			}
		}
	}

	if _, err := exec.LookPath("semodule"); err != nil {
		return nil
	}

	if out, err := exec.Command("semodule", "-r", selinuxModuleName).CombinedOutput(); err != nil {
		// Not installed is the common case: selinux was turned on after the
		// install, or this build shipped no module.
		h.logger.Debugf("unable to remove selinux policy module: %v: %s", err, string(out))
	}

	return nil
}
