package cli //nolint:dupl

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/k0sproject/k0s/cmd/stop"
	"github.com/spf13/cobra"
)

// CmdStopOptions is a struct to support the stop command
type CmdStopOptions struct {
	DataDir string
}

// NewCmdStopOptions returns initialized StopOptions
func NewCmdStopOptions() *CmdStopOptions {
	return &CmdStopOptions{}
}

// NewCmdStop returns a cobra command for stopping the systemd service
func NewCmdStop(cli *CLI) *cobra.Command {
	o := NewCmdStopOptions()
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stops the systemd service",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Complete(args); err != nil {
				return err
			}
			if err := o.Validate(); err != nil {
				return err
			}
			return o.Run(cmd.Context(), cli)
		},
	}

	return cmd
}

// Complete completes all the required options
func (o *CmdStopOptions) Complete(_ []string) error {
	// nothing to complete
	return nil
}

// Validate validates the provided options
func (o *CmdStopOptions) Validate() error {
	// nothing to validate
	return nil
}

// Run executes the command
func (o *CmdStopOptions) Run(ctx context.Context, _ *CLI) error {
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	cmd := stop.NewStopCmd()
	cmd.SetArgs([]string{})
	err := cmd.ExecuteContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to stop k0s: %w", err)
	}

	return nil
}
