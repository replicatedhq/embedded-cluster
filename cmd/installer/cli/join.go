package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/AlecAivazis/survey/v2/terminal"
	k0sconfig "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/kinds/types/join"
	"github.com/replicatedhq/embedded-cluster/pkg/addons"
	"github.com/replicatedhq/embedded-cluster/pkg/airgap"
	"github.com/replicatedhq/embedded-cluster/pkg/config"
	"github.com/replicatedhq/embedded-cluster/pkg/configutils"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/k0s"
	"github.com/replicatedhq/embedded-cluster/pkg/kotsadm"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
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

type JoinCmdFlags struct {
	airgapBundle         string
	isAirgap             bool
	noHA                 bool
	networkInterface     string
	assumeYes            bool
	skipHostPreflights   bool
	ignoreHostPreflights bool
}

// JoinCmd returns a cobra command for joining a node to the cluster.
func JoinCmd(ctx context.Context, name string) *cobra.Command {
	var flags JoinCmdFlags

	ctx, cancel := context.WithCancel(ctx)

	cmd := &cobra.Command{
		Use:   "join <url> <token>",
		Short: fmt.Sprintf("Join %s", name),
		Args:  cobra.ExactArgs(2),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := preRunJoin(&flags); err != nil {
				return err
			}

			flags.isAirgap = flags.airgapBundle != ""

			return nil
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			runtimeconfig.Cleanup()
			cancel() // Cancel context when command completes
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			logrus.Debugf("fetching join token remotely")
			jcmd, err := kotsadm.GetJoinToken(ctx, args[0], args[1])
			if err != nil {
				return fmt.Errorf("unable to get join token: %w", err)
			}
			metricsReporter := NewJoinReporter(
				jcmd.InstallationSpec.MetricsBaseURL, jcmd.ClusterID, cmd.CalledAs(), flagsToStringSlice(cmd.Flags()),
			)
			metricsReporter.ReportJoinStarted(ctx)

			// Setup signal handler with the metrics reporter cleanup function
			signalHandler(ctx, cancel, func(ctx context.Context, sig os.Signal) {
				metricsReporter.ReportSignalAborted(ctx, sig)
			})

			if err := runJoin(cmd.Context(), name, flags, jcmd, metricsReporter); err != nil {
				// Check if this is an interrupt error from the terminal
				if errors.Is(err, terminal.InterruptErr) {
					metricsReporter.ReportSignalAborted(ctx, syscall.SIGINT)
				} else {
					metricsReporter.ReportJoinFailed(ctx, err)
				}
				return err
			}

			metricsReporter.ReportJoinSucceeded(ctx)
			return nil
		},
	}

	if err := addJoinFlags(cmd, &flags); err != nil {
		panic(err)
	}

	cmd.AddCommand(JoinRunPreflightsCmd(ctx, name))

	return cmd
}

func preRunJoin(flags *JoinCmdFlags) error {
	if os.Getuid() != 0 {
		return fmt.Errorf("join command must be run as root")
	}

	flags.isAirgap = flags.airgapBundle != ""

	// set the umask to 022 so that we can create files/directories with 755 permissions
	// this does not return an error - it returns the previous umask
	_ = syscall.Umask(0o022)

	return nil
}

func addJoinFlags(cmd *cobra.Command, flags *JoinCmdFlags) error {
	cmd.Flags().StringVar(&flags.airgapBundle, "airgap-bundle", "", "Path to the air gap bundle. If set, the installation will complete without internet access.")
	cmd.Flags().StringVar(&flags.networkInterface, "network-interface", "", "The network interface to use for the cluster")
	cmd.Flags().BoolVar(&flags.ignoreHostPreflights, "ignore-host-preflights", false, "Run host preflight checks, but prompt the user to continue if they fail instead of exiting.")
	cmd.Flags().BoolVar(&flags.noHA, "no-ha", false, "Do not prompt for or enable high availability.")

	cmd.Flags().BoolVar(&flags.skipHostPreflights, "skip-host-preflights", false, "Skip host preflight checks. This is not recommended and has been deprecated.")
	if err := cmd.Flags().MarkHidden("skip-host-preflights"); err != nil {
		return err
	}
	if err := cmd.Flags().MarkDeprecated("skip-host-preflights", "This flag is deprecated and will be removed in a future version. Use --ignore-host-preflights instead."); err != nil {
		return err
	}

	cmd.Flags().BoolVarP(&flags.assumeYes, "yes", "y", false, "Assume yes to all prompts.")
	cmd.Flags().SetNormalizeFunc(normalizeNoPromptToYes)

	return nil
}

