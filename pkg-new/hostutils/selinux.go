package hostutils

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/opencontainers/selinux/go-selinux"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
)

// fcontextRule is a single selinux file context rule: every path matching Regex
// gets Type as its selinux type.
type fcontextRule struct {
	Regex string
	Type  string
}

// fcontextRules returns the file context rules embedded cluster needs.
//
// The bin_t rule covers our own binaries (the installer, kubectl, the local
// artifact mirror). They live under the data directory, which the base policy
// labels var_lib_t, and a var_lib_t file cannot be executed.
//
// The remaining rules are the ones k0s documents for running under selinux (see
// https://docs.k0sproject.io/stable/selinux/). k0s ships containerd and runc
// inside its own data directory rather than installing them from a package, so
// container-selinux's file contexts -- which target /usr/bin/containerd and
// /var/lib/containers -- never match them. Without these, containerd does not
// transition into container_runtime_t and container images are not labeled as
// container content.
func fcontextRules(rc runtimeconfig.RuntimeConfig) []fcontextRule {
	k0sDir := rc.EmbeddedClusterK0sSubDir()
	containerdDir := filepath.Join(k0sDir, "containerd")

	return []fcontextRule{
		{rc.EmbeddedClusterBinsSubDir() + "(/.*)?", "bin_t"},
		{filepath.Join(k0sDir, "bin", "containerd.*"), "container_runtime_exec_t"},
		{filepath.Join(k0sDir, "bin", "runc"), "container_runtime_exec_t"},
		{containerdDir + "(/.*)?", "container_var_lib_t"},
		{filepath.Join(containerdDir, "io.containerd.snapshotter.*", "snapshots") + "(/.*)?", "container_ro_file_t"},
	}
}

// ConfigureSELinuxFcontext registers the file context rules from fcontextRules
// with the local policy store. The rules persist across reboots and survive a
// filesystem relabel; RestoreSELinuxContext applies them.
//
// This is best-effort: a host without selinux has no semanage, and a host
// without container-selinux has no container_* types. Neither should fail an
// install, but both are surfaced by the selinux host preflights.
func (h *HostUtils) ConfigureSELinuxFcontext(rc runtimeconfig.RuntimeConfig) error {
	if !selinux.GetEnabled() {
		h.logger.Debugln("selinux is not enabled, skipping fcontext configuration")
		return nil
	}

	h.logger.Debugln("checking for semanage binary in $PATH")
	if _, err := exec.LookPath("semanage"); err != nil {
		h.logger.Debugln("semanage not found in $PATH, skipping fcontext configuration")
		return nil
	}

	for _, rule := range fcontextRules(rc) {
		h.logger.Debugf("setting selinux fcontext for %s to %s", rule.Regex, rule.Type)
		if err := h.applyFcontextRule(rule); err != nil {
			// A missing type means container-selinux is not installed. Log it
			// and keep going so the rules that can apply still do.
			h.logger.Debugf("unable to set selinux fcontext for %s: %v", rule.Regex, err)
		}
	}

	return nil
}

// applyFcontextRule adds a file context rule, modifying it instead if it is
// already defined. `semanage fcontext -a` errors on a path that already has a
// rule, which happens on every reinstall and upgrade.
func (h *HostUtils) applyFcontextRule(rule fcontextRule) error {
	out, err := h.runSemanageFcontext("-a", rule)
	if err == nil {
		return nil
	}

	if !strings.Contains(out, "already defined") {
		return fmt.Errorf("add fcontext: %w: %s", err, out)
	}

	h.logger.Debugf("selinux fcontext for %s already defined, modifying it instead", rule.Regex)
	if out, err := h.runSemanageFcontext("-m", rule); err != nil {
		return fmt.Errorf("modify fcontext: %w: %s", err, out)
	}
	return nil
}

func (h *HostUtils) runSemanageFcontext(action string, rule fcontextRule) (string, error) {
	args := []string{"fcontext", action, "-s", "system_u", "-t", rule.Type, rule.Regex}
	out, err := exec.Command("semanage", args...).CombinedOutput()
	return string(out), err
}

// RemoveSELinuxFcontext deletes the file context rules registered by
// ConfigureSELinuxFcontext. Without this, reset leaves them in the local policy
// store forever, still pointing at a data directory that no longer exists.
func (h *HostUtils) RemoveSELinuxFcontext(rc runtimeconfig.RuntimeConfig) error {
	if !selinux.GetEnabled() {
		h.logger.Debugln("selinux is not enabled, skipping fcontext removal")
		return nil
	}

	h.logger.Debugln("checking for semanage binary in $PATH")
	if _, err := exec.LookPath("semanage"); err != nil {
		h.logger.Debugln("semanage not found in $PATH, skipping fcontext removal")
		return nil
	}

	for _, rule := range fcontextRules(rc) {
		h.logger.Debugf("removing selinux fcontext for %s", rule.Regex)
		out, err := exec.Command("semanage", "fcontext", "-d", "-t", rule.Type, rule.Regex).CombinedOutput()
		if err == nil {
			continue
		}
		// A rule we never managed to add, or one already removed, is not a failure.
		if strings.Contains(string(out), "not defined") || strings.Contains(string(out), "does not exist") {
			h.logger.Debugf("selinux fcontext for %s was not defined", rule.Regex)
			continue
		}
		h.logger.Debugf("unable to remove selinux fcontext for %s: %v: %s", rule.Regex, err, string(out))
	}

	return nil
}

// RestoreSELinuxContext relabels the data directory according to the rules
// registered by ConfigureSELinuxFcontext.
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
