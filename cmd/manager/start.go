package main

import (
	"context"
	"fmt"
	"os"

	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/socket"
	"github.com/replicatedhq/embedded-cluster/pkg/websocket"
	"github.com/spf13/cobra"
)

func StartCmd(ctx context.Context, name string) *cobra.Command {
	var (
		disableWebsocket bool
	)

	cmd := &cobra.Command{
		Use:   "start",
		Short: fmt.Sprintf("Start the %s cluster manager", name),
		PreRun: func(cmd *cobra.Command, args []string) {
			// init runtime config and relevant env vars
			if dataDir := os.Getenv("DATA_DIR"); dataDir != "" {
				runtimeconfig.SetDataDir(dataDir)
			}

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
			if !disableWebsocket {
				go func() {
					kcli, err := kubeutils.KubeClient()
					if err != nil {
						panic(err)
					}
					websocket.ConnectToKOTSWebSocket(ctx, kcli)
				}()
			}

			<-ctx.Done()
			return nil
		},
	}

	// flags to enable running in test mode
	cmd.Flags().BoolVar(&disableWebsocket, "disable-websocket", false, "When set, don't connect to the KOTS webscoket")
	cmd.Flags().MarkHidden("disable-websocket")

	return cmd
}
