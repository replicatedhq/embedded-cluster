package hostutils

import (
	"os/exec"

	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
)

func (h *HostUtils) ConfigureSELinuxFcontext(rc runtimeconfig.RuntimeConfig) error {
	h.logger.Debugln("checking for semanage binary in $PATH")
	if _, err := exec.LookPath("semanage"); err != nil {
		h.logger.Debugln("semanage not found in $PATH")
		return nil
	}

	h.logger.Debugf("setting selinux fcontext for embedded-cluster binary directory to bin_t")
	args := []string{
		"fcontext",
		"-a",
		"-s",
		"system_u",
		"-t",
		"bin_t",
		rc.EmbeddedClusterBinsSubDir() + "(/.*)?",
	}
	out, err := exec.Command("semanage", args...).CombinedOutput()
	if err != nil {
		h.logger.Debugf("unable to set contexts on binary directory: %v", err)
		h.logger.Debugln(string(out))
	}

	return nil
}

func (h *HostUtils) RestoreSELinuxContext(rc runtimeconfig.RuntimeConfig) error {
	h.logger.Debugln("checking for restorecon binary in $PATH")
	if _, err := exec.LookPath("restorecon"); err != nil {
		h.logger.Debugln("restorecon not found in $PATH")
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
