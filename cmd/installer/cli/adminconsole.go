package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func AdminConsoleCmd(ctx context.Context, releaseName string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "admin-console",
		Short: fmt.Sprintf("Manage the %s Admin Console", releaseName),
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	cmd.AddCommand(AdminConsoleResetPasswordCmd(ctx, releaseName))

	return cmd
}
