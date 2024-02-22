package main

import (
	"github.com/urfave/cli/v2"
)

var nodeCommands = &cli.Command{
	Name:   "node",
	Usage:  "Manage cluster nodes",
	Hidden: true, // this has been replaced by top-level commands
	Subcommands: []*cli.Command{
		joinCommand,
		uninstallCommand,
	},
}
