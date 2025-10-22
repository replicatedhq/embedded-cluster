package cli

import (
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/spf13/cobra"
)

func RootCmd(cli *CLI) *cobra.Command {
	cmd := &cobra.Command{
		Use:   cli.Name,
		Short: "Run or pull data for the local artifact mirror",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			cli.bindFlags(cmd.Flags())

			// If the command is help, don't setup the data dir
			if cmd.Name() == "help" {
				return nil
			}

			cli.setupDataDir()
			return nil
		},
	}

	cmd.AddCommand(ServeCmd(cli))
	cmd.AddCommand(PullCmd(cli))

	cmd.PersistentFlags().String("data-dir", ecv1beta1.DefaultDataDir, "Path to the data directory")

	cobra.OnInitialize(func() {
		cli.init()
	})

	return cmd
}
