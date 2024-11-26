package main

import (
	"context"
	"fmt"
	"os"

	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/socket"
	"github.com/replicatedhq/embedded-cluster/pkg/websocket"
	"github.com/spf13/cobra"
)

func StartCmd(ctx context.Context, name string) *cobra.Command {
	var (
		dataDir string
	)

	cmd := &cobra.Command{
		Use:   "start",
		Short: fmt.Sprintf("Start the %s cluster manager", name),
		PreRun: func(cmd *cobra.Command, args []string) {
			// init runtime config and relevant env vars
			runtimeconfig.ApplyFlags(cmd.Flags())
			os.Setenv("KUBECONFIG", runtimeconfig.PathToKubeConfig())
			os.Setenv("TMPDIR", runtimeconfig.EmbeddedClusterTmpSubDir())
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// start a unix socket so we can respond to commands
			go func() {
				if err := socket.StartSocketServer(ctx); err != nil {
					panic(err)
				}
			}()

			// connect to the KOTS WebSocket server
			go websocket.ConnectToKOTSWebSocket(ctx)

			<-ctx.Done()
			return nil
		},
	}

	cmd.Flags().StringVar(&dataDir, "data-dir", "", "Path to the data directory")

	return cmd
}
