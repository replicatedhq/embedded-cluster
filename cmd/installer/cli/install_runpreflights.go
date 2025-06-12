package cli

import (
	"context"
	"errors"
	"fmt"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/hostutils"
	"github.com/replicatedhq/embedded-cluster/pkg-new/preflights"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/prompts"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// ErrPreflightsHaveFail is an error returned when we managed to execute the host preflights but
// they contain failures. We use this to differentiate the way we provide user feedback.
var ErrPreflightsHaveFail = metrics.NewErrorNoFail(fmt.Errorf("host preflight failures detected"))

func InstallRunPreflightsCmd(ctx context.Context, name string) *cobra.Command {
	var flags InstallCmdFlags
	rc := runtimeconfig.New(nil)

	cmd := &cobra.Command{
		Use:    "run-preflights",
		Short:  "Run install host preflights",
		Hidden: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := preRunInstall(cmd, &flags, rc); err != nil {
				return err
			}

			return nil
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			rc.Cleanup()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := runInstallRunPreflights(cmd.Context(), name, flags, rc); err != nil {
				return err
			}

			return nil
		},
	}

	if err := addInstallFlags(cmd, &flags, ecv1beta1.DefaultDataDir); err != nil {
		panic(err)
	}
	if err := addInstallAdminConsoleFlags(cmd, &flags); err != nil {
		panic(err)
	}

	return cmd
}

func runInstallRunPreflights(ctx context.Context, name string, flags InstallCmdFlags, rc runtimeconfig.RuntimeConfig) error {
	if err := verifyAndPrompt(ctx, name, flags, prompts.New()); err != nil {
		return err
	}

	logrus.Debugf("materializing binaries")
	if err := hostutils.MaterializeFiles(rc, flags.airgapBundle); err != nil {
		return fmt.Errorf("unable to materialize files: %w", err)
	}

	logrus.Debugf("configuring sysctl")
	if err := hostutils.ConfigureSysctl(); err != nil {
		logrus.Debugf("unable to configure sysctl: %v", err)
	}

	logrus.Debugf("configuring kernel modules")
	if err := hostutils.ConfigureKernelModules(); err != nil {
		logrus.Debugf("unable to configure kernel modules: %v", err)
	}

	logrus.Debugf("running install preflights")
	if err := runInstallPreflights(ctx, flags, rc, nil); err != nil {
		if errors.Is(err, preflights.ErrPreflightsHaveFail) {
			return NewErrorNothingElseToAdd(err)
		}
		return fmt.Errorf("unable to run install preflights: %w", err)
	}

	logrus.Info("Host preflights completed successfully")

	return nil
}

func runInstallPreflights(ctx context.Context, flags InstallCmdFlags, rc runtimeconfig.RuntimeConfig, metricsReporter metrics.ReporterInterface) error {
	replicatedAppURL := replicatedAppURL()
	proxyRegistryURL := proxyRegistryURL()

	nodeIP, err := netutils.FirstValidAddress(flags.networkInterface)
	if err != nil {
		return fmt.Errorf("unable to find first valid address: %w", err)
	}

	hpf, err := preflights.Prepare(ctx, preflights.PrepareOptions{
		HostPreflightSpec:       release.GetHostPreflights(),
		ReplicatedAppURL:        replicatedAppURL,
		ProxyRegistryURL:        proxyRegistryURL,
		AdminConsolePort:        rc.AdminConsolePort(),
		LocalArtifactMirrorPort: rc.LocalArtifactMirrorPort(),
		DataDir:                 rc.EmbeddedClusterHomeDirectory(),
		K0sDataDir:              rc.EmbeddedClusterK0sSubDir(),
		OpenEBSDataDir:          rc.EmbeddedClusterOpenEBSLocalSubDir(),
		Proxy:                   flags.proxy,
		PodCIDR:                 flags.cidrCfg.PodCIDR,
		ServiceCIDR:             flags.cidrCfg.ServiceCIDR,
		GlobalCIDR:              flags.cidrCfg.GlobalCIDR,
		NodeIP:                  nodeIP,
		IsAirgap:                flags.isAirgap,
	})
	if err != nil {
		return err
	}

	if err := runHostPreflights(ctx, hpf, flags.proxy, rc, flags.skipHostPreflights, flags.ignoreHostPreflights, flags.assumeYes, metricsReporter); err != nil {
		return err
	}

	return nil
}
