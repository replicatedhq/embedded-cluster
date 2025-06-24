package cli

import (
	"context"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/pkg-new/metadata"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/spf13/cobra"
)

func VersionListImagesCmd(ctx context.Context, name string) *cobra.Command {
	var (
		omitReleaseMetadata bool
	)

	cmd := &cobra.Command{
		Use:   "list-images",
		Short: "List images embedded in the cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			var channelRelease *release.ChannelRelease
			if !omitReleaseMetadata {
				channelRelease = release.GetChannelRelease()
			}
			metadata, err := metadata.GatherVersionMetadata(channelRelease)
			if err != nil {
				return fmt.Errorf("failed to gather version metadata: %w", err)
			}

			for _, image := range metadata.Images {
				fmt.Println(image)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&omitReleaseMetadata, "omit-release-metadata", false, "Omit the release metadata from the output")

	return cmd
}
