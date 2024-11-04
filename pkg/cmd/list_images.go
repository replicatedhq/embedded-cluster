package cmd

import (
	"fmt"

	"github.com/replicatedhq/embedded-cluster/pkg/config"
	"github.com/urfave/cli/v2"
)

var listImagesCommand = &cli.Command{
	Name:   "list-images",
	Usage:  "List images embedded in the cluster",
	Hidden: true,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "omit-release-metadata",
			Usage: "Omit the release metadata from the output",
		},
	},
	Action: func(c *cli.Context) error {
		k0sCfg := config.RenderK0sConfig()
		metadata, err := gatherVersionMetadata(k0sCfg, !c.Bool("omit-release-metadata"))
		if err != nil {
			return fmt.Errorf("failed to gather version metadata: %w", err)
		}

		for _, image := range metadata.Images {
			fmt.Println(image)
		}

		return nil
	},
}
