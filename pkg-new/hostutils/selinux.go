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

// ConfigureSELinux loads our policy module, which labels the data directory and
// grants the bundled workloads what container-selinux denies them by default.
// See selinux/ec.te and selinux/ec.fc for what it contains and why.
//
// The module's file contexts are compiled in and only match the default data
// directory. A relocated one is handled with a path equivalence rule, so the
// same module rules apply there too.
//
// Best-effort: a host without selinux has none of these tools, and that should
// not fail an install. The selinux host preflights surface it instead.
func (h *HostUtils) ConfigureSELinux(rc runtimeconfig.RuntimeConfig) error {
	if !selinux.GetEnabled() {
		h.logger.Debugln("selinux is not enabled, skipping selinux configuration")
		return nil
	}

	if err := h.installSELinuxModule(rc); err != nil {
		h.logger.Debugf("unable to install selinux policy module: %v", err)
		return nil
	}

	if err := h.equateSELinuxDataDir(rc); err != nil {
		h.logger.Debugf("unable to equate relocated data directory: %v", err)
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

	h.logger.Debugf("installing selinux policy module %s", selinuxModuleName)
	// Reinstalls in place if already present, so this is safe on upgrade.
	if out, err := exec.Command("semodule", "-i", path).CombinedOutput(); err != nil {
		// Usually means container-selinux is absent, so the types the module
		// requires do not exist.
		return fmt.Errorf("semodule -i: %w: %s", err, string(out))
	}

	return nil
}

// equateSELinuxDataDir tells selinux that a relocated data directory is
// equivalent to the default one, so the file contexts compiled into our module
// apply to it. A no-op for the default path.
//
// This is what makes a configurable data directory work without generating
// policy on the host, which would need a compiler we cannot expect to be
// installed.
func (h *HostUtils) equateSELinuxDataDir(rc runtimeconfig.RuntimeConfig) error {
	dataDir := rc.EmbeddedClusterHomeDirectory()
	if dataDir == ecv1beta1.DefaultDataDir {
		return nil
	}

	if _, err := exec.LookPath("semanage"); err != nil {
		return fmt.Errorf("semanage not found in $PATH, cannot label a relocated data directory")
	}

	h.logger.Debugf("equating %s to %s for selinux", dataDir, ecv1beta1.DefaultDataDir)
	out, err := exec.Command("semanage", "fcontext", "-a", "-e", ecv1beta1.DefaultDataDir, dataDir).CombinedOutput()
	if err == nil {
		return nil
	}
	// Already equated, from a previous install on this host.
	if strings.Contains(string(out), "already defined") || strings.Contains(string(out), "already exists") {
		return nil
	}
	return fmt.Errorf("semanage fcontext -a -e: %w: %s", err, string(out))
}

// RemoveSELinuxModule unloads the policy module and drops any equivalence rule.
// Without this, reset leaves both behind, pointing at a data directory that no
// longer exists.
func (h *HostUtils) RemoveSELinuxModule(rc runtimeconfig.RuntimeConfig) error {
	if !selinux.GetEnabled() {
		h.logger.Debugln("selinux is not enabled, skipping selinux cleanup")
		return nil
	}

	if dataDir := rc.EmbeddedClusterHomeDirectory(); dataDir != ecv1beta1.DefaultDataDir {
		if _, err := exec.LookPath("semanage"); err == nil {
			h.logger.Debugf("removing selinux equivalence for %s", dataDir)
			if out, err := exec.Command("semanage", "fcontext", "-d", "-e", ecv1beta1.DefaultDataDir, dataDir).CombinedOutput(); err != nil {
				h.logger.Debugf("unable to remove selinux equivalence for %s: %v: %s", dataDir, err, string(out))
			}
		}
	}

	if _, err := exec.LookPath("semodule"); err != nil {
		return nil
	}

	h.logger.Debugf("removing selinux policy module %s", selinuxModuleName)
	if out, err := exec.Command("semodule", "-r", selinuxModuleName).CombinedOutput(); err != nil {
		// Not installed is the common case: selinux was turned on after the
		// install, or this build shipped no module.
		h.logger.Debugf("unable to remove selinux policy module: %v: %s", err, string(out))
	}

	return nil
}

// RestoreSELinuxContext relabels the data directory according to the file
// contexts our policy module registered.
func (h *HostUtils) RestoreSELinuxContext(rc runtimeconfig.RuntimeConfig) error {
	if !selinux.GetEnabled() {
		h.logger.Debugln("selinux is not enabled, skipping context restore")
		return nil
	}

	h.logger.Debugln("checking for restorecon binary in $PATH")
	if _, err := exec.LookPath("restorecon"); err != nil {
		h.logger.Debugln("restorecon not found in $PATH, skipping context restore")
		return nil
	}

	h.logger.Debugf("relabeling embedded-cluster data directory with restorecon")
	out, err := exec.Command("restorecon", "-RvF", rc.EmbeddedClusterHomeDirectory()).CombinedOutput()
	if err != nil {
		h.logger.Debugf("unable to run restorecon: %v", err)
		h.logger.Debugln(string(out))
	}

	return nil
}
