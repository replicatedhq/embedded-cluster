package main

import (
	"fmt"

	"github.com/urfave/cli/v2"

	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/goods"
)

var materialiseCommand = &cli.Command{
	Name:      "materialise",
	Usage:     "Materialise embedded assets into a directory",
	Hidden:    true,
	UsageText: fmt.Sprintf("%s materialise <dir>", defaults.BinaryName()),
	Before: func(c *cli.Context) error {
		if len(c.Args().Slice()) != 1 {
			return fmt.Errorf("materialise command requires exactly 1 argument")
		}
		return nil
	},
	Action: func(c *cli.Context) error {
		dst := c.Args().First()
		defaults.GlobalProvider = defaults.NewProvider("")
		defaults.GlobalProvider.Home = dst
		if err := goods.Materialize(); err != nil {
			return fmt.Errorf("unable to materialise: %v", err)
		}
		fmt.Printf("Materialising embedded binaries into %s\n", dst)
		return nil
	},
}
