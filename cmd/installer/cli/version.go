package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/replicatedhq/embedded-cluster/pkg/addons"
	"github.com/replicatedhq/embedded-cluster/pkg/config"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"github.com/spf13/cobra"
)

func VersionCmd(ctx context.Context, name string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: fmt.Sprintf("Show the %s component versions", name),
		RunE: func(cmd *cobra.Command, args []string) error {
			applierVersions, err := addons.NewApplier(addons.WithoutPrompt(), addons.OnlyDefaults(), addons.Quiet()).Versions(config.AdditionalCharts())
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

	cmd.AddCommand(VersionMetadataCmd(ctx, name))
	cmd.AddCommand(VersionEmbeddedDataCmd(ctx, name))
	cmd.AddCommand(VersionListImagesCmd(ctx, name))

	return cmd
}
