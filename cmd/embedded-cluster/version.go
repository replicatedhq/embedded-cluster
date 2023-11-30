package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jedib0t/go-pretty/table"
	"github.com/urfave/cli/v2"

	"github.com/replicatedhq/embedded-cluster/pkg/addons"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
)

var versionCommand = &cli.Command{
	Name:  "version",
	Usage: fmt.Sprintf("Shows the %s installer version", defaults.BinaryName()),
	Flags: []cli.Flag{
		&cli.BoolFlag{Name: "json", Usage: "Output in JSON format", Value: false},
	},
	Action: func(c *cli.Context) error {
		opts := []addons.Option{addons.Quiet(), addons.WithoutPrompt()}
		versions, err := addons.NewApplier(opts...).Versions()
		if err != nil {
			return fmt.Errorf("unable to get versions: %w", err)
		}
		versions["Installer"] = defaults.Version
		versions["Kubernetes"] = defaults.K0sVersion
		if c.Bool("json") {
			data, err := json.MarshalIndent(versions, "", "\t")
			if err != nil {
				return fmt.Errorf("unable to marshal versions: %w", err)
			}
			fmt.Println(string(data))
			return nil
		}
		writer := table.NewWriter()
		writer.AppendHeader(table.Row{"component", "version"})
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
