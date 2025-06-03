package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/replicatedhq/embedded-cluster/cmd/installer/kotscli"
	"github.com/replicatedhq/embedded-cluster/pkg/prompts"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	rcutil "github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig/util"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func AdminConsoleResetPasswordCmd(ctx context.Context, name string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reset-password [password]",
		Args:  cobra.MaximumNArgs(1),
		Short: fmt.Sprintf("Reset the %s Admin Console password. If no password is provided, you will be prompted to enter a new one.", name),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if os.Getuid() != 0 {
				return fmt.Errorf("reset-password command must be run as root")
			}

			if err := rcutil.InitRuntimeConfigFromCluster(cmd.Context()); err != nil {
				return fmt.Errorf("failed to init runtime config from cluster: %w", err)
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var password string
			if len(args) == 1 {
				password = args[0]
			} else {
				maxTries := 3
				for i := 0; i < maxTries; i++ {
					promptA, err := prompts.New().Password(fmt.Sprintf("Set the Admin Console password (minimum %d characters):", minAdminPasswordLength))
					if err != nil {
						return fmt.Errorf("failed to get password: %w", err)
					}
					promptB, err := prompts.New().Password("Confirm the Admin Console password:")
					if err != nil {
						return fmt.Errorf("failed to get password confirmation: %w", err)
					}

					if validateAdminConsolePassword(promptA, promptB) {
						password = promptA
						break
					}
				}
				if password == "" {
					return NewErrorNothingElseToAdd(errors.New("password is not valid"))
				}
			}

			if err := kotscli.ResetPassword(runtimeconfig.EmbeddedClusterDataDirectory(), password); err != nil {
				return err
			}

			logrus.Info("Admin Console password reset successfully")
			return nil
		},
	}

	return cmd
}
