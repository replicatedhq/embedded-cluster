package cli

import (
	"context"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/cmd/installer/kotscli"
	"github.com/spf13/cobra"
)

func JoinPrintCommandCmd(ctx context.Context, name string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "print-command",
		Short: fmt.Sprintf("Print controller join command for %s", name),
		RunE: func(cmd *cobra.Command, args []string) error {
			jcmd, err := kotscli.GetJoinCommand(cmd.Context())
			if err != nil {
				return fmt.Errorf("unable to get join command: %w", err)
			}
			fmt.Println(jcmd)
			return nil
		},
	}

	return cmd
}
