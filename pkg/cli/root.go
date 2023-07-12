package cli

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// RootOptions is a struct to support the `helmbin` command
type RootOptions struct {
}

// NewDefaultRootCommand creates the `helmbin` command with default arguments
func NewDefaultRootCommand() *cobra.Command {
	var debug bool
	cli := NewCLI()
	cmd := &cobra.Command{
		Use:   cli.Name,
		Short: "An embeddable Kubernetes distribution",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if debug {
				logrus.SetLevel(logrus.DebugLevel)
			}
		},
	}
	cmd.SetArgs(cli.Args[1:])

	cmd.AddCommand(NewCmdRun(cli))
	cmd.AddCommand(NewCmdInstall(cli))
	// required for the install command
	controllerCmd := NewCmdRunController(cli)
	controllerCmd.Hidden = true
	cmd.AddCommand(controllerCmd)

	cmd.AddCommand(NewCmdStart(cli))
	cmd.AddCommand(NewCmdStop(cli))
	cmd.AddCommand(NewCmdKubeconfig(cli))
	cmd.AddCommand(NewCmdKubectl(cli))
	cmd.AddCommand(NewCmdVersion(cli))
	cmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "Enables debug logging")
	return cmd
}
