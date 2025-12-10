package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func AdminConsoleCmd(ctx context.Context, appTitle string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "admin-console",
		Short: fmt.Sprintf("Manage the %s Admin Console", appTitle),
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	cmd.AddCommand(AdminConsoleResetPasswordCmd(ctx, appTitle))
	cmd.AddCommand(AdminConsoleUpdateTLSCmd(ctx, appTitle))

	return cmd
}
