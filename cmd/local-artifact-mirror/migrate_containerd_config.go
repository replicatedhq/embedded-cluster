package main

import (
	"fmt"

	"github.com/replicatedhq/embedded-cluster/pkg-new/hostutils"
	"github.com/spf13/cobra"
)

// MigrateContainerdConfigCmd migrates the containerd registry drop-in to the
// k0s 1.36+ schema (see hostutils.MigrateContainerdConfigToV3). Run per node
// during an airgap upgrade.
func MigrateContainerdConfigCmd(cli *CLI) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate-containerd-config",
		Short: "Migrate the containerd registry drop-in to the containerd 2.x (k0s 1.36+) schema",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			cli.bindFlags(cmd.Flags())
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := hostutils.MigrateContainerdConfigToV3(cli.V.GetBool("airgap")); err != nil {
				return fmt.Errorf("migrate containerd config: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().Bool("airgap", false, "Migrate the drop-in to the v3 schema instead of removing it (online installs don't use the in-cluster registry)")

	return cmd
}
