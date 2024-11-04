package cmd

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/goods"
)

func materializeCommand() *cli.Command {
	runtimeConfig := ecv1beta1.GetDefaultRuntimeConfig()

	return &cli.Command{
		Name:   "materialize",
		Usage:  "Materialize embedded assets into the data directory",
		Hidden: true,
		Flags: []cli.Flag{
			getDataDirFlagWithDefault(runtimeConfig),
		},
		Before: func(c *cli.Context) error {
			if os.Getuid() != 0 {
				return fmt.Errorf("materialize command must be run as root")
			}
			return nil
		},
		Action: func(c *cli.Context) error {
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
}
