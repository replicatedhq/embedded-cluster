package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/replicatedhq/embedded-cluster/cmd/installer/cli/kotscli"
	rcutil "github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig/util"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func AdminConsoleResetPasswordCmd(ctx context.Context, name string) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "reset-password",
		Short:         fmt.Sprintf("Reset the %s Admin Console password", name),
		SilenceErrors: true,
		SilenceUsage:  true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if os.Getuid() != 0 {
				return fmt.Errorf("reset-password command must be run as root")
			}
			if len(args) != 1 {
				return fmt.Errorf("expected admin console password as argument")
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := rcutil.InitRuntimeConfigFromCluster(ctx); err != nil {
				return fmt.Errorf("failed to init runtime config from cluster: %w", err)
			}

			password := args[0]
			if !validateAdminConsolePassword(password, password) {
				return ErrNothingElseToAdd
			}

			if err := kotscli.ResetPassword(password); err != nil {
				return err
			}

			logrus.Info("Admin Console password reset successfully")
			return nil
		},
	}

	return cmd
}
