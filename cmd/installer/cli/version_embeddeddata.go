package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/spf13/cobra"
)

func VersionEmbeddedDataCmd(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "embedded-data",
		Short: "Read the application data embedded in the cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Application
			app := release.GetApplication()
			appJson, err := json.MarshalIndent(app, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal application: %w", err)
			}
			fmt.Printf("Application:\n%s\n\n", string(appJson))

			// Embedded Cluster Config
			cfg := release.GetEmbeddedClusterConfig()
			cfgJson, err := json.MarshalIndent(cfg, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal embedded cluster config: %w", err)
			}
			fmt.Printf("Embedded Cluster Config:\n%s\n\n", string(cfgJson))

			// Channel Release
			rel := release.GetChannelRelease()
			relJson, err := json.MarshalIndent(rel, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal release: %w", err)
			}
			fmt.Printf("Release:\n%s\n\n", string(relJson))

			// Host Preflights
			pre := release.GetHostPreflights()
			preJson, err := json.MarshalIndent(pre, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal host preflights: %w", err)
			}
			fmt.Printf("Preflights:\n%s\n\n", string(preJson))

			return nil
		},
	}

	return cmd
}
