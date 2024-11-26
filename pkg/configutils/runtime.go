package configutils

import (
	"fmt"
	"os/exec"

	"github.com/replicatedhq/embedded-cluster/pkg/goods"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/sirupsen/logrus"
)

// sysctlConfigPath is the path to the sysctl config file that is used to configure
// the embedded cluster. This could have been a constant but we want to be able to
// override it for testing purposes.
var sysctlConfigPath = "/etc/sysctl.d/99-embedded-cluster.conf"

// ConfigureSysctl writes the sysctl config file for the embedded cluster and
// reloads the sysctl configuration. This function has a distinct behavior: if
// the sysctl binary does not exist it returns an error but if it fails to lay
// down the sysctl config on disk it simply returns nil.
func ConfigureSysctl() error {
	if _, err := exec.LookPath("sysctl"); err != nil {
		return fmt.Errorf("unable to find sysctl binary: %w", err)
	}

	materializer := goods.NewMaterializer()
	if err := materializer.SysctlConfig(sysctlConfigPath); err != nil {
		logrus.Debugf("unable to materialize sysctl config: %v", err)
		return nil
	}

	if _, err := helpers.RunCommand("sysctl", "--system"); err != nil {
		logrus.Debugf("unable to configure sysctl: %v", err)
	}
	return nil
}
