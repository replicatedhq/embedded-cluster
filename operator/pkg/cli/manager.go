package cli

import (
	"context"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/pkg/manager/migrate"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func MigrateManagerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "manager",
		Short: "Migrate to the manager service",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMigrateManager(cmd.Context())
		},
	}

	return cmd
}

func runMigrateManager(ctx context.Context) error {
	logrus.SetLevel(logrus.DebugLevel)

	if err := migrate.Migrate(ctx); err != nil {
		return fmt.Errorf("failed to run manager migration: %w", err)
	}
	return nil
}
