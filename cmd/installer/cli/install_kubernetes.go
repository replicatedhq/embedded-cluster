package cli

import (
	"context"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/spf13/cobra"
)

type InstallKubernetesCmdFlags struct {
	// TODO: add flags here
}

// InstallLinuxCmd returns a cobra command for installing the embedded cluster.
func InstallKubernetesCmd(ctx context.Context, name string) *cobra.Command {
	var flags InstallKubernetesCmdFlags

	ctx, cancel := context.WithCancel(ctx)
	rc := runtimeconfig.New(nil)

	cmd := &cobra.Command{
		Use:   "kubernetes",
		Short: fmt.Sprintf("kubernetes %s", name),
		PostRun: func(cmd *cobra.Command, args []string) {
			rc.Cleanup()
			cancel() // Cancel context when command completes
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: implement

			return nil
		},
	}

	if err := addInstallKubernetesFlags(cmd, &flags); err != nil {
		panic(err)
	}

	return cmd
}

func addInstallKubernetesFlags(cmd *cobra.Command, flags *InstallKubernetesCmdFlags) error {
	// TODO: add flags here
	return nil
}
