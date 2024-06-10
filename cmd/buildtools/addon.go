package main

import (
	"github.com/urfave/cli/v2"
)

var addonCommand = &cli.Command{
	Name:  "update",
	Usage: "Manage the embedded cluster addons",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "force",
			Usage: "Force the update of the addons",
		},
	},
	Subcommands: []*cli.Command{
		updateAddonCommand,
	},
}

var updateAddonCommand = &cli.Command{
	Name:  "addon",
	Usage: "Update an embedded cluster addon by copying the chart to the Replicated registry and setting the version in the Makefile",
	Subcommands: []*cli.Command{
		updateOpenEBSAddonCommand,
		updateSeaweedFSAddonCommand,
		updateRegistryAddonCommand,
		updateVeleroAddonCommand,
	},
}
