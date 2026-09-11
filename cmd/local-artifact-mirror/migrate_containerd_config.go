package main

import (
	"fmt"

	"github.com/replicatedhq/embedded-cluster/pkg-new/hostutils"
	"github.com/spf13/cobra"
)

// MigrateContainerdConfigCmd reconciles the containerd registry drop-in with the
// k0s 1.36+ schema (see hostutils.MigrateContainerdConfigToV3): airgap migrates it
// to the containerd 2.x schema, online deletes it if present. Run per node on upgrade.
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

	cmd.Flags().Bool("airgap", false, "Whether this is an airgap install: airgap migrates the registry drop-in to the containerd 2.x (k0s 1.36+) schema; online deletes it if present (online installs don't use the in-cluster registry)")

	return cmd
}
