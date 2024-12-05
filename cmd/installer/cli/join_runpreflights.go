package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/configutils"
	"github.com/replicatedhq/embedded-cluster/pkg/kotsadm"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func JoinRunPreflightsCmd(ctx context.Context, name string) *cobra.Command {
	var (
		airgapBundle     string
		networkInterface string
		assumeYes        bool
	)
	cmd := &cobra.Command{
		Use:   "run-preflights",
		Short: fmt.Sprintf("Run join host preflights for %s", name),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if os.Getuid() != 0 {
				return fmt.Errorf("run-preflights command must be run as root")
			}

			return nil
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			runtimeconfig.Cleanup()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return fmt.Errorf("usage: %s join preflights <url> <token>", name)
			}

			logrus.Debugf("fetching join token remotely")
			jcmd, err := kotsadm.GetJoinToken(cmd.Context(), args[0], args[1])
			if err != nil {
				return fmt.Errorf("unable to get join token: %w", err)
			}

			runtimeconfig.Set(jcmd.InstallationSpec.RuntimeConfig)
			os.Setenv("TMPDIR", runtimeconfig.EmbeddedClusterTmpSubDir())

			// check to make sure the version returned by the join token is the same as the one we are running
			if strings.TrimPrefix(jcmd.EmbeddedClusterVersion, "v") != strings.TrimPrefix(versions.Version, "v") {
				return fmt.Errorf("embedded cluster version mismatch - this binary is version %q, but the cluster is running version %q", versions.Version, jcmd.EmbeddedClusterVersion)
			}

			setProxyEnv(jcmd.InstallationSpec.Proxy)

			networkInterfaceFlag, err := cmd.Flags().GetString("network-interface")
			if err != nil {
				return fmt.Errorf("unable to get network-interface flag: %w", err)
			}
			proxyOK, localIP, err := checkProxyConfigForLocalIP(jcmd.InstallationSpec.Proxy, networkInterfaceFlag)
			if err != nil {
				return fmt.Errorf("failed to check proxy config for local IP: %w", err)
			}
			if !proxyOK {
				return fmt.Errorf("no-proxy config %q does not allow access to local IP %q", jcmd.InstallationSpec.Proxy.NoProxy, localIP)
			}

			isAirgap := false
			if airgapBundle != "" {
				isAirgap = true
			}
			logrus.Debugf("materializing binaries")
			if err := materializeFiles(airgapBundle); err != nil {
				return err
			}

			if err := configutils.ConfigureSysctl(); err != nil {
				return err
			}

			opts := addonsApplierOpts{
				assumeYes:    assumeYes,
				license:      "",
				airgapBundle: airgapBundle,
				overrides:    "",
				privateCAs:   nil,
				configValues: "",
			}
			applier, err := getAddonsApplier(cmd, opts, "", jcmd.InstallationSpec.Proxy)
			if err != nil {
				return err
			}

			podCIDR, serviceCIDR, err := netutils.SplitNetworkCIDR(ecv1beta1.DefaultNetworkCIDR)
			if err != nil {
				return fmt.Errorf("unable to split default network CIDR: %w", err)
			}

			if jcmd.InstallationSpec.Network != nil {
				if jcmd.InstallationSpec.Network.PodCIDR != "" {
					podCIDR = jcmd.InstallationSpec.Network.PodCIDR
				}
				if jcmd.InstallationSpec.Network.ServiceCIDR != "" {
					serviceCIDR = jcmd.InstallationSpec.Network.ServiceCIDR
				}
			}

			cidrCfg := &CIDRConfig{
				PodCIDR:     podCIDR,
				ServiceCIDR: serviceCIDR,
			}

			logrus.Debugf("running host preflights")
			replicatedAPIURL := jcmd.InstallationSpec.MetricsBaseURL
			proxyRegistryURL := fmt.Sprintf("https://%s", runtimeconfig.ProxyRegistryAddress)
			if err := RunHostPreflights(cmd, applier, replicatedAPIURL, proxyRegistryURL, isAirgap, jcmd.InstallationSpec.Proxy, cidrCfg, jcmd.TCPConnectionsRequired, assumeYes); err != nil {
				if err == ErrPreflightsHaveFail {
					return ErrNothingElseToAdd
				}
				return err
			}

			logrus.Info("Host preflights completed successfully")

			return nil
		},
	}

	cmd.Flags().StringVar(&airgapBundle, "airgap-bundle", "", "Path to the air gap bundle. If set, the installation will complete without internet access.")
	cmd.Flags().MarkHidden("airgap-bundle")
	cmd.Flags().StringVar(&networkInterface, "network-interface", "", "The network interface to use for the cluster")

	cmd.Flags().Bool("skip-host-preflights", false, "Skip host preflight checks. This is not recommended and has been deprecated.")
	cmd.Flags().MarkHidden("skip-host-preflights")
	cmd.Flags().Bool("ignore-host-preflights", false, "Run host preflight checks, but prompt the user to continue if they fail instead of exiting.")
	cmd.Flags().MarkHidden("ignore-host-preflights")

	cmd.Flags().BoolVar(&assumeYes, "yes", false, "Assume yes to all prompts.")
	cmd.Flags().SetNormalizeFunc(normalizeNoPromptToYes)

	return cmd
}
