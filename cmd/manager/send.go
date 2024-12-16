package main

import (
	"context"
	"os"

	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/spf13/cobra"
)

// SendCmd is a debug command that sends a message
// on the unix socket so that we can test, debug, simulate
// receiving messages from the websocket
func SendCmd(ctx context.Context, name string) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "send",
		Short:  name,
		Hidden: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// init runtime config and relevant env vars
			runtimeconfig.ApplyFlags(cmd.Flags())
			os.Setenv("KUBECONFIG", runtimeconfig.PathToKubeConfig())
			os.Setenv("TMPDIR", runtimeconfig.EmbeddedClusterTmpSubDir())
		},
		Run: func(cmd *cobra.Command, args []string) {

		},
	}

	cmd.AddCommand(SendUpgradeClusterCmd(ctx, name))
	return cmd
}
