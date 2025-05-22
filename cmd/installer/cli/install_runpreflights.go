package cli

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/replicatedhq/embedded-cluster/api"
	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/configutils"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/preflights"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func InstallRunPreflightsCmd(ctx context.Context, name string) *cobra.Command {
	var installConfig apitypes.InstallationConfig
	var cliFlags installCmdFlags

	cmd := &cobra.Command{
		Use:    "run-preflights",
		Short:  "Run install host preflights",
		Hidden: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := preRunInstall(cmd, &installConfig, &cliFlags); err != nil {
				return err
			}

			return nil
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			runtimeconfig.Cleanup()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := runInstallRunPreflights(cmd.Context(), name, installConfig, cliFlags); err != nil {
				return err
			}

			return nil
		},
	}

	if err := addInstallConfigFlags(cmd, &installConfig); err != nil {
		panic(err)
	}
	if err := addInstallCmdFlags(cmd, &cliFlags); err != nil {
		panic(err)
	}

	return cmd
}

func runInstallRunPreflights(ctx context.Context, name string, inInstallConfig apitypes.InstallationConfig, cliFlags installCmdFlags) error {
	logger, err := api.NewLogger()
	if err != nil {
		logrus.Warnf("Unable to setup API logging: %v", err)
	}

	listener, err := net.Listen("tcp", ":30080")
	if err != nil {
		return fmt.Errorf("unable to create listener: %w", err)
	}

	apiCtx, apiCancel := context.WithCancel(ctx)
	defer apiCancel()
	go runInstallAPI(apiCtx, listener, logger)

	if err := waitForInstallAPI(ctx, listener.Addr().String()); err != nil {
		return fmt.Errorf("unable to wait for install API: %w", err)
	}

	installConfig, err := initializeInstallAPIConfig(inInstallConfig, listener.Addr().String())
	if err != nil {
		return fmt.Errorf("unable to initialize install API config: %w", err)
	}

	return doRunInstallRunPreflights(ctx, name, *installConfig, cliFlags)
}

func doRunInstallRunPreflights(ctx context.Context, name string, installConfig apitypes.InstallationConfig, cliFlags installCmdFlags) error {
	if err := runInstallVerifyAndPrompt(ctx, name, installConfig, cliFlags); err != nil {
		return err
	}

	logrus.Debugf("materializing binaries")
	if err := materializeFiles(cliFlags.airgapBundle); err != nil {
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
	if err := runInstallPreflights(ctx, installConfig, cliFlags, nil); err != nil {
		if errors.Is(err, preflights.ErrPreflightsHaveFail) {
			return NewErrorNothingElseToAdd(err)
		}
		return fmt.Errorf("unable to run install preflights: %w", err)
	}

	logrus.Info("Host preflights completed successfully")

	return nil
}

func runInstallPreflights(ctx context.Context, installConfig apitypes.InstallationConfig, cliFlags installCmdFlags, metricsReported preflights.MetricsReporter) error {
	replicatedAppURL := replicatedAppURL()
	proxyRegistryURL := proxyRegistryURL()

	nodeIP, err := netutils.FirstValidAddress(installConfig.NetworkInterface)
	if err != nil {
		return fmt.Errorf("unable to find first valid address: %w", err)
	}

	if err := preflights.PrepareAndRun(ctx, preflights.PrepareAndRunOptions{
		ReplicatedAppURL:     replicatedAppURL,
		ProxyRegistryURL:     proxyRegistryURL,
		Proxy:                runtimeconfig.ProxySpec(),
		PodCIDR:              installConfig.PodCIDR,
		ServiceCIDR:          installConfig.ServiceCIDR,
		GlobalCIDR:           installConfig.GlobalCIDR,
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
