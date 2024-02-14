package main

import (
	"github.com/urfave/cli/v2"
)

var nodeCommands = &cli.Command{
	Name:  "node",
	Usage: "Manage cluster nodes",
	Subcommands: []*cli.Command{
		joinCommand,
		resetCommand,
	},
}
