package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/jedib0t/go-pretty/table"
	"github.com/urfave/cli/v2"

	"github.com/replicatedhq/embedded-cluster/pkg/addons"
	"github.com/replicatedhq/embedded-cluster/pkg/config"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"github.com/replicatedhq/embedded-cluster/sdk/defaults"
)

var versionCommand = &cli.Command{
	Name:  "version",
	Usage: fmt.Sprintf("Show the %s component versions", defaults.BinaryName()),
	Subcommands: []*cli.Command{
		metadataCommand,
		embeddedDataCommand,
		listImagesCommand,
	},
	Action: func(c *cli.Context) error {
		opts := []addons.Option{addons.Quiet(), addons.WithoutPrompt()}
		applierVersions, err := addons.NewApplier(opts...).Versions(config.AdditionalCharts())
		if err != nil {
			return fmt.Errorf("unable to get versions: %w", err)
		}
		writer := table.NewWriter()
		writer.AppendHeader(table.Row{"component", "version"})
		channelRelease, err := release.GetChannelRelease()
		if err == nil && channelRelease != nil {
			writer.AppendRow(table.Row{defaults.BinaryName(), channelRelease.VersionLabel})
		}
		writer.AppendRow(table.Row{"Installer", versions.Version})
		writer.AppendRow(table.Row{"Kubernetes", versions.K0sVersion})

		keys := []string{}
		for k := range applierVersions {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			version := applierVersions[k]
			if !strings.HasPrefix(version, "v") {
				version = fmt.Sprintf("v%s", version)
			}
			writer.AppendRow(table.Row{k, version})
		}
		fmt.Printf("%s\n", writer.Render())
		return nil
	},
}

var embeddedDataCommand = &cli.Command{
	Name:   "embedded-data",
	Usage:  "Read the application data embedded in the cluster",
	Hidden: true,
	Action: func(context *cli.Context) error {
		// Application
		app, err := release.GetApplication()
		if err != nil {
			return fmt.Errorf("failed to get embedded application: %w", err)
		}
		fmt.Printf("Application:\n%s\n\n", string(app))

		// Embedded Cluster Config
		cfg, err := release.GetEmbeddedClusterConfig()
		if err != nil {
			return fmt.Errorf("failed to get embedded cluster config: %w", err)
		}
		if cfg != nil {
			cfgJson, err := json.MarshalIndent(cfg, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal embedded cluster config: %w", err)
			}

			fmt.Printf("Embedded Cluster Config:\n%s\n\n", string(cfgJson))
		}

		// Channel Release
		rel, err := release.GetChannelRelease()
		if err != nil {
			return fmt.Errorf("failed to get release: %w", err)
		}
		if rel != nil {
			relJson, err := json.MarshalIndent(rel, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal release: %w", err)
			}

			fmt.Printf("Release:\n%s\n\n", string(relJson))
		}

		// Host Preflights
		pre, err := release.GetHostPreflights()
		if err != nil {
			return fmt.Errorf("failed to get host preflights: %w", err)
		}
		if pre != nil {
			preJson, err := json.MarshalIndent(pre, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal host preflights: %w", err)
			}

			fmt.Printf("Preflights:\n%s\n\n", string(preJson))
		}

		return nil
	},
}
