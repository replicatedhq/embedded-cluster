package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2"
	"github.com/replicatedhq/embedded-cluster/pkg/airgap"
	"github.com/replicatedhq/embedded-cluster/pkg/config"
	"github.com/replicatedhq/embedded-cluster/pkg/configutils"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/k0s"
	"github.com/replicatedhq/embedded-cluster/pkg/kotsadm"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/preflights"
	"github.com/replicatedhq/embedded-cluster/pkg/prompts"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8syaml "sigs.k8s.io/yaml"
)

type Join2CmdFlags struct {
	airgapBundle           string
	enableHighAvailability bool
	networkInterface       string
	assumeYes              bool
	skipHostPreflights     bool
	ignoreHostPreflights   bool
}

// This is the upcoming version of join without the operator and where
// join does all of the work. This is a hidden command until it's tested
// and ready.
func Join2Cmd(ctx context.Context, name string) *cobra.Command {
	var flags Join2CmdFlags

	cmd := &cobra.Command{
		Use:           "join2 <url> <token>",
		Short:         fmt.Sprintf("Join %s", name),
		Args:          cobra.ExactArgs(2),
		SilenceErrors: true,
		SilenceUsage:  true,
		Hidden:        true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if os.Getuid() != 0 {
				return fmt.Errorf("join command must be run as root")
			}
			if flags.skipHostPreflights {
				logrus.Warnf("Warning: --skip-host-preflights is deprecated and will be removed in a later version. Use --ignore-host-preflights instead.")
			}

			return nil
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			runtimeconfig.Cleanup()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := runJoin2(cmd, args, name, flags); err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&flags.airgapBundle, "airgap-bundle", "", "Path to the air gap bundle. If set, the installation will complete without internet access.")
	cmd.Flags().StringVar(&flags.networkInterface, "network-interface", "", "The network interface to use for the cluster")
	cmd.Flags().BoolVar(&flags.ignoreHostPreflights, "ignore-host-preflights", false, "Run host preflight checks, but prompt the user to continue if they fail instead of exiting.")

	cmd.Flags().BoolVar(&flags.enableHighAvailability, "enable-ha", false, "Enable high availability.")
	cmd.Flags().MarkHidden("enable-ha")

	cmd.Flags().BoolVar(&flags.skipHostPreflights, "skip-host-preflights", false, "Skip host preflight checks. This is not recommended and has been deprecated.")
	cmd.Flags().MarkHidden("skip-host-preflights")
	cmd.Flags().MarkDeprecated("skip-host-preflights", "This flag is deprecated and will be removed in a future version. Use --ignore-host-preflights instead.")

	cmd.Flags().BoolVar(&flags.assumeYes, "yes", false, "Assume yes to all prompts.")
	cmd.Flags().SetNormalizeFunc(normalizeNoPromptToYes)

	// TODO (@salah): add join preflights subcommand

	return cmd
}

