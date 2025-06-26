package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/replicatedhq/embedded-cluster/cmd/installer/goods"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/dryrun"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/spf13/cobra"
)

func MaterializeCmd(ctx context.Context, name string) *cobra.Command {
	var dataDir string
	rc := runtimeconfig.New(nil)

	cmd := &cobra.Command{
		Use:    "materialize",
		Short:  "Materialize embedded assets into the data directory",
		Hidden: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// Skip root check if dryrun mode is enabled
			if !dryrun.Enabled() && os.Getuid() != 0 {
				return fmt.Errorf("materialize command must be run as root")
			}

			rc.SetDataDir(dataDir)
			os.Setenv("TMPDIR", rc.EmbeddedClusterTmpSubDir())

			return nil
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			rc.Cleanup()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			materializer := goods.NewMaterializer(rc)
			if err := materializer.Materialize(); err != nil {
				return fmt.Errorf("unable to materialize: %v", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&dataDir, "data-dir", ecv1beta1.DefaultDataDir, "Path to the data directory")

	return cmd
}