func runJoin(ctx context.Context, name string, flags JoinCmdFlags, jcmd *join.JoinCommandResponse, metricsReporter preflights.MetricsReporter) error {
	// both controller and worker nodes will have 'worker' in the join command
	isWorker := !strings.Contains(jcmd.K0sJoinCommand, "controller")
	if !isWorker {
		logrus.Warnf("Do not join another node until this join is complete.")
	}

	if err := runJoinVerifyAndPrompt(name, flags, jcmd); err != nil {
		return err
	}

	logrus.Debugf("materializing %s binaries", name)
	if err := materializeFiles(flags.airgapBundle); err != nil {
		return err
	}

	logrus.Debugf("configuring sysctl")
	if err := configutils.ConfigureSysctl(); err != nil {
		logrus.Debugf("unable to configure sysctl: %v", err)
	}

	logrus.Debugf("configuring kernel modules")
	if err := configutils.ConfigureKernelModules(); err != nil {
		logrus.Debugf("unable to configure kernel modules: %v", err)
	}

	logrus.Debugf("configuring network manager")
	if err := configureNetworkManager(ctx); err != nil {
		return fmt.Errorf("unable to configure network manager: %w", err)
	}

	cidrCfg, err := getJoinCIDRConfig(jcmd)
	if err != nil {
		return fmt.Errorf("unable to get join CIDR config: %w", err)
	}

	logrus.Debugf("configuring firewalld")
	if err := configureFirewalld(ctx, cidrCfg.PodCIDR, cidrCfg.ServiceCIDR); err != nil {
		logrus.Debugf("unable to configure firewalld: %v", err)
	}

	logrus.Debugf("running join preflights")
	if err := runJoinPreflights(ctx, jcmd, flags, cidrCfg, metricsReporter); err != nil {
		if errors.Is(err, preflights.ErrPreflightsHaveFail) {
			return NewErrorNothingElseToAdd(err)
		}
		return fmt.Errorf("unable to run join preflights: %w", err)
	}

	logrus.Debugf("installing and joining cluster")
	if err := installAndJoinCluster(ctx, jcmd, name, flags, isWorker); err != nil {
		return err
	}

	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return fmt.Errorf("unable to get kube client: %w", err)
	}

	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("unable to get hostname: %w", err)
	}

	logrus.Debugf("waiting for node to join cluster")
	nodename := strings.ToLower(hostname)
	if err := waitForNodeToJoin(ctx, kcli, nodename, isWorker); err != nil {
		return fmt.Errorf("unable to wait for node: %w", err)
	}

	if isWorker {
		logrus.Debugf("worker node join finished")
		return nil
	}

	if err := maybeEnableHA(ctx, kcli, flags, cidrCfg.ServiceCIDR, jcmd); err != nil {
		return fmt.Errorf("unable to enable high availability: %w", err)
	}

	logrus.Debugf("controller node join finished")
	return nil
}