func runJoin2(cmd *cobra.Command, args []string, name string, flags Join2CmdFlags) error {
	logrus.Debugf("fetching join token remotely")
	jcmd, err := kotsadm.GetJoinToken(cmd.Context(), args[0], args[1])
	if err != nil {
		return fmt.Errorf("unable to get join token: %w", err)
	}

	if err := runJoinVerifyAndPrompt(name, flags, jcmd); err != nil {
		return err
	}

	metrics.ReportJoinStarted(cmd.Context(), jcmd.InstallationSpec.MetricsBaseURL, jcmd.ClusterID)
	logrus.Debugf("materializing %s binaries", name)
	if err := materializeFiles(flags.airgapBundle); err != nil {
		metrics.ReportJoinFailed(cmd.Context(), jcmd.InstallationSpec.MetricsBaseURL, jcmd.ClusterID, err)
		return err
	}

	logrus.Debugf("configuring sysctl")
	if err := configutils.ConfigureSysctl(); err != nil {
		return fmt.Errorf("unable to configure sysctl: %w", err)
	}

	logrus.Debugf("configuring network manager")
	if err := configureNetworkManager(cmd.Context()); err != nil {
		return fmt.Errorf("unable to configure network manager: %w", err)
	}

	cidrCfg, err := getJoinCIDRConfig(jcmd)
	if err != nil {
		return fmt.Errorf("unable to get join CIDR config: %w", err)
	}

	logrus.Debugf("running join preflights")
	if err := runJoinPreflights(cmd.Context(), jcmd, flags, cidrCfg); err != nil {
		return fmt.Errorf("unable to run join preflights: %w", err)
	}

	logrus.Debugf("installing and joining cluster")
	if err := installAndJoinCluster(cmd, jcmd, name, flags); err != nil {
		return err
	}

	if !strings.Contains(jcmd.K0sJoinCommand, "controller") {
		metrics.ReportJoinSucceeded(cmd.Context(), jcmd.InstallationSpec.MetricsBaseURL, jcmd.ClusterID)
		logrus.Debugf("worker node join finished")
		return nil
	}

	kcli, err := kubeutils.KubeClient()
	if err != nil {
		err := fmt.Errorf("unable to get kube client: %w", err)
		metrics.ReportJoinFailed(cmd.Context(), jcmd.InstallationSpec.MetricsBaseURL, jcmd.ClusterID, err)
		return err
	}

	hostname, err := os.Hostname()
	if err != nil {
		err := fmt.Errorf("unable to get hostname: %w", err)
		metrics.ReportJoinFailed(cmd.Context(), jcmd.InstallationSpec.MetricsBaseURL, jcmd.ClusterID, err)
		return err
	}

	if err := waitForNode(cmd.Context(), kcli, hostname); err != nil {
		err := fmt.Errorf("unable to wait for node: %w", err)
		metrics.ReportJoinFailed(cmd.Context(), jcmd.InstallationSpec.MetricsBaseURL, jcmd.ClusterID, err)
		return err
	}

	logrus.Debugf("installing manager")
	if err := installAndEnableManager(cmd.Context()); err != nil {
		err := fmt.Errorf("unable to install and enable manager: %w", err)
		metrics.ReportJoinFailed(cmd.Context(), jcmd.InstallationSpec.MetricsBaseURL, jcmd.ClusterID, err)
		return err
	}

	if flags.enableHighAvailability {
		if err := maybeEnableHA(cmd.Context(), kcli, flags.airgapBundle != "", cidrCfg.ServiceCIDR, jcmd.InstallationSpec.Proxy); err != nil {
			err := fmt.Errorf("unable to enable high availability: %w", err)
			metrics.ReportJoinFailed(cmd.Context(), jcmd.InstallationSpec.MetricsBaseURL, jcmd.ClusterID, err)
			return err
		}
	}

	metrics.ReportJoinSucceeded(cmd.Context(), jcmd.InstallationSpec.MetricsBaseURL, jcmd.ClusterID)
	logrus.Debugf("controller node join finished")
	return nil
}

func runJoinVerifyAndPrompt(name string, flags Join2CmdFlags, jcmd *kotsadm.JoinCommandResponse) error {
	logrus.Debugf("checking if %s is already installed", name)
	installed, err := k0s.IsInstalled()
	if err != nil {
		return err
	}
	if installed {
		logrus.Errorf("An installation has been detected on this machine.")
		logrus.Infof("If you want to join a node to an existing installation, you need to remove the existing installation first.")
		logrus.Infof("You can do this by running the following command:")
		logrus.Infof("\n  sudo ./%s reset\n", name)
		os.Exit(1)
	}

	channelRelease, err := release.GetChannelRelease()
	if err != nil {
		return fmt.Errorf("unable to read channel release data: %w", err)
	}

	if channelRelease != nil && channelRelease.Airgap && flags.airgapBundle == "" && !flags.assumeYes {
		logrus.Infof("You downloaded an air gap bundle but are performing an online join.")
		logrus.Infof("To do an air gap join, pass the air gap bundle with --airgap-bundle.")
		if !prompts.New().Confirm("Do you want to proceed with an online join?", false) {
			return ErrNothingElseToAdd
		}
	}

	isAirgap := false
	if flags.airgapBundle != "" {
		isAirgap = true
	}
	if isAirgap {
		logrus.Debugf("checking airgap bundle matches binary")
		if err := checkAirgapMatches(flags.airgapBundle); err != nil {
			return err // we want the user to see the error message without a prefix
		}
	}

	runtimeconfig.Set(jcmd.InstallationSpec.RuntimeConfig)
	os.Setenv("KUBECONFIG", runtimeconfig.PathToKubeConfig())
	os.Setenv("TMPDIR", runtimeconfig.EmbeddedClusterTmpSubDir())

	if err := runtimeconfig.WriteToDisk(); err != nil {
		return fmt.Errorf("unable to write runtime config: %w", err)
	}

	// check to make sure the version returned by the join token is the same as the one we are running
	if strings.TrimPrefix(jcmd.EmbeddedClusterVersion, "v") != strings.TrimPrefix(versions.Version, "v") {
		return fmt.Errorf("embedded cluster version mismatch - this binary is version %q, but the cluster is running version %q", versions.Version, jcmd.EmbeddedClusterVersion)
	}

	setProxyEnv(jcmd.InstallationSpec.Proxy)

	proxyOK, localIP, err := checkProxyConfigForLocalIP(jcmd.InstallationSpec.Proxy, flags.networkInterface)
	if err != nil {
		return fmt.Errorf("failed to check proxy config for local IP: %w", err)
	}

	if !proxyOK {
		logrus.Errorf("This node's IP address %s is not included in the no-proxy list (%s).", localIP, jcmd.InstallationSpec.Proxy.NoProxy)
		logrus.Infof(`The no-proxy list cannot easily be modified after initial installation.`)
		logrus.Infof(`Recreate the first node and pass all node IP addresses to --no-proxy.`)
		return ErrNothingElseToAdd
	}

	return nil
}

