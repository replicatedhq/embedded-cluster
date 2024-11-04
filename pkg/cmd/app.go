package cmd

import (
	"fmt"

	"github.com/urfave/cli/v2"
)

func NewApp(name string) *cli.App {
	return &cli.App{
		Name:    name,
		Usage:   fmt.Sprintf("Install and manage %s", name),
		Suggest: true,
		Commands: []*cli.Command{
			installCommand(),
			shellCommand(),
			nodeCommands,
			versionCommand,
			joinCommand,
			resetCommand(),
			materializeCommand(),
			updateCommand(),
			restoreCommand(),
			adminConsoleCommand(),
			supportBundleCommand(),
		},
	}
}
