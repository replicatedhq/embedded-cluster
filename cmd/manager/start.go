package main

import (
	"context"
	"fmt"
	"os"

	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
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
		RunE: func(cmd *cobra.Command, args []string) error {
			// init the kubeconfig and tmpdir from the data dir
			provider := defaults.NewProvider(dataDir)
			os.Setenv("KUBECONFIG", provider.PathToKubeConfig())
			os.Setenv("TMPDIR", provider.EmbeddedClusterTmpSubDir())

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
