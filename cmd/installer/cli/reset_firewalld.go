package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	rcutil "github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig/util"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func ResetFirewalldCmd(ctx context.Context, name string) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "firewalld",
		Short:  "Remove %s firewalld configuration from the current node",
		Hidden: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if os.Getuid() != 0 {
				return fmt.Errorf("reset firewalld command must be run as root")
			}

			rcutil.InitBestRuntimeConfig(cmd.Context())

			os.Setenv("KUBECONFIG", runtimeconfig.PathToKubeConfig())
			os.Setenv("TMPDIR", runtimeconfig.EmbeddedClusterTmpSubDir())

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			err := resetFirewalld(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to reset firewalld: %w", err)
			}

			logrus.Infof("Firewalld reset successfully")

			return nil
		},
	}

	return cmd
}
