package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/replicatedhq/embedded-cluster/pkg/addons"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/prompts"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	rcutil "github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig/util"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type EnableHACmdFlags struct {
	assumeYes bool
}

// EnableHACmd is the command for enabling HA mode.
func EnableHACmd(ctx context.Context, name string) *cobra.Command {
	var flags EnableHACmdFlags

	cmd := &cobra.Command{
		Use:   "enable-ha",
		Short: fmt.Sprintf("Enable high availability for the %s cluster", name),
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
			if err := runEnableHA(cmd.Context(), flags); err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&flags.assumeYes, "yes", "y", false, "Assume yes to all prompts.")

	return cmd
}

func runEnableHA(ctx context.Context, flags EnableHACmdFlags) error {
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return fmt.Errorf("unable to get kube client: %w", err)
	}

	canEnableHA, reason, err := addons.CanEnableHA(ctx, kcli)
	if err != nil {
		return fmt.Errorf("unable to check if HA can be enabled: %w", err)
	}
	if !canEnableHA {
		logrus.Warnf("High availability cannot be enabled because %s", reason)
		return NewErrorNothingElseToAdd(fmt.Errorf("high availability cannot be enabled because %s", reason))
	}

	if !flags.assumeYes {
		// logrus.Info("High availability can be enabled once you have three or more controller nodes.")
		// logrus.Info("Enabling it will replicate data across the cluster to ensure resilience and fault tolerance.")
		// logrus.Info("After HA is enabled, you must maintain at least three controller nodes to keep it active.")
		// TODO: @ajp-io add in controller role name
		logrus.Info("You can enable high availability for clusters with three or more controller nodes.")
		logrus.Info("This will migrate data so that it is replicated across cluster nodes.")
		logrus.Info("When high availability is enabled, you must maintain at least three controller nodes.")
		logrus.Info("")

		shouldEnableHA := prompts.New().Confirm("Do you want to enable high availability?", true)
		if !shouldEnableHA {
			return nil
		}
		logrus.Info("")
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
