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
	Before: func(c *cli.Context) error {
		if os.Getuid() != 0 {
			return fmt.Errorf("materialize command must be run as root")
		}
		return nil
	},
	Action: func(c *cli.Context) error {
		if err := goods.Materialize(); err != nil {
			return fmt.Errorf("unable to materialize: %v", err)
		}
		return nil
	},
}