func getJoinCIDRConfig(jcmd *kotsadm.JoinCommandResponse) (*CIDRConfig, error) {
	podCIDR, serviceCIDR, err := netutils.SplitNetworkCIDR(ecv1beta1.DefaultNetworkCIDR)
	if err != nil {
		return nil, fmt.Errorf("unable to split default network CIDR: %w", err)
	}

	if jcmd.InstallationSpec.Network != nil {
		if jcmd.InstallationSpec.Network.PodCIDR != "" {
			podCIDR = jcmd.InstallationSpec.Network.PodCIDR
		}
		if jcmd.InstallationSpec.Network.ServiceCIDR != "" {
			serviceCIDR = jcmd.InstallationSpec.Network.ServiceCIDR
		}
	}

	return &CIDRConfig{
		PodCIDR:     podCIDR,
		ServiceCIDR: serviceCIDR,
	}, nil
}

func runJoinPreflights(ctx context.Context, jcmd *kotsadm.JoinCommandResponse, flags Join2CmdFlags, cidrCfg *CIDRConfig) error {
	logrus.Debugf("running host preflights")
	if err := preflights.PrepareAndRun(ctx, preflights.PrepareAndRunOptions{
		ReplicatedAPIURL:       jcmd.InstallationSpec.MetricsBaseURL, // MetricsBaseURL is the replicated.app endpoint url
		ProxyRegistryURL:       fmt.Sprintf("https://%s", runtimeconfig.ProxyRegistryAddress),
		Proxy:                  jcmd.InstallationSpec.Proxy,
		PodCIDR:                cidrCfg.PodCIDR,
		ServiceCIDR:            cidrCfg.ServiceCIDR,
		IsAirgap:               flags.airgapBundle != "",
		SkipHostPreflights:     flags.skipHostPreflights,
		IgnoreHostPreflights:   flags.ignoreHostPreflights,
		AssumeYes:              flags.assumeYes,
		TCPConnectionsRequired: jcmd.TCPConnectionsRequired,
	}); err != nil {
		if err == preflights.ErrPreflightsHaveFail {
			// we exit and not return an error to prevent the error from being printed to stderr
			// we already handled the output
			os.Exit(1)
			return nil
		}
		return fmt.Errorf("unable to prepare and run preflights: %w", err)
	}

	return nil
}

