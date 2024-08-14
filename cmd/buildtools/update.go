package main

import (
	"github.com/urfave/cli/v2"
)

var updateCommand = &cli.Command{
	Name:  "update",
	Usage: "Manage the embedded cluster components",
	Subcommands: []*cli.Command{
		updateAddonCommand,
	},
}

var updateAddonCommand = &cli.Command{
	Name:  "addon",
	Usage: "Update an embedded cluster addon by copying the chart to the Replicated registry and setting the version in the Makefile",
	Subcommands: []*cli.Command{
		updateK0sAddonCommand,
		updateOpenEBSAddonCommand,
		updateSeaweedFSAddonCommand,
		updateRegistryAddonCommand,
		updateVeleroAddonCommand,
		updateOperatorAddonCommand,
		updateAdminConsoleAddonCommand,
	},
}
