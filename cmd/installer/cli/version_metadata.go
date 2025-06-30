package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/pkg-new/metadata"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/spf13/cobra"
)

func VersionMetadataCmd(ctx context.Context) *cobra.Command {
	var (
		omitReleaseMetadata bool
	)

	cmd := &cobra.Command{
		Use:   "metadata",
		Short: "Print metadata about this release",
		RunE: func(cmd *cobra.Command, args []string) error {
			var channelRelease *release.ChannelRelease
			if !omitReleaseMetadata {
				channelRelease = release.GetChannelRelease()
			}
			metadata, err := metadata.GatherVersionMetadata(channelRelease)
			if err != nil {
				return fmt.Errorf("failed to gather version metadata: %w", err)
			}
			data, err := json.MarshalIndent(metadata, "", "\t")
			if err != nil {
				return fmt.Errorf("failed to marshal versions: %w", err)
			}
			fmt.Println(string(data))
			return nil
		},
	}

	cmd.Flags().BoolVar(&omitReleaseMetadata, "omit-release-metadata", false, "Omit the release metadata from the output")

	return cmd
}
