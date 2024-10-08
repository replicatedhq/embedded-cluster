package main

import (
	"fmt"
	"os"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/config"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/urfave/cli/v2"
)

func listImagesCommand() *cli.Command {
	runtimeConfig := ecv1beta1.GetDefaultRuntimeConfig()

	return &cli.Command{
		Name:   "list-images",
		Usage:  "List images embedded in the cluster",
		Hidden: true,
		Flags: []cli.Flag{
			getDataDirFlag(runtimeConfig),
		},
		Before: func(c *cli.Context) error {
			if os.Getuid() != 0 {
				return fmt.Errorf("version metadata command must be run as root")
			}
			return nil
		},
		Action: func(c *cli.Context) error {
			provider := defaults.NewProviderFromRuntimeConfig(runtimeConfig)
			os.Setenv("TMPDIR", provider.EmbeddedClusterTmpSubDir())

			k0sCfg := config.RenderK0sConfig()
			metadata, err := gatherVersionMetadata(provider, k0sCfg)
			if err != nil {
				return fmt.Errorf("failed to gather version metadata: %w", err)
			}

			for _, image := range metadata.Images {
				fmt.Println(image)
			}

			tryRemoveTmpDirContents(provider)

			return nil
		},
	}
}