func installAndJoinCluster(cmd *cobra.Command, jcmd *kotsadm.JoinCommandResponse, name string, flags Join2CmdFlags) error {
	logrus.Debugf("saving token to disk")
	if err := saveTokenToDisk(jcmd.K0sToken); err != nil {
		err := fmt.Errorf("unable to save token to disk: %w", err)
		metrics.ReportJoinFailed(cmd.Context(), jcmd.InstallationSpec.MetricsBaseURL, jcmd.ClusterID, err)
		return err
	}

	logrus.Debugf("installing %s binaries", name)
	if err := installK0sBinary(); err != nil {
		err := fmt.Errorf("unable to install k0s binary: %w", err)
		metrics.ReportJoinFailed(cmd.Context(), jcmd.InstallationSpec.MetricsBaseURL, jcmd.ClusterID, err)
		return err
	}

	if jcmd.AirgapRegistryAddress != "" {
		if err := airgap.AddInsecureRegistry(jcmd.AirgapRegistryAddress); err != nil {
			err := fmt.Errorf("unable to add insecure registry: %w", err)
			metrics.ReportJoinFailed(cmd.Context(), jcmd.InstallationSpec.MetricsBaseURL, jcmd.ClusterID, err)
			return err
		}
	}

	logrus.Debugf("creating systemd unit files")
	// both controller and worker nodes will have 'worker' in the join command
	if err := createSystemdUnitFiles(!strings.Contains(jcmd.K0sJoinCommand, "controller"), jcmd.InstallationSpec.Proxy); err != nil {
		err := fmt.Errorf("unable to create systemd unit files: %w", err)
		metrics.ReportJoinFailed(cmd.Context(), jcmd.InstallationSpec.MetricsBaseURL, jcmd.ClusterID, err)
		return err
	}

	logrus.Debugf("overriding network configuration")
	if err := applyNetworkConfiguration(flags.networkInterface, jcmd); err != nil {
		err := fmt.Errorf("unable to apply network configuration: %w", err)
		metrics.ReportJoinFailed(cmd.Context(), jcmd.InstallationSpec.MetricsBaseURL, jcmd.ClusterID, err)
	}

	logrus.Debugf("applying configuration overrides")
	if err := applyJoinConfigurationOverrides(jcmd); err != nil {
		err := fmt.Errorf("unable to apply configuration overrides: %w", err)
		metrics.ReportJoinFailed(cmd.Context(), jcmd.InstallationSpec.MetricsBaseURL, jcmd.ClusterID, err)
		return err
	}

	logrus.Debugf("joining node to cluster")
	if err := runK0sInstallCommand(flags.networkInterface, jcmd.K0sJoinCommand); err != nil {
		err := fmt.Errorf("unable to join node to cluster: %w", err)
		metrics.ReportJoinFailed(cmd.Context(), jcmd.InstallationSpec.MetricsBaseURL, jcmd.ClusterID, err)
		return err
	}

	if err := startAndWaitForK0s(cmd.Context(), name, jcmd); err != nil {
		return err
	}

	return nil
}

// saveTokenToDisk saves the provided token in "/etc/k0s/join-token".
func saveTokenToDisk(token string) error {
	if err := os.MkdirAll("/etc/k0s", 0755); err != nil {
		return err
	}
	data := []byte(token)
	if err := os.WriteFile("/etc/k0s/join-token", data, 0644); err != nil {
		return err
	}
	return nil
}

// installK0sBinary moves the embedded k0s binary to its destination.
func installK0sBinary() error {
	ourbin := runtimeconfig.PathToEmbeddedClusterBinary("k0s")
	hstbin := runtimeconfig.K0sBinaryPath()
	if err := helpers.MoveFile(ourbin, hstbin); err != nil {
		return fmt.Errorf("unable to move k0s binary: %w", err)
	}
	return nil
}

func applyNetworkConfiguration(networkInterface string, jcmd *kotsadm.JoinCommandResponse) error {
	if jcmd.InstallationSpec.Network != nil {
		clusterSpec := config.RenderK0sConfig()

		address, err := netutils.FirstValidAddress(networkInterface)
		if err != nil {
			return fmt.Errorf("unable to find first valid address: %w", err)
		}
		clusterSpec.Spec.API.Address = address
		clusterSpec.Spec.Storage.Etcd.PeerAddress = address
		// NOTE: we should be copying everything from the in cluster config spec and overriding
		// the node specific config from clusterSpec.GetClusterWideConfig()
		clusterSpec.Spec.Network.PodCIDR = jcmd.InstallationSpec.Network.PodCIDR
		clusterSpec.Spec.Network.ServiceCIDR = jcmd.InstallationSpec.Network.ServiceCIDR
		if jcmd.InstallationSpec.Network.NodePortRange != "" {
			if clusterSpec.Spec.API.ExtraArgs == nil {
				clusterSpec.Spec.API.ExtraArgs = map[string]string{}
			}
			clusterSpec.Spec.API.ExtraArgs["service-node-port-range"] = jcmd.InstallationSpec.Network.NodePortRange
		}
		clusterSpecYaml, err := k8syaml.Marshal(clusterSpec)

		if err != nil {
			return fmt.Errorf("unable to marshal cluster spec: %w", err)
		}
		err = os.WriteFile(runtimeconfig.PathToK0sConfig(), clusterSpecYaml, 0644)
		if err != nil {
			return fmt.Errorf("unable to write cluster spec to /etc/k0s/k0s.yaml: %w", err)
		}
	}
	return nil
}

