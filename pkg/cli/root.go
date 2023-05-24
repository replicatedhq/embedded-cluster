package cli

import (
	"flag"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/klog/v2"
)

// RootOptions is a struct to support `helmbin` command
type RootOptions struct {
}

// NewDefaultRootCommand creates the `helmbin` command with default arguments
func NewDefaultRootCommand() *cobra.Command {
	cli := NewCLI()
	return NewRootCommand(cli)
}

// NewRootCommand creates the `helmbin` command and its nested children.
func NewRootCommand(cli *CLI) *cobra.Command {
	executable := filepath.Base(cli.Args[0])

	var debug bool

	cmd := &cobra.Command{
		Use:   executable,
		Short: "TODO",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if debug {
				logrus.SetLevel(logrus.DebugLevel)
			}
		},
	}
	cmd.SetArgs(cli.Args[1:])

	initKlog(cmd.PersistentFlags())
	cmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "Debug logging (default: false)")

	cmd.AddCommand(NewCmdServer(cli))
	cmd.AddCommand(NewCmdInstall(cli))
	cmd.AddCommand(NewCmdKubectl(cli))
	cmd.AddCommand(NewCmdVersion(cli))

	return cmd
}

// TODO: use logrus
func initKlog(fs *pflag.FlagSet) {
	var allFlags flag.FlagSet
	klog.InitFlags(&allFlags)
	allFlags.VisitAll(func(f *flag.Flag) {
		switch f.Name {
		case "v", "vmodule":
			fs.AddGoFlag(f)
		}
	})
}
