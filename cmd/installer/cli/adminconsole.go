package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func AdminConsoleCmd(ctx context.Context, name string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "admin-console",
		Short: fmt.Sprintf("Manage the %s Admin Console", name),
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	cmd.AddCommand(AdminConsoleResetPasswordCmd(ctx, name))

	return cmd
}
