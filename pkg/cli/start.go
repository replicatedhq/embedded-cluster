package cli //nolint:dupl

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/k0sproject/k0s/cmd/start"
	"github.com/spf13/cobra"
)

// CmdStartOptions is a struct to support the start command
type CmdStartOptions struct {
	DataDir string
}

// NewCmdStartOptions returns initialized StartOptions
func NewCmdStartOptions() *CmdStartOptions {
	return &CmdStartOptions{}
}

// NewCmdStart returns a cobra command for starting the systemd service
func NewCmdStart(cli *CLI) *cobra.Command {
	o := NewCmdStartOptions()
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Starts the systemd service",
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
func (o *CmdStartOptions) Complete(_ []string) error {
	// nothing to complete
	return nil
}

// Validate validates the provided options
func (o *CmdStartOptions) Validate() error {
	// nothing to validate
	return nil
}

// Run executes the command
func (o *CmdStartOptions) Run(ctx context.Context, _ *CLI) error {
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	cmd := start.NewStartCmd()
	cmd.SetArgs([]string{})
	err := cmd.ExecuteContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to start k0s: %w", err)
	}

	return nil
}
