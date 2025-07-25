package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/replicatedhq/embedded-cluster/cmd/installer/kotscli"
	"github.com/replicatedhq/embedded-cluster/pkg-new/constants"
	"github.com/replicatedhq/embedded-cluster/pkg/airgap"
	"github.com/replicatedhq/embedded-cluster/pkg/dryrun"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	rcutil "github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig/util"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func UpdateCmd(ctx context.Context, name string) *cobra.Command {
	var airgapBundle string
	var rc runtimeconfig.RuntimeConfig

	cmd := &cobra.Command{
		Use:   "update",
		Short: fmt.Sprintf("Update %s with a new air gap bundle", name),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// Skip root check if dryrun mode is enabled
			if !dryrun.Enabled() && os.Getuid() != 0 {
				return fmt.Errorf("update command must be run as root")
			}

			var err error
			rc, err = rcutil.GetRuntimeConfigFromCluster(ctx)
			if err != nil {
				return fmt.Errorf("failed to init runtime config from cluster: %w", err)
			}

			os.Setenv("KUBECONFIG", rc.PathToKubeConfig())
			os.Setenv("TMPDIR", rc.EmbeddedClusterTmpSubDir())

			return nil
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			rc.Cleanup()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if airgapBundle != "" {
				logrus.Debugf("checking airgap bundle matches binary")

				// read file from path
				metadata, err := airgap.AirgapMetadataFromPath(airgapBundle)
				if err != nil {
					return fmt.Errorf("failed to get airgap metadata: %w", err)
				}
				airgapInfo := metadata.AirgapInfo

				if err := checkAirgapMatches(airgapInfo); err != nil {
					return err // we want the user to see the error message without a prefix
				}
			}

			rel := release.GetChannelRelease()
			if rel == nil {
				return fmt.Errorf("no channel release found")
			}

			if err := kotscli.AirgapUpdate(kotscli.AirgapUpdateOptions{
				RuntimeConfig: rc,
				AppSlug:       rel.AppSlug,
				Namespace:     constants.KotsadmNamespace,
				AirgapBundle:  airgapBundle,
			}); err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&airgapBundle, "airgap-bundle", "", "Path to the air gap bundle. If set, the installation will complete without internet access.")
	cmd.MarkFlagRequired("airgap-bundle")

	return cmd
}
