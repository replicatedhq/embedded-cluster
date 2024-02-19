package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/jedib0t/go-pretty/table"
	"github.com/urfave/cli/v2"

	k0sconfig "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons"
	"github.com/replicatedhq/embedded-cluster/pkg/config"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/goods"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
)

var versionCommand = &cli.Command{
	Name:        "version",
	Usage:       fmt.Sprintf("Show the %s component versions", defaults.BinaryName()),
	Subcommands: []*cli.Command{metadataCommand, embeddedDataCommand},
	Action: func(c *cli.Context) error {
		opts := []addons.Option{addons.Quiet(), addons.WithoutPrompt()}
		versions, err := addons.NewApplier(opts...).Versions(config.AdditionalCharts())
		if err != nil {
			return fmt.Errorf("unable to get versions: %w", err)
		}
		writer := table.NewWriter()
		writer.AppendHeader(table.Row{"component", "version"})
		channelRelease, err := release.GetChannelRelease()
		if err == nil && channelRelease != nil {
			writer.AppendRow(table.Row{defaults.BinaryName(), channelRelease.VersionLabel})
		}
		writer.AppendRow(table.Row{"Installer", defaults.Version})
		writer.AppendRow(table.Row{"Kubernetes", defaults.K0sVersion})

		keys := []string{}
		for k := range versions {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			version := versions[k]
			if !strings.HasPrefix(version, "v") {
				version = fmt.Sprintf("v%s", version)
			}
			writer.AppendRow(table.Row{k, version})
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
	Configs      k0sconfig.HelmExtensions
	Protected    map[string][]string
}

var metadataCommand = &cli.Command{
	Name:   "metadata",
	Usage:  "Print metadata about this release",
	Hidden: true,
	Action: func(c *cli.Context) error {
		opts := []addons.Option{addons.Quiet(), addons.WithoutPrompt(), addons.OnlyDefaults()}
		versions, err := addons.NewApplier(opts...).Versions(config.AdditionalCharts())
		if err != nil {
			return fmt.Errorf("unable to get versions: %w", err)
		}
		versions["Kubernetes"] = defaults.K0sVersion
		versions["Installer"] = defaults.Version
		channelRelease, err := release.GetChannelRelease()
		if err == nil && channelRelease != nil {
			versions[defaults.BinaryName()] = channelRelease.VersionLabel
		}
		sha, err := goods.K0sBinarySHA256()
		if err != nil {
			return fmt.Errorf("unable to get k0s binary sha256: %w", err)
		}
		meta := ReleaseMetadata{
			Versions:     versions,
			K0sSHA:       sha,
			K0sBinaryURL: defaults.K0sBinaryURL,
		}
		applier := addons.NewApplier(opts...)
		chtconfig, repconfig, err := applier.GenerateHelmConfigs(config.AdditionalCharts(), config.AdditionalRepositories())
		if err != nil {
			return fmt.Errorf("unable to apply addons: %w", err)
		}
		meta.Configs = k0sconfig.HelmExtensions{
			ConcurrencyLevel: 1,
			Charts:           chtconfig,
			Repositories:     repconfig,
		}
		protectedFields, err := applier.ProtectedFields()
		if err != nil {
			return fmt.Errorf("unable to get protected fields: %w", err)
		}
		meta.Protected = protectedFields
		data, err := json.MarshalIndent(meta, "", "\t")
		if err != nil {
			return fmt.Errorf("unable to marshal versions: %w", err)
		}
		fmt.Println(string(data))
		return nil
	},
}

var embeddedDataCommand = &cli.Command{
	Name:   "embedded-data",
	Usage:  "read the application data embedded in the cluster",
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
