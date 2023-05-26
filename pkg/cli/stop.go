package cli //nolint:dupl

import (
	"fmt"

	"github.com/k0sproject/k0s/cmd/stop"
	"github.com/spf13/cobra"
)

// NewCmdStop returns a cobra command for stopping the systemd service
func NewCmdStop(_ *CLI) *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stops the systemd service",
		RunE: func(cmd *cobra.Command, args []string) error {
			k0scmd := stop.NewStopCmd()
			k0scmd.SetArgs([]string{})
			if err := cmd.ExecuteContext(cmd.Context()); err != nil {
				return fmt.Errorf("failed to stop k0s: %w", err)
			}
			return nil
		},
	}
}
