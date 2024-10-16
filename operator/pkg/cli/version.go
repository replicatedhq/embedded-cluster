package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/replicatedhq/embedded-cluster/pkg/addons"
	"github.com/replicatedhq/embedded-cluster/pkg/config"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"github.com/spf13/cobra"
)

// VersionCmd returns a cobra command for listing versions of embedded cluster components
func VersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "version",
		Short:        "list versions",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			applierVersions, err := addons.NewApplier(
				addons.WithoutPrompt(),
				addons.OnlyDefaults(),
				addons.Quiet(),
			).Versions(config.AdditionalCharts())
			if err != nil {
				return fmt.Errorf("unable to get versions: %w", err)
			}
			writer := table.NewWriter()
			writer.AppendHeader(table.Row{"component", "version"})
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

	return cmd
}
