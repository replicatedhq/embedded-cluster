package k0s

import (
	"fmt"
	"os"

	"github.com/replicatedhq/embedded-cluster/pkg/config"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
)

// Install runs the k0s install command and waits for it to finish. If no configuration
// is found one is generated.
func Install(networkInterface string) error {
	ourbin := runtimeconfig.PathToEmbeddedClusterBinary("k0s")
	hstbin := runtimeconfig.K0sBinaryPath()
	if err := helpers.MoveFile(ourbin, hstbin); err != nil {
		return fmt.Errorf("unable to move k0s binary: %w", err)
	}

	nodeIP, err := netutils.FirstValidAddress(networkInterface)
	if err != nil {
		return fmt.Errorf("unable to find first valid address: %w", err)
	}
	if _, err := helpers.RunCommand(hstbin, config.InstallFlags(nodeIP)...); err != nil {
		return fmt.Errorf("unable to install: %w", err)
	}
	if _, err := helpers.RunCommand(hstbin, "start"); err != nil {
		return fmt.Errorf("unable to start: %w", err)
	}
	return nil
}

// IsInstalled checks if the embedded cluster is already installed by looking for
// the k0s configuration file existence.
func IsInstalled() (bool, error) {
	_, err := os.Stat(runtimeconfig.PathToK0sConfig())
	if err == nil {
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	}

	return false, fmt.Errorf("unable to check if already installed: %w", err)
}
