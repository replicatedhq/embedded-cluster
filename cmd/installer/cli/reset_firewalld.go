package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/replicatedhq/embedded-cluster/pkg-new/hostutils"
	"github.com/replicatedhq/embedded-cluster/pkg/dryrun"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	rcutil "github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig/util"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func ResetFirewalldCmd(ctx context.Context, name string) *cobra.Command {
	var rc runtimeconfig.RuntimeConfig

	cmd := &cobra.Command{
		Use:    "firewalld",
		Short:  "Remove %s firewalld configuration from the current node",
		Hidden: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// Skip root check if dryrun mode is enabled
			if !dryrun.Enabled() && os.Getuid() != 0 {
				return fmt.Errorf("reset firewalld command must be run as root")
			}

			rc = rcutil.InitBestRuntimeConfig(cmd.Context())

			_ = rc.SetEnv()

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			err := hostutils.ResetFirewalld(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to reset firewalld: %w", err)
			}

			logrus.Infof("Firewalld reset successfully")

			return nil
		},
	}

	return cmd
}