// startAndWaitForK0s starts the k0s service and waits for the node to be ready.
func startAndWaitForK0s(ctx context.Context, name string, jcmd *kotsadm.JoinCommandResponse) error {
	loading := spinner.Start()
	defer loading.Close()
	loading.Infof("Installing %s node", name)
	logrus.Debugf("starting %s service", name)
	if _, err := helpers.RunCommand(runtimeconfig.K0sBinaryPath(), "start"); err != nil {
		err := fmt.Errorf("unable to start service: %w", err)
		metrics.ReportJoinFailed(ctx, jcmd.InstallationSpec.MetricsBaseURL, jcmd.ClusterID, err)
		return err
	}

	loading.Infof("Waiting for %s node to be ready", name)
	logrus.Debugf("waiting for k0s to be ready")
	if err := waitForK0s(); err != nil {
		err := fmt.Errorf("unable to wait for node: %w", err)
		metrics.ReportJoinFailed(ctx, jcmd.InstallationSpec.MetricsBaseURL, jcmd.ClusterID, err)
		return err
	}

	loading.Infof("Node installation finished!")
	return nil
}

// applyJoinConfigurationOverrides applies both config overrides received from the kots api.
// Applies first the EmbeddedOverrides and then the EndUserOverrides.
func applyJoinConfigurationOverrides(jcmd *kotsadm.JoinCommandResponse) error {
	patch, err := jcmd.EmbeddedOverrides()
	if err != nil {
		return fmt.Errorf("unable to get embedded overrides: %w", err)
	}
	if len(patch) > 0 {
		if data, err := yaml.Marshal(patch); err != nil {
			return fmt.Errorf("unable to marshal embedded overrides: %w", err)
		} else if err := k0s.PatchK0sConfig(
			runtimeconfig.PathToK0sConfig(), string(data),
		); err != nil {
			return fmt.Errorf("unable to patch config with embedded data: %w", err)
		}
	}
	if patch, err = jcmd.EndUserOverrides(); err != nil {
		return fmt.Errorf("unable to get embedded overrides: %w", err)
	} else if len(patch) == 0 {
		return nil
	}
	if data, err := yaml.Marshal(patch); err != nil {
		return fmt.Errorf("unable to marshal embedded overrides: %w", err)
	} else if err := k0s.PatchK0sConfig(
		runtimeconfig.PathToK0sConfig(), string(data),
	); err != nil {
		return fmt.Errorf("unable to patch config with embedded data: %w", err)
	}
	return nil
}

// runK0sInstallCommand runs the k0s install command as provided by the kots
// adm api.
func runK0sInstallCommand(networkInterface string, fullcmd string) error {
	args := strings.Split(fullcmd, " ")
	args = append(args, "--token-file", "/etc/k0s/join-token")

	nodeIP, err := netutils.FirstValidAddress(networkInterface)
	if err != nil {
		return fmt.Errorf("unable to find first valid address: %w", err)
	}

	args = append(args, config.AdditionalInstallFlags(nodeIP)...)

	if strings.Contains(fullcmd, "controller") {
		args = append(args, config.AdditionalInstallFlagsController()...)
	}

	if _, err := helpers.RunCommand(args[0], args[1:]...); err != nil {
		return err
	}
	return nil
}

func waitForNode(ctx context.Context, kcli client.Client, hostname string) error {
	loading := spinner.Start()
	defer loading.Close()
	loading.Infof("Waiting for node to join the cluster")
	if err := kubeutils.WaitForControllerNode(ctx, kcli, hostname); err != nil {
		return fmt.Errorf("unable to wait for node: %w", err)
	}
	loading.Infof("Node has joined the cluster!")
	return nil
}

func maybeEnableHA(ctx context.Context, kcli client.Client, isAirgap bool, serviceCIDR string, proxy *ecv1beta1.ProxySpec) error {
	canEnableHA, err := addons2.CanEnableHA(ctx, kcli)
	if err != nil {
		return fmt.Errorf("unable to check if HA can be enabled: %w", err)
	}
	if !canEnableHA {
		return nil
	}
	logrus.Info("")
	logrus.Info("You can enable high availability when adding a third controller node or more. This will migrate data so that it is replicated across cluster nodes. Once enabled, you must maintain at least three controller nodes.")
	logrus.Info("")
	shouldEnableHA := prompts.New().Confirm("Do you want to enable high availability?", false)
	if !shouldEnableHA {
		return nil
	}
	logrus.Info("")
	return addons2.EnableHA(ctx, kcli, isAirgap, serviceCIDR, proxy)
}
