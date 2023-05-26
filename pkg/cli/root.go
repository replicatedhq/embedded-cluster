package cli

import (
	"flag"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/klog/v2"
)

// RootOptions is a struct to support the `helmbin` command
type RootOptions struct {
}

// NewDefaultRootCommand creates the `helmbin` command with default arguments
func NewDefaultRootCommand() *cobra.Command {
	var debug bool
	cli := NewCLI()
	cmd := &cobra.Command{
		Use:   filepath.Base(cli.Args[0]),
		Short: "An embeddable Kubernetes distribution",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if debug {
				logrus.SetLevel(logrus.DebugLevel)
			}
		},
	}
	cmd.SetArgs(cli.Args[1:])
	initKlog(cmd.PersistentFlags())
	cmd.AddCommand(NewCmdRun(cli))
	cmd.AddCommand(NewCmdInstall(cli))
	cmd.AddCommand(NewCmdStart(cli))
	cmd.AddCommand(NewCmdStop(cli))
	cmd.AddCommand(NewCmdKubectl(cli))
	cmd.AddCommand(NewCmdVersion(cli))
	cmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "Enables debug logging")
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
