package cli

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/replicatedhq/embedded-cluster/api/console"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/configutils"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/preflights"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func InstallRunPreflightsCmd(ctx context.Context, name string) *cobra.Command {
	var consoleConfig console.Config
	var cliFlags installCmdFlags

	cmd := &cobra.Command{
		Use:    "run-preflights",
		Short:  "Run install host preflights",
		Hidden: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := preRunInstall(cmd, &consoleConfig, &cliFlags); err != nil {
				return err
			}

			return nil
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			runtimeconfig.Cleanup()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := runInstallRunPreflights(cmd.Context(), name, consoleConfig, cliFlags); err != nil {
				return err
			}

			return nil
		},
	}

	if err := addInstallConsoleConfigFlags(cmd, &consoleConfig); err != nil {
		panic(err)
	}
	if err := addInstallCmdFlags(cmd, &cliFlags); err != nil {
		panic(err)
	}

	return cmd
}

func runInstallRunPreflights(ctx context.Context, name string, inConsoleConfig console.Config, cliFlags installCmdFlags) error {
	listener, err := net.Listen("tcp", ":30080")
	if err != nil {
		return fmt.Errorf("unable to create listener: %w", err)
	}
	go runInstallAPI(ctx, listener)

	if err := waitForInstallAPI(ctx, listener.Addr().String()); err != nil {
		return fmt.Errorf("unable to wait for install API: %w", err)
	}

	consoleConfig, err := initializeConsoleAPIConfig(inConsoleConfig, listener.Addr().String())
	if err != nil {
		return fmt.Errorf("unable to initialize console API config: %w", err)
	}

	return doRunInstallRunPreflights(ctx, name, *consoleConfig, cliFlags)
}

func doRunInstallRunPreflights(ctx context.Context, name string, consoleConfig console.Config, cliFlags installCmdFlags) error {
	if err := runInstallVerifyAndPrompt(ctx, name, consoleConfig, cliFlags); err != nil {
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

	proxySpec, err := consoleConfig.GetProxySpec()
	if err != nil {
		return fmt.Errorf("unable to get proxy spec: %w", err)
	}

	logrus.Debugf("running install preflights")
	if err := runInstallPreflights(ctx, consoleConfig, cliFlags, proxySpec, nil); err != nil {
		if errors.Is(err, preflights.ErrPreflightsHaveFail) {
			return NewErrorNothingElseToAdd(err)
		}
		return fmt.Errorf("unable to run install preflights: %w", err)
	}

	logrus.Info("Host preflights completed successfully")

	return nil
}

func runInstallPreflights(ctx context.Context, consoleConfig console.Config, cliFlags installCmdFlags, proxySpec *ecv1beta1.ProxySpec, metricsReported preflights.MetricsReporter) error {
	replicatedAppURL := replicatedAppURL()
	proxyRegistryURL := proxyRegistryURL()

	nodeIP, err := netutils.FirstValidAddress(consoleConfig.NetworkInterface)
	if err != nil {
		return fmt.Errorf("unable to find first valid address: %w", err)
	}

	if err := preflights.PrepareAndRun(ctx, preflights.PrepareAndRunOptions{
		ReplicatedAppURL:     replicatedAppURL,
		ProxyRegistryURL:     proxyRegistryURL,
		Proxy:                proxySpec,
		PodCIDR:              consoleConfig.PodCIDR,
		ServiceCIDR:          consoleConfig.ServiceCIDR,
		GlobalCIDR:           consoleConfig.GlobalCIDR,
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
