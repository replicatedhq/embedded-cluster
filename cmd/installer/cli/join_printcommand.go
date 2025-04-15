package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/replicatedhq/embedded-cluster/pkg/kotsadm"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/spf13/cobra"
)

func JoinPrintCommandCmd(ctx context.Context, name string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "print-command",
		Short: fmt.Sprintf("Print controller join command for %s", name),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := release.GetEmbeddedClusterConfig()
			controllerRoleName := "controller"
			if cfg != nil && cfg.Spec.Roles.Controller.Name != "" {
				controllerRoleName = cfg.Spec.Roles.Controller.Name
			}

			jcmd, err := getJoinCommand(cmd.Context(), []string{controllerRoleName})
			if err != nil {
				return fmt.Errorf("unable to get join command: %w", err)
			}
			fmt.Println(jcmd)
			return nil
		},
	}

	return cmd
}

// getJoinCommand makes a request to the kots API to get a join command for the provided set of roles
func getJoinCommand(ctx context.Context, role []string) (string, error) {
	kcpath := runtimeconfig.PathToKubeConfig()
	if kcpath == "" {
		return "", fmt.Errorf("kubeconfig not found")
	}
	os.Setenv("KUBECONFIG", kcpath)

	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return "", fmt.Errorf("unable to get kube client: %w", err)
	}

	return kotsadm.GetJoinCommand(ctx, kcli, role)
}
