package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jedib0t/go-pretty/table"
	"github.com/urfave/cli/v2"

	"github.com/replicatedhq/embedded-cluster/pkg/addons"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/goods"
)

var versionCommand = &cli.Command{
	Name:        "version",
	Usage:       fmt.Sprintf("Shows the %s installer version", defaults.BinaryName()),
	Subcommands: []*cli.Command{metadataCommand},
	Action: func(c *cli.Context) error {
		opts := []addons.Option{addons.Quiet(), addons.WithoutPrompt()}
		versions, err := addons.NewApplier(opts...).Versions()
		if err != nil {
			return fmt.Errorf("unable to get versions: %w", err)
		}
		writer := table.NewWriter()
		writer.AppendHeader(table.Row{"component", "version"})
		writer.AppendRow(table.Row{"Installer", defaults.Version})
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

// ReleaseMetadata holds the metadata about a specific release, including addons and
// their versions.
type ReleaseMetadata struct {
	Versions     map[string]string
	K0sSHA       string
	K0sBinaryURL string
}

var metadataCommand = &cli.Command{
	Name:   "metadata",
	Usage:  "Print metadata about this release",
	Hidden: true,
	Action: func(c *cli.Context) error {
		opts := []addons.Option{addons.Quiet(), addons.WithoutPrompt()}
		versions, err := addons.NewApplier(opts...).Versions()
		if err != nil {
			return fmt.Errorf("unable to get versions: %w", err)
		}
		versions["Kubernetes"] = defaults.K0sVersion
		versions["Installer"] = defaults.Version
		sha, err := goods.K0sBinarySHA256()
		if err != nil {
			return fmt.Errorf("unable to get k0s binary sha256: %w", err)
		}
		meta := ReleaseMetadata{
			Versions:     versions,
			K0sSHA:       sha,
			K0sBinaryURL: defaults.K0sBinaryURL,
		}
		data, err := json.MarshalIndent(meta, "", "\t")
		if err != nil {
			return fmt.Errorf("unable to marshal versions: %w", err)
		}
		fmt.Println(string(data))
		return nil
	},
}
