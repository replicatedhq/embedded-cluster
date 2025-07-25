package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/replicatedhq/embedded-cluster/cmd/installer/kotscli"
	"github.com/replicatedhq/embedded-cluster/pkg-new/constants"
	"github.com/replicatedhq/embedded-cluster/pkg/airgap"
	"github.com/replicatedhq/embedded-cluster/pkg/dryrun"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	rcutil "github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig/util"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func UpdateCmd(ctx context.Context, appSlug, appTitle string) *cobra.Command {
	var airgapBundle string
	var rc runtimeconfig.RuntimeConfig

	cmd := &cobra.Command{
		Use:   "update",
		Short: fmt.Sprintf("Update %s with a new air gap bundle", appTitle),
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

			kcli, err := kubeutils.KubeClient()
			if err != nil {
				return fmt.Errorf("failed to create kubernetes client: %w", err)
			}

			in, err := kubeutils.GetLatestInstallation(ctx, kcli)
			if err != nil {
				return fmt.Errorf("failed to get latest installation: %w", err)
			}

			if err := kotscli.AirgapUpdate(kotscli.AirgapUpdateOptions{
				RuntimeConfig: rc,
				AppSlug:       appSlug,
				Namespace:     constants.KotsadmNamespace,
				AirgapBundle:  airgapBundle,
				ClusterID:     in.Spec.ClusterID,
			}); err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&airgapBundle, "airgap-bundle", "", "Path to the air gap bundle. If set, the installation will complete without internet access.")
	mustMarkFlagRequired(cmd.Flags(), "airgap-bundle")

	return cmd
}
