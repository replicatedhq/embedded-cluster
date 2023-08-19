package main

import (
	"fmt"
	"strings"

	"github.com/jedib0t/go-pretty/table"
	"github.com/urfave/cli/v2"

	"github.com/replicatedhq/helmvm/pkg/addons"
	"github.com/replicatedhq/helmvm/pkg/defaults"
)

var Version = "v0.0.0"

var versionCommand = &cli.Command{
	Name:  "version",
	Usage: fmt.Sprintf("Shows the %s installer version", defaults.BinaryName()),
	Action: func(c *cli.Context) error {
		applier, err := addons.NewApplier(true, false)
		if err != nil {
			return fmt.Errorf("unable to create applier: %w", err)
		}
		versions, err := applier.Versions()
		if err != nil {
			return fmt.Errorf("unable to get versions: %w", err)
		}
		writer := table.NewWriter()
		writer.AppendHeader(table.Row{"component", "version"})
		writer.AppendRow(table.Row{"Installer", Version})
		writer.AppendRow(table.Row{"Kubernetes", defaults.K0sVersion})
		for name, version := range versions {
			if !strings.HasPrefix(version, "v") {
				version = fmt.Sprintf("v%s", version)
			}
			writer.AppendRow(table.Row{name, version})
		}
		fmt.Printf("%s\n", writer.Render())
		return nil
	},
}
