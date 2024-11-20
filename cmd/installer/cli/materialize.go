package cli

import (
	"context"
	"fmt"
	"os"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/goods"
	"github.com/spf13/cobra"
)

func MaterializeCmd(ctx context.Context, name string) *cobra.Command {
	runtimeConfig := ecv1beta1.GetDefaultRuntimeConfig()

	var (
		dataDir string
	)

	cmd := &cobra.Command{
		Use:   "materialize",
		Short: "Materialize embedded assets into the data directory",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if os.Getuid() != 0 {
				return fmt.Errorf("materialize command must be run as root")
			}

			dd, err := cmd.Flags().GetString("data-dir")
			if err != nil {
				return fmt.Errorf("unable to get data-dir flag: %w", err)
			}

			runtimeConfig.DataDir = dd

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			provider := defaults.NewProviderFromRuntimeConfig(runtimeConfig)
			os.Setenv("TMPDIR", provider.EmbeddedClusterTmpSubDir())

			defer tryRemoveTmpDirContents(provider)

			materializer := goods.NewMaterializer(provider)
			if err := materializer.Materialize(); err != nil {
				return fmt.Errorf("unable to materialize: %v", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&dataDir, "data-dir", ecv1beta1.DefaultDataDir, "Path to the data directory")

	return cmd
}
