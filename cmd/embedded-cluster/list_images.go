package main

import (
	"fmt"

	"github.com/urfave/cli/v2"
)

var listImagesCommand = &cli.Command{
	Name:   "list-images",
	Usage:  "List images embedded in the cluster",
	Hidden: true,
	Action: func(c *cli.Context) error {
		metadata, err := gatherVersionMetadata()
		if err != nil {
			return fmt.Errorf("failed to gather version metadata: %w", err)
		}

		for _, image := range metadata.Images {
			fmt.Println(image)
		}

		return nil
	},
}
