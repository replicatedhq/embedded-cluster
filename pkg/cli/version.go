package cli

import (
	"context"
	"fmt"

	"github.com/emosbaugh/helmbin/pkg/version"
	"github.com/spf13/cobra"
)

// CmdVersionOptions is a struct to support the version command
type CmdVersionOptions struct {
}

// NewCmdVersionOptions returns initialized VersionOptions
func NewCmdVersionOptions() *CmdVersionOptions {
	return &CmdVersionOptions{}
}

// NewCmdVersion returns a cobra command for printing version information
func NewCmdVersion(cli *CLI) *cobra.Command {
	o := NewCmdVersionOptions()
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Prints version information",
		Long:  "Prints version information",
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
func (o *CmdVersionOptions) Complete(_ []string) error {
	// nothing to complete
	return nil
}

// Validate validates the provided options
func (o *CmdVersionOptions) Validate() error {
	// nothing to validate
	return nil
}

// Run executes the command
func (o *CmdVersionOptions) Run(_ context.Context, cli *CLI) error {
	fmt.Fprintln(cli.Out, version.Get())
	return nil
}
