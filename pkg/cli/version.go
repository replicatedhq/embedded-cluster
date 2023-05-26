package cli

import (
	"fmt"

	"github.com/emosbaugh/helmbin/pkg/version"
	"github.com/spf13/cobra"
)

// NewCmdVersion returns a cobra command for printing version information
func NewCmdVersion(cli *CLI) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Prints version information",
		Long:  "Prints version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cli.Out, version.Get())
			return nil
		},
	}
}
