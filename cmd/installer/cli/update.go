package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/replicatedhq/embedded-cluster/cmd/installer/cli/kotscli"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	rcutil "github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig/util"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func UpdateCmd(ctx context.Context, name string) *cobra.Command {
	var (
		airgapBundle string
	)

	cmd := &cobra.Command{
		Use:           "update",
		Short:         fmt.Sprintf("Update %s", name),
		SilenceErrors: true,
		SilenceUsage:  true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if os.Getuid() != 0 {
				return fmt.Errorf("update command must be run as root")
			}

			if err := rcutil.InitRuntimeConfigFromCluster(ctx); err != nil {
				return fmt.Errorf("failed to init runtime config from cluster: %w", err)
			}

			os.Setenv("KUBECONFIG", runtimeconfig.PathToKubeConfig())
			os.Setenv("TMPDIR", runtimeconfig.EmbeddedClusterTmpSubDir())

			return nil
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			runtimeconfig.Cleanup()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if airgapBundle != "" {
				logrus.Debugf("checking airgap bundle matches binary")
				if err := checkAirgapMatches(airgapBundle); err != nil {
					return err // we want the user to see the error message without a prefix
				}
			}

			rel, err := release.GetChannelRelease()
			if err != nil {
				return fmt.Errorf("unable to get channel release: %w", err)
			}
			if rel == nil {
				return fmt.Errorf("no channel release found")
			}

			if err := kotscli.AirgapUpdate(kotscli.AirgapUpdateOptions{
				AppSlug:      rel.AppSlug,
				Namespace:    runtimeconfig.KotsadmNamespace,
				AirgapBundle: airgapBundle,
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
