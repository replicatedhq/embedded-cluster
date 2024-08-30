package cli

import (
	"fmt"

	"github.com/replicatedhq/embedded-cluster-operator/pkg/migrations"
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
			err := migrations.RegistryData(cmd.Context())
			if err != nil {
				return fmt.Errorf("migration failed: %w", err)
			}
			return nil
		},
	}

	return cmd
}
