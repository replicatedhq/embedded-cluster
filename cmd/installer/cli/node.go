package cli

import (
	"context"

	"github.com/spf13/cobra"
)

func NodeCmd(ctx context.Context, appSlug string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node",
		Short: "Manage cluster nodes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		Hidden: true,
	}

	// here for legacy reasons
	joinCmd := JoinCmd(ctx, appSlug)
	joinCmd.Hidden = true
	cmd.AddCommand(joinCmd)

	resetCmd := ResetCmd(ctx, appSlug)
	resetCmd.Hidden = true
	cmd.AddCommand(resetCmd)

	return cmd
}
