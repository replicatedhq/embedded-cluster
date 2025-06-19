package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/pkg-new/hostutils"
	"github.com/replicatedhq/embedded-cluster/pkg-new/preflights"
	"github.com/replicatedhq/embedded-cluster/pkg/airgap"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/prompts"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
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

			_ = rc.SetEnv()

			return nil
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			rc.Cleanup()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var airgapInfo *kotsv1beta1.Airgap
			if flags.airgapBundle != "" {
				var err error
				airgapInfo, err = airgap.AirgapInfoFromPath(flags.airgapBundle)
				if err != nil {
					return fmt.Errorf("failed to get airgap info: %w", err)
				}
			}

			if err := runInstallRunPreflights(cmd.Context(), name, flags, rc, airgapInfo); err != nil {
				return err
			}

			return nil
		},
	}

	if err := addInstallFlags(cmd, &flags); err != nil {
		panic(err)
	}
	if err := addInstallAdminConsoleFlags(cmd, &flags); err != nil {
		panic(err)
	}

	return cmd
}

func runInstallRunPreflights(ctx context.Context, name string, flags InstallCmdFlags, rc runtimeconfig.RuntimeConfig, airgapInfo *kotsv1beta1.Airgap) error {
	if err := verifyAndPrompt(ctx, name, flags, prompts.New(), airgapInfo); err != nil {
		return err
	}

	logrus.Debugf("configuring host")
	if err := hostutils.ConfigureHost(ctx, rc, hostutils.InitForInstallOptions{
		LicenseFile:  flags.licenseFile,
		AirgapBundle: flags.airgapBundle,
	}); err != nil {
		return fmt.Errorf("configure host: %w", err)
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

	nodeIP, err := netutils.FirstValidAddress(rc.NetworkInterface())
	if err != nil {
		return fmt.Errorf("unable to find first valid address: %w", err)
	}

	opts := preflights.PrepareOptions{
		HostPreflightSpec:       release.GetHostPreflights(),
		ReplicatedAppURL:        replicatedAppURL,
		ProxyRegistryURL:        proxyRegistryURL,
		AdminConsolePort:        rc.AdminConsolePort(),
		LocalArtifactMirrorPort: rc.LocalArtifactMirrorPort(),
		DataDir:                 rc.EmbeddedClusterHomeDirectory(),
		K0sDataDir:              rc.EmbeddedClusterK0sSubDir(),
		OpenEBSDataDir:          rc.EmbeddedClusterOpenEBSLocalSubDir(),
		Proxy:                   rc.ProxySpec(),
		PodCIDR:                 rc.PodCIDR(),
		ServiceCIDR:             rc.ServiceCIDR(),
		NodeIP:                  nodeIP,
		IsAirgap:                flags.isAirgap,
	}
	if globalCIDR := rc.GlobalCIDR(); globalCIDR != "" {
		opts.GlobalCIDR = &globalCIDR
	}

	hpf, err := preflights.Prepare(ctx, opts)
	if err != nil {
		return err
	}

	if err := runHostPreflights(ctx, hpf, rc, flags.skipHostPreflights, flags.ignoreHostPreflights, flags.assumeYes, metricsReporter); err != nil {
		return err
	}

	return nil
}
