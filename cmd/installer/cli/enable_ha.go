package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/replicatedhq/embedded-cluster/pkg/addons"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	rcutil "github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig/util"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// EnableHACmd is the command for enabling HA mode.
func EnableHACmd(ctx context.Context, name string) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "enable-ha",
		Short:  fmt.Sprintf("Enable high availability for the %s cluster", name),
		Hidden: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if os.Getuid() != 0 {
				return fmt.Errorf("enable-ha command must be run as root")
			}

			rcutil.InitBestRuntimeConfig(cmd.Context())

			os.Setenv("KUBECONFIG", runtimeconfig.PathToKubeConfig())
			os.Setenv("TMPDIR", runtimeconfig.EmbeddedClusterTmpSubDir())

			return nil
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			runtimeconfig.Cleanup()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := runEnableHA(cmd.Context()); err != nil {
				return err
			}

			return nil
		},
	}

	return cmd
}

func runEnableHA(ctx context.Context) error {
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return fmt.Errorf("unable to get kube client: %w", err)
	}

	canEnableHA, reason, err := addons.CanEnableHA(ctx, kcli)
	if err != nil {
		return fmt.Errorf("unable to check if HA can be enabled: %w", err)
	}
	if !canEnableHA {
		logrus.Warnf("High availability cannot be enabled: %s", reason)
		return NewErrorNothingElseToAdd(fmt.Errorf("high availability cannot be enabled: %s", reason))
	}

	kclient, err := kubeutils.GetClientset()
	if err != nil {
		return fmt.Errorf("unable to create kubernetes client: %w", err)
	}

	in, err := kubeutils.GetLatestInstallation(ctx, kcli)
	if err != nil {
		return fmt.Errorf("unable to get latest installation: %w", err)
	}

	airgapChartsPath := ""
	if in.Spec.AirGap {
		airgapChartsPath = runtimeconfig.EmbeddedClusterChartsSubDir()
	}

	hcli, err := helm.NewClient(helm.HelmOptions{
		KubeConfig: runtimeconfig.PathToKubeConfig(),
		K0sVersion: versions.K0sVersion,
		AirgapPath: airgapChartsPath,
	})
	if err != nil {
		return fmt.Errorf("unable to create helm client: %w", err)
	}
	defer hcli.Close()

	return addons.EnableHA(ctx, kcli, kclient, hcli, in.Spec.AirGap, in.Spec.Network.ServiceCIDR, in.Spec.Proxy, in.Spec.Config)
}
