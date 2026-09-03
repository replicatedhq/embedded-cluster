package main

import (
	"fmt"

	"github.com/replicatedhq/embedded-cluster/pkg-new/hostutils"
	"github.com/spf13/cobra"
)

// ConfigureSELinuxCmd applies the selinux configuration on upgrade. Installs go
// through hostutils.ConfigureHost, which does this already, but the upgrade
// path never calls it -- so without this an upgraded cluster keeps whatever the
// release that installed it applied, and never picks up policy changes or, for
// a cluster installed before the policy module existed, gets one at all.
//
// Run per node from the freshly pulled target binary, so the policy applied is
// the target release's.
func ConfigureSELinuxCmd(cli *CLI) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "configure-selinux",
		Short: "Apply the selinux policy and labels for this release",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			cli.bindFlags(cmd.Flags())
			cli.setupDataDir()
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// enable_selinux in the containerd drop-in. Only takes effect when
			// containerd restarts, which autopilot does later in the upgrade.
			if err := hostutils.ConfigureContainerdSELinux(); err != nil {
				return fmt.Errorf("configure containerd selinux: %w", err)
			}

			if err := hostutils.ConfigureSELinux(cli.RC); err != nil {
				return fmt.Errorf("configure selinux: %w", err)
			}

			if err := hostutils.RestoreSELinuxContext(cli.RC); err != nil {
				return fmt.Errorf("restore selinux context: %w", err)
			}

			return nil
		},
	}

	return cmd
}
