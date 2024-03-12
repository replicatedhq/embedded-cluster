package main

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/goods"
)

var materializeCommand = &cli.Command{
	Name:      "materialize",
	Usage:     "Materialize embedded assets into a directory",
	Hidden:    true,
	UsageText: fmt.Sprintf("%s materialize <dir>", defaults.BinaryName()),
	Before: func(c *cli.Context) error {
		if os.Getuid() != 0 {
			return fmt.Errorf("materialize command must be run as root")
		}
		if len(c.Args().Slice()) != 1 {
			return fmt.Errorf("materialize command requires exactly 1 argument")
		}
		return nil
	},
	Action: func(c *cli.Context) error {
		dst := c.Args().First()
		defaults.GlobalProvider = defaults.NewProvider("")
		defaults.GlobalProvider.Home = dst
		if err := goods.Materialize(); err != nil {
			return fmt.Errorf("unable to materialize: %v", err)
		}
		fmt.Printf("Materialising embedded binaries into %s\n", dst)
		return nil
	},
}
