package k0s

import (
	"fmt"

	"github.com/replicatedhq/embedded-cluster/pkg/config"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
)

// Install runs the k0s install command and waits for it to finish. If no configuration
// is found one is generated.
func Install(networkInterface string, provider *defaults.Provider) error {
	ourbin := provider.PathToEmbeddedClusterBinary("k0s")
	hstbin := defaults.K0sBinaryPath()
	if err := helpers.MoveFile(ourbin, hstbin); err != nil {
		return fmt.Errorf("unable to move k0s binary: %w", err)
	}

	nodeIP, err := netutils.FirstValidAddress(networkInterface)
	if err != nil {
		return fmt.Errorf("unable to find first valid address: %w", err)
	}
	if _, err := helpers.RunCommand(hstbin, config.InstallFlags(provider, nodeIP)...); err != nil {
		return fmt.Errorf("unable to install: %w", err)
	}
	if _, err := helpers.RunCommand(hstbin, "start"); err != nil {
		return fmt.Errorf("unable to start: %w", err)
	}
	return nil
}
