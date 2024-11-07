package cmd

import (
	"fmt"

	"github.com/replicatedhq/embedded-cluster/pkg/dryrun"
	"github.com/urfave/cli/v2"
)

func NewApp(name string) *cli.App {
	return &cli.App{
		Name:    name,
		Usage:   fmt.Sprintf("Install and manage %s", name),
		Suggest: true,
		Before: func(c *cli.Context) error {
			if dryrun.Enabled() {
				dryrun.RecordFlags(c)
			}
			return nil
		},
		After: func(c *cli.Context) error {
			if dryrun.Enabled() {
				if err := dryrun.Dump(); err != nil {
					return fmt.Errorf("unable to dump dry run info: %w", err)
				}
			}
			return nil
		},
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
