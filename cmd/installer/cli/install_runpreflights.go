package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/console"
	"github.com/replicatedhq/embedded-cluster/pkg/configutils"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/preflights"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func InstallRunPreflightsCmd(ctx context.Context, name string) *cobra.Command {
	var flags InstallCmdFlags

	cmd := &cobra.Command{
		Use:    "run-preflights",
		Short:  "Run install host preflights",
		Hidden: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := preRunInstall(cmd, &flags); err != nil {
				return err
			}

			return nil
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			runtimeconfig.Cleanup()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := runInstallRunPreflights(cmd.Context(), name, flags); err != nil {
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

func runInstallRunPreflights(ctx context.Context, name string, flags InstallCmdFlags) error {
	if err := runInstallVerifyAndPrompt(ctx, name, &flags); err != nil {
		return err
	}

	logrus.Debugf("materializing binaries")
	if err := materializeFiles(flags.airgapBundle); err != nil {
		return fmt.Errorf("unable to materialize files: %w", err)
	}

	logrus.Debugf("configuring sysctl")
	if err := configutils.ConfigureSysctl(); err != nil {
		logrus.Debugf("unable to configure sysctl: %v", err)
	}

	logrus.Debugf("configuring kernel modules")
	if err := configutils.ConfigureKernelModules(); err != nil {
		logrus.Debugf("unable to configure kernel modules: %v", err)
	}

	logrus.Debugf("running install preflights")
	if err := runInstallPreflights(ctx, flags, nil); err != nil {
		if errors.Is(err, preflights.ErrPreflightsHaveFail) {
			return NewErrorNothingElseToAdd(err)
		}
		return fmt.Errorf("unable to run install preflights: %w", err)
	}

	logrus.Info("Host preflights completed successfully")

	return nil
}

func runInstallPreflights(ctx context.Context, consoleConfig console.Config, cliFlags CLIFlags, metricsReported preflights.MetricsReporter) error {
	replicatedAppURL := replicatedAppURL()
	proxyRegistryURL := proxyRegistryURL()

	nodeIP, err := netutils.FirstValidAddress(consoleConfig.NetworkInterface)
	if err != nil {
		return fmt.Errorf("unable to find first valid address: %w", err)
	}

	if err := preflights.PrepareAndRun(ctx, preflights.PrepareAndRunOptions{
		ReplicatedAppURL:     replicatedAppURL,
		ProxyRegistryURL:     proxyRegistryURL,
		Proxy:                flags.proxy,
		PodCIDR:              flags.cidrCfg.PodCIDR,
		ServiceCIDR:          cliFlags.cidrCfg.ServiceCIDR,
		GlobalCIDR:           cliFlags.cidrCfg.GlobalCIDR,
		NodeIP:               nodeIP,
		PrivateCAs:           cliFlags.privateCAs,
		IsAirgap:             cliFlags.airgapBundle != "",
		SkipHostPreflights:   cliFlags.skipHostPreflights,
		IgnoreHostPreflights: cliFlags.ignoreHostPreflights,
		AssumeYes:            cliFlags.assumeYes,
		MetricsReporter:      metricsReported,
	}); err != nil {
		return err
	}

	return nil
}
