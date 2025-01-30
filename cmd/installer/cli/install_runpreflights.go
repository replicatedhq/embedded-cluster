package cli

import (
	"context"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/pkg/configutils"
	"github.com/replicatedhq/embedded-cluster/pkg/preflights"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// ErrNothingElseToAdd is an error returned when there is nothing else to add to the
// screen. This is useful when we want to exit an error from a function here but
// don't want to print anything else (possibly because we have already printed the
// necessary data to the screen).
var ErrNothingElseToAdd = fmt.Errorf("")

// ErrPreflightsHaveFail is an error returned when we managed to execute the
// host preflights but they contain failures. We use this to differentiate the
// way we provide user feedback.
var ErrPreflightsHaveFail = fmt.Errorf("host preflight failures detected")

func InstallRunPreflightsCmd(ctx context.Context, name string) *cobra.Command {
	var flags Install2CmdFlags

	cmd := &cobra.Command{
		Use:   "run-preflights",
		Short: "Run install host preflights",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := preRunInstall2(cmd, &flags); err != nil {
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

func runInstallRunPreflights(ctx context.Context, name string, flags Install2CmdFlags) error {
	if err := runInstallVerifyAndPrompt(ctx, name, &flags); err != nil {
		return err
	}

	logrus.Debugf("materializing binaries")
	if err := materializeFiles(flags.airgapBundle); err != nil {
		return fmt.Errorf("unable to materialize files: %w", err)
	}

	if err := configutils.ConfigureSysctl(); err != nil {
		return fmt.Errorf("unable to configure sysctl: %w", err)
	}

	logrus.Debugf("running host preflights")
	if err := runInstallPreflights(ctx, flags); err != nil {
		return err
	}

	logrus.Info("Host preflights completed successfully")

	return nil
}

func runInstallPreflights(ctx context.Context, flags Install2CmdFlags) error {
	var replicatedAPIURL, proxyRegistryURL string
	if flags.license != nil {
		replicatedAPIURL = flags.license.Spec.Endpoint
		proxyRegistryURL = fmt.Sprintf("https://%s", runtimeconfig.ProxyRegistryAddress)
	}

	if err := preflights.PrepareAndRun(ctx, preflights.PrepareAndRunOptions{
		ReplicatedAPIURL:     replicatedAPIURL,
		ProxyRegistryURL:     proxyRegistryURL,
		Proxy:                flags.proxy,
		PodCIDR:              flags.cidrCfg.PodCIDR,
		ServiceCIDR:          flags.cidrCfg.ServiceCIDR,
		GlobalCIDR:           flags.cidrCfg.GlobalCIDR,
		PrivateCAs:           flags.privateCAs,
		IsAirgap:             flags.isAirgap,
		SkipHostPreflights:   flags.skipHostPreflights,
		IgnoreHostPreflights: flags.ignoreHostPreflights,
		AssumeYes:            flags.assumeYes,
	}); err != nil {
		if err == preflights.ErrPreflightsHaveFail {
			return ErrNothingElseToAdd
		}
		return err
	}

	return nil
}