func runJoinVerifyAndPrompt(name string, flags JoinCmdFlags, jcmd *join.JoinCommandResponse) error {
	logrus.Debugf("checking if k0s is already installed")
	err := verifyNoInstallation(name, "join a node")
	if err != nil {
		return err
	}

	err = verifyChannelRelease("join", flags.isAirgap, flags.assumeYes)
	if err != nil {
		return err
	}

	if flags.isAirgap {
		logrus.Debugf("checking airgap bundle matches binary")
		if err := checkAirgapMatches(flags.airgapBundle); err != nil {
			return err // we want the user to see the error message without a prefix
		}
	}

	runtimeconfig.Set(jcmd.InstallationSpec.RuntimeConfig)
	isWorker := !strings.Contains(jcmd.K0sJoinCommand, "controller")
	if isWorker {
		os.Setenv("KUBECONFIG", runtimeconfig.PathToKubeletConfig())
	} else {
		os.Setenv("KUBECONFIG", runtimeconfig.PathToKubeConfig())
	}
	os.Setenv("TMPDIR", runtimeconfig.EmbeddedClusterTmpSubDir())

	if err := runtimeconfig.WriteToDisk(); err != nil {
		return fmt.Errorf("unable to write runtime config: %w", err)
	}

	if err := os.Chmod(runtimeconfig.EmbeddedClusterHomeDirectory(), 0755); err != nil {
		// don't fail as there are cases where we can't change the permissions (bind mounts, selinux, etc...),
		// and we handle and surface those errors to the user later (host preflights, checking exec errors, etc...)
		logrus.Debugf("unable to chmod embedded-cluster home dir: %s", err)
	}

	// if the application version is set, check to make sure that it matches the version we are running
	channelRelease := release.GetChannelRelease()
	if jcmd.AppVersionLabel != "" && channelRelease != nil {
		if jcmd.AppVersionLabel != channelRelease.VersionLabel {
			return fmt.Errorf("embedded cluster application version mismatch - this binary is compiled for app version %q, but the cluster is running version %q", channelRelease.VersionLabel, jcmd.AppVersionLabel)
		}
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
		return NewErrorNothingElseToAdd(errors.New("node ip address not included in no-proxy list"))
	}

	return nil
}

