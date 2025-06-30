package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/kinds/types/join"
	newconfig "github.com/replicatedhq/embedded-cluster/pkg-new/config"
	"github.com/replicatedhq/embedded-cluster/pkg-new/domains"
	"github.com/replicatedhq/embedded-cluster/pkg-new/hostutils"
	"github.com/replicatedhq/embedded-cluster/pkg-new/preflights"
	"github.com/replicatedhq/embedded-cluster/pkg/kotsadm"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func JoinRunPreflightsCmd(ctx context.Context, name string) *cobra.Command {
	var flags JoinCmdFlags
	rc := runtimeconfig.New(nil)

	cmd := &cobra.Command{
		Use:   "run-preflights",
		Short: fmt.Sprintf("Run join host preflights for %s", name),
		Args:  cobra.ExactArgs(2),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := preRunJoin(&flags); err != nil {
				return err
			}

			return nil
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			rc.Cleanup()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			logrus.Debugf("fetching join token remotely")
			jcmd, err := kotsadm.GetJoinToken(ctx, args[0], args[1])
			if err != nil {
				return fmt.Errorf("unable to get join token: %w", err)
			}
			if err := runJoinRunPreflights(cmd.Context(), name, flags, rc, jcmd, args[0]); err != nil {
				return err
			}

			return nil
		},
	}

	if err := addJoinFlags(cmd, &flags); err != nil {
		panic(err)
	}

	return cmd
}

func runJoinRunPreflights(ctx context.Context, name string, flags JoinCmdFlags, rc runtimeconfig.RuntimeConfig, jcmd *join.JoinCommandResponse, kotsAPIAddress string) error {
	if err := runJoinVerifyAndPrompt(name, flags, rc, jcmd); err != nil {
		return err
	}

	logrus.Debugf("materializing %s binaries", name)
	if err := materializeFilesForJoin(ctx, rc, jcmd, kotsAPIAddress); err != nil {
		return fmt.Errorf("failed to materialize files: %w", err)
	}

	logrus.Debugf("configuring sysctl")
	if err := hostutils.ConfigureSysctl(); err != nil {
		logrus.Debugf("unable to configure sysctl: %v", err)
	}

	logrus.Debugf("configuring kernel modules")
	if err := hostutils.ConfigureKernelModules(); err != nil {
		logrus.Debugf("unable to configure kernel modules: %v", err)
	}

	cidrCfg, err := getJoinCIDRConfig(rc)
	if err != nil {
		return fmt.Errorf("unable to get join CIDR config: %w", err)
	}

	logrus.Debugf("running join preflights")
	if err := runJoinPreflights(ctx, jcmd, flags, rc, cidrCfg, nil); err != nil {
		if errors.Is(err, preflights.ErrPreflightsHaveFail) {
			return NewErrorNothingElseToAdd(err)
		}
		return fmt.Errorf("unable to run join preflights: %w", err)
	}

	logrus.Info("Host preflights completed successfully")

	return nil
}

func runJoinPreflights(ctx context.Context, jcmd *join.JoinCommandResponse, flags JoinCmdFlags, rc runtimeconfig.RuntimeConfig, cidrCfg *newconfig.CIDRConfig, metricsReporter metrics.ReporterInterface) error {
	nodeIP, err := netutils.FirstValidAddress(flags.networkInterface)
	if err != nil {
		return fmt.Errorf("unable to find first valid address: %w", err)
	}

	domains := domains.GetDomains(jcmd.InstallationSpec.Config, release.GetChannelRelease())

	hpf, err := preflights.Prepare(ctx, preflights.PrepareOptions{
		HostPreflightSpec:       release.GetHostPreflights(),
		ReplicatedAppURL:        netutils.MaybeAddHTTPS(domains.ReplicatedAppDomain),
		ProxyRegistryURL:        netutils.MaybeAddHTTPS(domains.ProxyRegistryDomain),
		AdminConsolePort:        rc.AdminConsolePort(),
		LocalArtifactMirrorPort: rc.LocalArtifactMirrorPort(),
		DataDir:                 rc.EmbeddedClusterHomeDirectory(),
		K0sDataDir:              rc.EmbeddedClusterK0sSubDir(),
		OpenEBSDataDir:          rc.EmbeddedClusterOpenEBSLocalSubDir(),
		Proxy:                   rc.ProxySpec(),
		PodCIDR:                 cidrCfg.PodCIDR,
		ServiceCIDR:             cidrCfg.ServiceCIDR,
		NodeIP:                  nodeIP,
		IsAirgap:                jcmd.InstallationSpec.AirGap,
		TCPConnectionsRequired:  jcmd.TCPConnectionsRequired,
		IsJoin:                  true,
	})
	if err != nil {
		return err
	}

	if err := runHostPreflights(ctx, hpf, rc, flags.skipHostPreflights, flags.ignoreHostPreflights, flags.assumeYes, metricsReporter); err != nil {
		return err
	}

	return nil
}
