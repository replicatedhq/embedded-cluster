package main

import (
	"github.com/urfave/cli/v2"
)

var updateCommand = &cli.Command{
	Name:  "update",
	Usage: "Manage the embedded cluster components",
	Subcommands: []*cli.Command{
		updateAddonCommand,
		updateImagesCommand,
	},
}

var updateAddonCommand = &cli.Command{
	Name:  "addon",
	Usage: "Update an embedded cluster addon by copying the chart to the Replicated registry and setting the version in the Makefile",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "force",
			Usage: "Pushes the addon chart even if no new version was found",
		},
	},
	Subcommands: []*cli.Command{
		updateOpenEBSAddonCommand,
		updateSeaweedFSAddonCommand,
		updateRegistryAddonCommand,
		updateVeleroAddonCommand,
		updateOperatorAddonCommand,
		updateAdminConsoleAddonCommand,
	},
}

var updateImagesCommand = &cli.Command{
	Name:  "images",
	Usage: "Update embedded cluster images",
	Subcommands: []*cli.Command{
		updateK0sImagesCommand,
	},
}