func getJoinCIDRConfig(jcmd *join.JoinCommandResponse) (*CIDRConfig, error) {
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

func installAndJoinCluster(ctx context.Context, jcmd *join.JoinCommandResponse, name string, flags JoinCmdFlags, isWorker bool) error {
	logrus.Debugf("saving token to disk")
	if err := saveTokenToDisk(jcmd.K0sToken); err != nil {
		return fmt.Errorf("unable to save token to disk: %w", err)
	}

	logrus.Debugf("installing %s binaries", name)
	if err := installK0sBinary(); err != nil {
		return fmt.Errorf("unable to install k0s binary: %w", err)
	}

	if jcmd.AirgapRegistryAddress != "" {
		if err := airgap.AddInsecureRegistry(jcmd.AirgapRegistryAddress); err != nil {
			return fmt.Errorf("unable to add insecure registry: %w", err)
		}
	}

	logrus.Debugf("creating systemd unit files")
	if err := createSystemdUnitFiles(ctx, isWorker, jcmd.InstallationSpec.Proxy); err != nil {
		return fmt.Errorf("unable to create systemd unit files: %w", err)
	}

	logrus.Debugf("overriding network configuration")
	if err := applyNetworkConfiguration(flags.networkInterface, jcmd); err != nil {
		return fmt.Errorf("unable to apply network configuration: %w", err)
	}

	logrus.Debugf("applying configuration overrides")
	if err := applyJoinConfigurationOverrides(jcmd); err != nil {
		return fmt.Errorf("unable to apply configuration overrides: %w", err)
	}

	profile, err := getFirstDefinedProfile()
	if err != nil {
		return fmt.Errorf("unable to get first defined profile: %w", err)
	}

	logrus.Debugf("joining node to cluster")
	if err := runK0sInstallCommand(flags.networkInterface, jcmd.K0sJoinCommand, profile); err != nil {
		return fmt.Errorf("unable to join node to cluster: %w", err)
	}

	if err := startAndWaitForK0s(ctx, name, jcmd); err != nil {
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

func applyNetworkConfiguration(networkInterface string, jcmd *join.JoinCommandResponse) error {
	if jcmd.InstallationSpec.Network != nil {
		domains := runtimeconfig.GetDomains(jcmd.InstallationSpec.Config)
		clusterSpec := config.RenderK0sConfig(domains.ProxyRegistryDomain)

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
func startAndWaitForK0s(ctx context.Context, name string, jcmd *join.JoinCommandResponse) error {
	loading := spinner.Start()
	defer loading.Close()
	loading.Infof("Installing %s node", name)
	logrus.Debugf("starting %s service", name)
	if _, err := helpers.RunCommand(runtimeconfig.K0sBinaryPath(), "start"); err != nil {
		return fmt.Errorf("unable to start service: %w", err)
	}

	loading.Infof("Waiting for %s node to be ready", name)
	logrus.Debugf("waiting for k0s to be ready")
	if err := waitForK0s(); err != nil {
		return fmt.Errorf("unable to wait for node: %w", err)
	}

	loading.Infof("Node installation finished!")
	return nil
}

// applyJoinConfigurationOverrides applies both config overrides received from the kots api.
// Applies first the EmbeddedOverrides and then the EndUserOverrides.
func applyJoinConfigurationOverrides(jcmd *join.JoinCommandResponse) error {
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

func getFirstDefinedProfile() (string, error) {
	k0scfg, err := os.Open(runtimeconfig.PathToK0sConfig())
	if err != nil {
		return "", fmt.Errorf("unable to open k0s config: %w", err)
	}
	defer k0scfg.Close()
	cfg, err := k0sconfig.ConfigFromReader(k0scfg)
	if err != nil {
		return "", fmt.Errorf("unable to parse k0s config: %w", err)
	}
	if len(cfg.Spec.WorkerProfiles) > 0 {
		return cfg.Spec.WorkerProfiles[0].Name, nil
	}
	return "", nil
}

// runK0sInstallCommand runs the k0s install command as provided by the kots
func runK0sInstallCommand(networkInterface string, fullcmd string, profile string) error {
	args := strings.Split(fullcmd, " ")
	args = append(args, "--token-file", "/etc/k0s/join-token")

	nodeIP, err := netutils.FirstValidAddress(networkInterface)
	if err != nil {
		return fmt.Errorf("unable to find first valid address: %w", err)
	}

	if profile != "" {
		args = append(args, "--profile", profile)
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

func waitForNodeToJoin(ctx context.Context, kcli client.Client, hostname string, isWorker bool) error {
	loading := spinner.Start()
	defer loading.Close()
	loading.Infof("Waiting for node to join the cluster")
	if err := kubeutils.WaitForNode(ctx, kcli, hostname, isWorker); err != nil {
		return fmt.Errorf("unable to wait for node: %w", err)
	}
	loading.Infof("Node has joined the cluster!")
	return nil
}

func maybeEnableHA(ctx context.Context, kcli client.Client, flags JoinCmdFlags, serviceCIDR string, jcmd *join.JoinCommandResponse) error {
	if flags.noHA {
		logrus.Debug("--no-ha flag provided, skipping high availability")
		return nil
	}

	canEnableHA, _, err := addons.CanEnableHA(ctx, kcli)
	if err != nil {
		return fmt.Errorf("unable to check if HA can be enabled: %w", err)
	}
	if !canEnableHA {
		return nil
	}
	if !flags.assumeYes {
		if config.HasCustomRoles() {
			controllerRoleName := config.GetControllerRoleName()
			logrus.Info("")
			logrus.Infof("You can enable high availability for clusters with three or more %s nodes.", controllerRoleName)
			logrus.Infof("Data will be migrated so it is replicated across cluster nodes.")
			logrus.Infof("When high availability is enabled, you must maintain at least three %s nodes.", controllerRoleName)
			logrus.Info("")
		} else {
			logrus.Info("")
			logrus.Info("You can enable high availability for clusters with three or more nodes.")
			logrus.Info("Data will be migrated so it is replicated across cluster nodes.")
			logrus.Info("When high availability is enabled, you must maintain at least three nodes.")
			logrus.Info("")
		}
		shouldEnableHA, err := prompts.New().Confirm("Do you want to enable high availability?", false)
		if err != nil {
			return fmt.Errorf("failed to get confirmation: %w", err)
		}
		if !shouldEnableHA {
			return nil
		}
		logrus.Info("")
	}

	kclient, err := kubeutils.GetClientset()
	if err != nil {
		return fmt.Errorf("unable to create kubernetes client: %w", err)
	}

	airgapChartsPath := ""
	if flags.isAirgap {
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

	loading := spinner.Start()
	defer loading.Close()

	return addons.EnableHA(
		ctx,
		kcli,
		kclient,
		hcli,
		serviceCIDR,
		jcmd.InstallationSpec,
		loading,
	)
}
