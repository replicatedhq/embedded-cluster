package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/replicatedhq/embedded-cluster/pkg/addons"
	"github.com/replicatedhq/embedded-cluster/pkg/extensions"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"github.com/spf13/cobra"
)

func VersionCmd(ctx context.Context, appSlug string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: fmt.Sprintf("Show the %s component versions", appSlug),
		RunE: func(cmd *cobra.Command, args []string) error {
			writer := table.NewWriter()
			writer.AppendHeader(table.Row{"component", "version"})
			channelRelease := release.GetChannelRelease()
			if channelRelease != nil {
				writer.AppendRow(table.Row{runtimeconfig.BinaryName(), channelRelease.VersionLabel})
			}
			writer.AppendRow(table.Row{"Installer", versions.Version})
			writer.AppendRow(table.Row{"Kubernetes", versions.K0sVersion})

			versionsMap := map[string]string{}
			for k, v := range addons.Versions() {
				versionsMap[k] = v
			}
			for k, v := range extensions.Versions() {
				versionsMap[k] = v
			}

			keys := []string{}
			for k := range versionsMap {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			for _, k := range keys {
				version := versionsMap[k]
				if !strings.HasPrefix(version, "v") {
					version = fmt.Sprintf("v%s", version)
				}
				writer.AppendRow(table.Row{k, version})
			}

			fmt.Printf("%s\n", writer.Render())
			return nil
		},
	}

	cmd.AddCommand(VersionMetadataCmd(ctx))
	cmd.AddCommand(VersionEmbeddedDataCmd(ctx))
	cmd.AddCommand(VersionListImagesCmd(ctx))

	return cmd
}
