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

// fcontextRules returns the file context rules embedded cluster needs, in the
// order they are registered. Selinux matches the most specific rule, so the
// narrower rules below override the blanket one.
//
// container-selinux's own file contexts target the standard paths --
// /var/lib/kubelet, /var/lib/containers, /usr/bin/containerd -- and we put
// everything under our data directory instead. Without these rules the whole
// tree keeps the base policy's var_lib_t, which confined containers cannot
// read, so anything mounting a host path out of it fails. k3s and rke2 ship
// the same shape of policy for the same reason.
//
// Blanket container_var_lib_t because kotsadm mounts the entire data directory
// and the entire k0s directory. Then:
//
//   - bin_t on our own binaries (the installer, kubectl, the local artifact
//     mirror), because a var_lib_t file cannot be executed.
//   - container_file_t on openebs-local and seaweedfs, which back persistent
//     volumes that containers read and write.
//   - container_runtime_exec_t on containerd and runc. k0s already labels
//     these itself, but with a chcon-style call that does not survive a
//     filesystem relabel, and the blanket rule above would otherwise demote
//     them on the next restorecon. k0s cannot register fcontext rules because
//     it ships no package to register them from.
func fcontextRules(rc runtimeconfig.RuntimeConfig) []fcontextRule {
	k0sBinDir := filepath.Join(rc.EmbeddedClusterK0sSubDir(), "bin")

	return []fcontextRule{
		{rc.EmbeddedClusterHomeDirectory() + "(/.*)?", "container_var_lib_t"},
		{rc.EmbeddedClusterBinsSubDir() + "(/.*)?", "bin_t"},
		{rc.EmbeddedClusterOpenEBSLocalSubDir() + "(/.*)?", "container_file_t"},
		{rc.EmbeddedClusterSeaweedFSSubDir() + "(/.*)?", "container_file_t"},
		{filepath.Join(k0sBinDir, "containerd.*"), "container_runtime_exec_t"},
		{filepath.Join(k0sBinDir, "runc"), "container_runtime_exec_t"},
	}
}

// selinuxBooleans are the container-selinux tunables embedded cluster needs.
//
// container_read_certs lets container domains read cert_t files. We mount the
// host CA bundle into the admin console and the operator, and container-selinux
// denies that by default.
//
// Deliberately not reverted on reset: booleans are host wide, and another
// workload may have come to depend on it.
var selinuxBooleans = []string{"container_read_certs"}

// ConfigureSELinuxFcontext registers the file context rules from fcontextRules
// with the local policy store. The rules persist across reboots and survive a
// filesystem relabel; RestoreSELinuxContext applies them.
//
// This is best-effort: a host without selinux has no semanage. That should not
// fail an install, and the selinux host preflights surface it.
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

// ConfigureSELinuxBooleans turns on the container-selinux tunables listed in
// selinuxBooleans. Best-effort for the same reasons as the fcontext rules.
func (h *HostUtils) ConfigureSELinuxBooleans() error {
	if !selinux.GetEnabled() {
		h.logger.Debugln("selinux is not enabled, skipping boolean configuration")
		return nil
	}

	h.logger.Debugln("checking for setsebool binary in $PATH")
	if _, err := exec.LookPath("setsebool"); err != nil {
		h.logger.Debugln("setsebool not found in $PATH, skipping boolean configuration")
		return nil
	}

	for _, name := range selinuxBooleans {
		h.logger.Debugf("enabling selinux boolean %s", name)
		// -P persists the change across reboots.
		if out, err := exec.Command("setsebool", "-P", name, "on").CombinedOutput(); err != nil {
			h.logger.Debugf("unable to enable selinux boolean %s: %v: %s", name, err, string(out))
		}
	}

	return nil
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
