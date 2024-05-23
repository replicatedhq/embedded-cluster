package main

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/replicatedhq/embedded-cluster/pkg/goods"
)

var materializeCommand = &cli.Command{
	Name:   "materialize",
	Usage:  "Materialize embedded assets on /var/lib/embedded-cluster",
	Hidden: true,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "basedir",
			Usage: "Base directory to materialize assets",
			Value: "",
		},
	},
	Before: func(c *cli.Context) error {
		if os.Getuid() != 0 {
			return fmt.Errorf("materialize command must be run as root")
		}
		return nil
	},
	Action: func(c *cli.Context) error {
		materializer := goods.NewMaterializer(c.String("basedir"))
		if err := materializer.Materialize(); err != nil {
			return fmt.Errorf("unable to materialize: %v", err)
		}
		return nil
	},
}
