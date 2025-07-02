package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/replicatedhq/embedded-cluster/cmd/installer/kotscli"
	"github.com/replicatedhq/embedded-cluster/pkg/dryrun"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	rcutil "github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig/util"
	"github.com/spf13/cobra"
)

func JoinPrintCommandCmd(ctx context.Context, appTitle string) *cobra.Command {
	var rc runtimeconfig.RuntimeConfig

	cmd := &cobra.Command{
		Use:   "print-command",
		Short: fmt.Sprintf("Print controller join command for %s", appTitle),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// Skip root check if dryrun mode is enabled
			if !dryrun.Enabled() && os.Getuid() != 0 {
				return fmt.Errorf("print-command command must be run as root")
			}

			var err error
			rc, err = rcutil.GetRuntimeConfigFromCluster(ctx)
			if err != nil {
				return fmt.Errorf("failed to init runtime config from cluster: %w", err)
			}

			os.Setenv("TMPDIR", rc.EmbeddedClusterTmpSubDir())

			return nil
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			rc.Cleanup()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			jcmd, err := kotscli.GetJoinCommand(cmd.Context(), rc)
			if err != nil {
				return fmt.Errorf("unable to get join command: %w", err)
			}
			fmt.Println(jcmd)
			return nil
		},
	}

	return cmd
}
