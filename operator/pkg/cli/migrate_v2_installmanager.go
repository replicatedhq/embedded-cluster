package cli

import (
	"fmt"

	"github.com/replicatedhq/embedded-cluster/operator/pkg/cli/migratev2"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// MigrateV2InstallManagerCmd returns a cobra command run by the migrate-v2 command that is run in
// a pod on all nodes in the cluster. It will download the manager binary and install it as a
// systemd service on the host.
func MigrateV2InstallManagerCmd() *cobra.Command {
	var flags migrateV2Flags

	cmd := &cobra.Command{
		Use:   "install-manager",
		Short: "Downloads the v2 manager binary and installs it as a systemd service.",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			logrus.SetLevel(logrus.DebugLevel)

			err := flags.Bind()
			if err != nil {
				return fmt.Errorf("failed to bind flags: %w", err)
			}

			// set the runtime config from the installation spec
			runtimeconfig.Set(flags.installation.Spec.RuntimeConfig)

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := migratev2.InstallAndStartManager(
				cmd.Context(), flags.licenseID, flags.licenseEndpoint, flags.versionLabel,
			); err != nil {
				return fmt.Errorf("failed to run manager migration: %w", err)
			}
			return nil
		},
	}

	err := flags.Configure(cmd)
	if err != nil {
		panic(err)
	}

	return cmd
}
