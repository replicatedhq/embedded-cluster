package cli

import (
	"context"
	"fmt"
	"os"

	cmdutil "github.com/replicatedhq/embedded-cluster/pkg/cmd/util"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/k0s"
	"github.com/replicatedhq/embedded-cluster/pkg/kotscli"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func AdminConsoleResetPasswordCmd(ctx context.Context, name string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reset-password",
		Short: fmt.Sprintf("Reset the %s Admin Console password", name),
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
			provider, err := getProviderFromCluster(cmd.Context())
			if err != nil {
				return err
			}

			password := args[0]
			if !validateAdminConsolePassword(password, password) {
				return ErrNothingElseToAdd
			}

			if err := kotscli.ResetPassword(provider, password); err != nil {
				return err
			}

			logrus.Info("Admin Console password reset successfully")
			return nil
		},
	}

	return cmd
}

// getProviderFromCluster finds the kubeconfig and discovers the provider from the cluster. If this
// is a prior version of EC, we will have to fall back to the filesystem.
func getProviderFromCluster(ctx context.Context) (*defaults.Provider, error) {
	status, err := k0s.GetStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to get k0s status: %w", err)
	}

	kubeconfigPath := status.Vars.AdminKubeConfigPath

	os.Setenv("KUBECONFIG", kubeconfigPath)

	// Discover the provider from the cluster
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return nil, fmt.Errorf("unable to create kube client: %w", err)
	}

	provider, err := cmdutil.NewProviderFromCluster(ctx, kcli)
	if err != nil {
		return nil, fmt.Errorf("unable to get config from cluster: %w", err)
	}
	return provider, nil
}
