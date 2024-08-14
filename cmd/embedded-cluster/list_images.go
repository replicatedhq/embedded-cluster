package main

import (
	"fmt"

	"github.com/replicatedhq/embedded-cluster/pkg/config"
	"github.com/urfave/cli/v2"
)

var listImagesCommand = &cli.Command{
	Name:   "list-images",
	Usage:  "List images embedded in the cluster",
	Hidden: true,
	Action: func(c *cli.Context) error {
		k0sCfg := config.RenderK0sConfig()

		metadata, err := gatherVersionMetadata(k0sCfg)
		if err != nil {
			return fmt.Errorf("failed to gather version metadata: %w", err)
		}

		for _, image := range metadata.Images {
			fmt.Println(image)
		}

		return nil
	},
}
