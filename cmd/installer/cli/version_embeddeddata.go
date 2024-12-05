package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/spf13/cobra"
)

func VersionEmbeddedDataCmd(ctx context.Context, name string) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "embedded-data",
		Short:         "Read the application data embedded in the cluster",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
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

	return cmd
}
