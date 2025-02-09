package cli

import (
	"fmt"

	"github.com/replicatedhq/embedded-cluster/pkg/addons/registry/migrate"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/spf13/cobra"
)

func MigrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Run the specified migration",
	}

	cmd.AddCommand(
		MigrateRegistryDataCmd(),
	)

	return cmd
}

func MigrateRegistryDataCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "registry-data",
		Short:        "Run the registry-data migration",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			cli, err := kubeutils.KubeClient()
			if err != nil {
				return fmt.Errorf("failed to create kubernetes client: %w", err)
			}

			err = migrate.RegistryData(ctx, cli)
			if err != nil {
				return fmt.Errorf("failed to migrate registry data: %w", err)
			}
			return nil
		},
	}

	return cmd
}
