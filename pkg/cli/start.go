package cli //nolint:dupl

import (
	"fmt"

	"github.com/k0sproject/k0s/cmd/start"
	"github.com/spf13/cobra"
)

// NewCmdStart returns a cobra command for starting the systemd service
func NewCmdStart(cli *CLI) *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Starts the systemd service",
		RunE: func(cmd *cobra.Command, args []string) error {
			k0scmd := start.NewStartCmd()
			k0scmd.SetArgs([]string{})
			if err := cmd.ExecuteContext(cmd.Context()); err != nil {
				return fmt.Errorf("failed to start k0s: %w", err)
			}
			return nil
		},
	}
}
