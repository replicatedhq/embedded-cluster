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
	"github.com/replicatedhq/embedded-cluster/cmd/installer/goods"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/kinds/types/join"
	newconfig "github.com/replicatedhq/embedded-cluster/pkg-new/config"
	"github.com/replicatedhq/embedded-cluster/pkg-new/domains"
	"github.com/replicatedhq/embedded-cluster/pkg-new/hostutils"
	"github.com/replicatedhq/embedded-cluster/pkg-new/k0s"
	"github.com/replicatedhq/embedded-cluster/pkg-new/preflights"
	"github.com/replicatedhq/embedded-cluster/pkg/addons"
	"github.com/replicatedhq/embedded-cluster/pkg/airgap"
	"github.com/replicatedhq/embedded-cluster/pkg/config"
	"github.com/replicatedhq/embedded-cluster/pkg/dryrun"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/kotsadm"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/prompts"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"github.com/replicatedhq/embedded-cluster/pkg/support"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/metadata"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8syaml "sigs.k8s.io/yaml"
)

type JoinCmdFlags struct {
	noHA                 bool
	networkInterface     string
	assumeYes            bool
	skipHostPreflights   bool
	ignoreHostPreflights bool
}

// JoinCmd returns a cobra command for joining a node to the cluster.
func JoinCmd(ctx context.Context, appSlug, appTitle string) *cobra.Command {
	var flags JoinCmdFlags

	ctx, cancel := context.WithCancel(ctx)
	rc := runtimeconfig.New(nil)

	cmd := &cobra.Command{
		Use:   "join <url> <token>",
		Short: fmt.Sprintf("Join a node to the %s cluster", appTitle),
		Args:  cobra.ExactArgs(2),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := preRunJoin(&flags); err != nil {
				return err
			}

			return nil
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			rc.Cleanup()
			cancel() // Cancel context when command completes
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			logrus.Debugf("fetching join token remotely")
			jcmd, err := kotsadm.GetJoinToken(ctx, args[0], args[1])
			if err != nil {
				return fmt.Errorf("unable to get join token: %w", err)
			}
			joinReporter := newJoinReporter(
				jcmd.InstallationSpec.MetricsBaseURL, jcmd.ClusterID, cmd.CalledAs(), flagsToStringSlice(cmd.Flags()),
			)
			joinReporter.ReportJoinStarted(ctx)

			// Setup signal handler with the metrics reporter cleanup function
			signalHandler(ctx, cancel, func(ctx context.Context, sig os.Signal) {
				joinReporter.ReportSignalAborted(ctx, sig)
			})

			if err := runJoin(cmd.Context(), appSlug, flags, rc, jcmd, args[0], joinReporter); err != nil {
				// Check if this is an interrupt error from the terminal
				if errors.Is(err, terminal.InterruptErr) {
					joinReporter.ReportSignalAborted(ctx, syscall.SIGINT)
				} else {
					joinReporter.ReportJoinFailed(ctx, err)
				}
				return err
			}

			joinReporter.ReportJoinSucceeded(ctx)
			return nil
		},
	}

	if err := addJoinFlags(cmd, &flags); err != nil {
		panic(err)
	}

	cmd.AddCommand(JoinRunPreflightsCmd(ctx, appSlug, appTitle))
	cmd.AddCommand(JoinPrintCommandCmd(ctx, appTitle))

	return cmd
}

func preRunJoin(flags *JoinCmdFlags) error {
	// Skip root check if dryrun mode is enabled
	if !dryrun.Enabled() && os.Getuid() != 0 {
		return fmt.Errorf("join command must be run as root")
	}

	// if a network interface flag was not provided, attempt to discover it
	if flags.networkInterface == "" {
		autoInterface, err := newconfig.DetermineBestNetworkInterface()
		if err == nil {
			flags.networkInterface = autoInterface
		}
	}

	return nil
}

func addJoinFlags(cmd *cobra.Command, flags *JoinCmdFlags) error {
	cmd.Flags().String("airgap-bundle", "", "Path to the air gap bundle. If set, the installation will complete without internet access.")
	if err := cmd.Flags().MarkDeprecated("airgap-bundle", "This flag is deprecated (ignored) and will be removed in a future version. The cluster will automatically determine if it's in airgap mode and fetch the necessary artifacts from other nodes."); err != nil {
		return err
	}

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

func runJoin(ctx context.Context, appSlug string, flags JoinCmdFlags, rc runtimeconfig.RuntimeConfig, jcmd *join.JoinCommandResponse, kotsAPIAddress string, joinReporter *JoinReporter) error {
	// both controller and worker nodes will have 'worker' in the join command
	isWorker := !strings.Contains(jcmd.K0sJoinCommand, "controller")
	if !isWorker {
		logrus.Warn("\nDo not join another node until this node has joined successfully.")
	}

	if err := runJoinVerifyAndPrompt(appSlug, flags, rc, jcmd); err != nil {
		return err
	}

	cidrCfg, err := initializeJoin(ctx, appSlug, rc, jcmd, kotsAPIAddress)
	if err != nil {
		return fmt.Errorf("unable to initialize join: %w", err)
	}

	logrus.Debugf("running join preflights")
	if err := runJoinPreflights(ctx, jcmd, flags, rc, cidrCfg, joinReporter.reporter); err != nil {
		if errors.Is(err, preflights.ErrPreflightsHaveFail) {
			return NewErrorNothingElseToAdd(err)
		}
		return fmt.Errorf("unable to run join preflights: %w", err)
	}

	logrus.Debugf("installing and joining cluster")
	loading := spinner.Start()
	loading.Infof("Installing node")
	if err := installAndJoinCluster(ctx, rc, jcmd, appSlug, flags, isWorker); err != nil {
		loading.ErrorClosef("Failed to install node")
		return err
	}

	kcli, err := kubeutils.KubeClient()
	if err != nil {
		loading.ErrorClosef("Failed to install node")
		return fmt.Errorf("unable to get kube client: %w", err)
	}

	mcli, err := kubeutils.MetadataClient()
	if err != nil {
		loading.ErrorClosef("Failed to install node")
		return fmt.Errorf("unable to get metadata client: %w", err)
	}

	hostname, err := os.Hostname()
	if err != nil {
		loading.ErrorClosef("Failed to install node")
		return fmt.Errorf("unable to get hostname: %w", err)
	}

	logrus.Debugf("waiting for node to join cluster")
	loading.Infof("Waiting for node")
	nodename := strings.ToLower(hostname)
	if err := waitForNodeToJoin(ctx, kcli, nodename, isWorker); err != nil {
		loading.ErrorClosef("Node failed to become ready")
		return fmt.Errorf("unable to wait for node: %w", err)
	}

	loading.Closef("Node is ready")
	logrus.Infof("\nNode joined the cluster successfully.\n")
	if isWorker {
		logrus.Debugf("worker node join finished")
		return nil
	}

	if err := maybeEnableHA(ctx, kcli, mcli, flags, rc, jcmd); err != nil {
		return fmt.Errorf("unable to enable high availability: %w", err)
	}

	logrus.Debugf("controller node join finished")
	return nil
}

func runJoinVerifyAndPrompt(appSlug string, flags JoinCmdFlags, rc runtimeconfig.RuntimeConfig, jcmd *join.JoinCommandResponse) error {
	logrus.Debugf("checking if k0s is already installed")
	err := verifyNoInstallation(appSlug, "join a node")
	if err != nil {
		return err
	}

	rc.Set(jcmd.InstallationSpec.RuntimeConfig)
	isWorker := !strings.Contains(jcmd.K0sJoinCommand, "controller")
	if isWorker {
		os.Setenv("KUBECONFIG", rc.PathToKubeletConfig())
	} else {
		os.Setenv("KUBECONFIG", rc.PathToKubeConfig())
	}
	os.Setenv("TMPDIR", rc.EmbeddedClusterTmpSubDir())

	if err := rc.WriteToDisk(); err != nil {
		return fmt.Errorf("unable to write runtime config: %w", err)
	}

	if err := os.Chmod(rc.EmbeddedClusterHomeDirectory(), 0755); err != nil {
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

	if proxySpec := rc.ProxySpec(); proxySpec != nil {
		newconfig.SetProxyEnv(proxySpec)

		proxyOK, localIP, err := newconfig.CheckProxyConfigForLocalIP(proxySpec, flags.networkInterface, nil)
		if err != nil {
			return fmt.Errorf("failed to check proxy config for local IP: %w", err)
		}

		if !proxyOK {
			logrus.Errorf("\nThis node's IP address %s is not included in the no-proxy list (%s).", localIP, proxySpec.NoProxy)
			logrus.Infof(`The no-proxy list cannot easily be modified after installing.`)
			logrus.Infof(`Recreate the first node and pass all node IP addresses to --no-proxy.`)
			return NewErrorNothingElseToAdd(errors.New("node ip address not included in no-proxy list"))
		}
	}

	return nil
}

func initializeJoin(ctx context.Context, appSlug string, rc runtimeconfig.RuntimeConfig, jcmd *join.JoinCommandResponse, kotsAPIAddress string) (cidrCfg *newconfig.CIDRConfig, err error) {
	logrus.Info("")
	spinner := spinner.Start()
	spinner.Infof("Initializing")
	defer func() {
		if err != nil {
			spinner.ErrorClosef("Initialization failed")
		} else {
			spinner.Closef("Initialization complete")
		}
	}()

	// set the umask to 022 so that we can create files/directories with 755 permissions
	// this does not return an error - it returns the previous umask
	_ = syscall.Umask(0o022)

	if err := os.Chmod(rc.EmbeddedClusterHomeDirectory(), 0755); err != nil {
		// don't fail as there are cases where we can't change the permissions (bind mounts, selinux, etc...),
		// and we handle and surface those errors to the user later (host preflights, checking exec errors, etc...)
		logrus.Debugf("unable to chmod embedded-cluster home dir: %s", err)
	}

	logrus.Debugf("materializing %s binaries", appSlug)
	if err := materializeFilesForJoin(ctx, rc, jcmd, kotsAPIAddress); err != nil {
		return nil, fmt.Errorf("failed to materialize files: %w", err)
	}

	logrus.Debugf("configuring sysctl")
	if err := hostutils.ConfigureSysctl(); err != nil {
		logrus.Debugf("unable to configure sysctl: %v", err)
	}

	logrus.Debugf("configuring kernel modules")
	if err := hostutils.ConfigureKernelModules(); err != nil {
		logrus.Debugf("unable to configure kernel modules: %v", err)
	}

	logrus.Debugf("configuring network manager")
	if err := hostutils.ConfigureNetworkManager(ctx, rc); err != nil {
		return nil, fmt.Errorf("unable to configure network manager: %w", err)
	}

	cidrCfg, err = getJoinCIDRConfig(rc)
	if err != nil {
		return nil, fmt.Errorf("unable to get join CIDR config: %w", err)
	}

	logrus.Debugf("configuring firewalld")
	if err := hostutils.ConfigureFirewalld(ctx, cidrCfg.PodCIDR, cidrCfg.ServiceCIDR); err != nil {
		logrus.Debugf("unable to configure firewalld: %v", err)
	}

	return cidrCfg, nil
}

func materializeFilesForJoin(ctx context.Context, rc runtimeconfig.RuntimeConfig, jcmd *join.JoinCommandResponse, kotsAPIAddress string) error {
	materializer := goods.NewMaterializer(rc)
	if err := materializer.Materialize(); err != nil {
		return fmt.Errorf("materialize binaries: %w", err)
	}

	if err := support.MaterializeSupportBundleSpec(rc, jcmd.InstallationSpec.AirGap); err != nil {
		return fmt.Errorf("materialize support bundle spec: %w", err)
	}

	if jcmd.InstallationSpec.AirGap {
		if err := airgap.FetchAndWriteArtifacts(ctx, kotsAPIAddress, rc); err != nil {
			return fmt.Errorf("failed to fetch artifacts: %w", err)
		}
	}

	return nil
}

func getJoinCIDRConfig(rc runtimeconfig.RuntimeConfig) (*newconfig.CIDRConfig, error) {
	globalCIDR := ecv1beta1.DefaultNetworkCIDR
	if rc.GlobalCIDR() != "" {
		globalCIDR = rc.GlobalCIDR()
	}

	podCIDR, serviceCIDR, err := netutils.SplitNetworkCIDR(globalCIDR)
	if err != nil {
		return nil, fmt.Errorf("unable to split global network CIDR: %w", err)
	}

	if rc.PodCIDR() != "" {
		podCIDR = rc.PodCIDR()
	}
	if rc.ServiceCIDR() != "" {
		serviceCIDR = rc.ServiceCIDR()
	}

	return &newconfig.CIDRConfig{
		PodCIDR:     podCIDR,
		ServiceCIDR: serviceCIDR,
	}, nil
}

func installAndJoinCluster(ctx context.Context, rc runtimeconfig.RuntimeConfig, jcmd *join.JoinCommandResponse, appSlug string, flags JoinCmdFlags, isWorker bool) error {
	logrus.Debugf("saving token to disk")
	if err := saveTokenToDisk(jcmd.K0sToken); err != nil {
		return fmt.Errorf("unable to save token to disk: %w", err)
	}

	logrus.Debugf("installing %s binaries", appSlug)
	if err := installK0sBinary(rc); err != nil {
		return fmt.Errorf("unable to install k0s binary: %w", err)
	}

	if jcmd.AirgapRegistryAddress != "" {
		if err := hostutils.AddInsecureRegistry(jcmd.AirgapRegistryAddress); err != nil {
			return fmt.Errorf("unable to add insecure registry: %w", err)
		}
	}

	logrus.Debugf("creating systemd unit files")
	if err := hostutils.CreateSystemdUnitFiles(ctx, logrus.StandardLogger(), rc, isWorker); err != nil {
		return fmt.Errorf("unable to create systemd unit files: %w", err)
	}

	logrus.Debugf("overriding network configuration")
	if err := applyNetworkConfiguration(rc, jcmd); err != nil {
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
	if err := runK0sInstallCommand(rc, flags.networkInterface, jcmd.K0sJoinCommand, profile); err != nil {
		return fmt.Errorf("unable to join node to cluster: %w", err)
	}

	if err := startAndWaitForK0s(appSlug); err != nil {
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
func installK0sBinary(rc runtimeconfig.RuntimeConfig) error {
	ourbin := rc.PathToEmbeddedClusterBinary("k0s")
	hstbin := runtimeconfig.K0sBinaryPath
	if err := helpers.MoveFile(ourbin, hstbin); err != nil {
		return fmt.Errorf("unable to move k0s binary: %w", err)
	}
	return nil
}

func applyNetworkConfiguration(rc runtimeconfig.RuntimeConfig, jcmd *join.JoinCommandResponse) error {
	domains := domains.GetDomains(jcmd.InstallationSpec.Config, release.GetChannelRelease())
	clusterSpec := config.RenderK0sConfig(domains.ProxyRegistryDomain)

	address, err := netutils.FirstValidAddress(rc.NetworkInterface())
	if err != nil {
		return fmt.Errorf("unable to find first valid address: %w", err)
	}

	cidrCfg, err := getJoinCIDRConfig(rc)
	if err != nil {
		return fmt.Errorf("unable to get join CIDR config: %w", err)
	}

	clusterSpec.Spec.API.Address = address
	clusterSpec.Spec.Storage.Etcd.PeerAddress = address
	// NOTE: we should be copying everything from the in cluster config spec and overriding
	// the node specific config from clusterSpec.GetClusterWideConfig()
	clusterSpec.Spec.Network.PodCIDR = cidrCfg.PodCIDR
	clusterSpec.Spec.Network.ServiceCIDR = cidrCfg.ServiceCIDR

	if rc.NodePortRange() != "" {
		if clusterSpec.Spec.API.ExtraArgs == nil {
			clusterSpec.Spec.API.ExtraArgs = map[string]string{}
		}
		clusterSpec.Spec.API.ExtraArgs["service-node-port-range"] = rc.NodePortRange()
	}

	clusterSpecYaml, err := k8syaml.Marshal(clusterSpec)
	if err != nil {
		return fmt.Errorf("unable to marshal cluster spec: %w", err)
	}

	err = os.WriteFile(runtimeconfig.K0sConfigPath, clusterSpecYaml, 0644)
	if err != nil {
		return fmt.Errorf("unable to write cluster spec to /etc/k0s/k0s.yaml: %w", err)
	}

	return nil
}

// startAndWaitForK0s starts the k0s service and waits for the node to be ready.
func startAndWaitForK0s(appSlug string) error {
	logrus.Debugf("starting %s service", appSlug)
	if _, err := helpers.RunCommand(runtimeconfig.K0sBinaryPath, "start"); err != nil {
		return fmt.Errorf("unable to start service: %w", err)
	}

	logrus.Debugf("waiting for k0s to be ready")
	if err := k0s.WaitForK0s(); err != nil {
		return fmt.Errorf("unable to wait for node: %w", err)
	}

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
			runtimeconfig.K0sConfigPath, string(data),
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
		runtimeconfig.K0sConfigPath, string(data),
	); err != nil {
		return fmt.Errorf("unable to patch config with embedded data: %w", err)
	}
	return nil
}

func getFirstDefinedProfile() (string, error) {
	k0scfg, err := os.Open(runtimeconfig.K0sConfigPath)
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
func runK0sInstallCommand(rc runtimeconfig.RuntimeConfig, networkInterface string, fullcmd string, profile string) error {
	args := strings.Split(fullcmd, " ")
	args = append(args, "--token-file", "/etc/k0s/join-token")

	nodeIP, err := netutils.FirstValidAddress(networkInterface)
	if err != nil {
		return fmt.Errorf("unable to find first valid address: %w", err)
	}

	if profile != "" {
		args = append(args, "--profile", profile)
	}

	args = append(args, config.AdditionalInstallFlags(rc, nodeIP)...)

	if strings.Contains(fullcmd, "controller") {
		args = append(args, config.AdditionalInstallFlagsController()...)
	}

	if _, err := helpers.RunCommand(args[0], args[1:]...); err != nil {
		return err
	}
	return nil
}

func waitForNodeToJoin(ctx context.Context, kcli client.Client, hostname string, isWorker bool) error {
	if err := kubeutils.WaitForNode(ctx, kcli, hostname, isWorker); err != nil {
		return fmt.Errorf("unable to wait for node: %w", err)
	}
	return nil
}

func maybeEnableHA(ctx context.Context, kcli client.Client, mcli metadata.Interface, flags JoinCmdFlags, rc runtimeconfig.RuntimeConfig, jcmd *join.JoinCommandResponse) error {
	if flags.noHA {
		logrus.Debug("--no-ha flag provided, skipping high availability")
		return nil
	}

	kclient, err := kubeutils.GetClientset()
	if err != nil {
		return fmt.Errorf("unable to create kubernetes client: %w", err)
	}

	airgapChartsPath := ""
	if jcmd.InstallationSpec.AirGap {
		airgapChartsPath = rc.EmbeddedClusterChartsSubDir()
	}
	hcli, err := helm.NewClient(helm.HelmOptions{
		KubeConfig: rc.PathToKubeConfig(),
		K0sVersion: versions.K0sVersion,
		AirgapPath: airgapChartsPath,
	})
	if err != nil {
		return fmt.Errorf("unable to create helm client: %w", err)
	}
	defer hcli.Close()

	addOns := addons.New(
		addons.WithLogFunc(logrus.Debugf),
		addons.WithKubernetesClient(kcli),
		addons.WithKubernetesClientSet(kclient),
		addons.WithMetadataClient(mcli),
		addons.WithHelmClient(hcli),
		addons.WithDomains(getDomains()),
	)

	canEnableHA, _, err := addOns.CanEnableHA(ctx)
	if err != nil {
		return fmt.Errorf("unable to check if HA can be enabled: %w", err)
	}
	if !canEnableHA {
		return nil
	}

	if config.HasCustomRoles() {
		controllerRoleName := config.GetControllerRoleName()
		logrus.Infof("High availability can be enabled once you have three or more %s nodes.", controllerRoleName)
		logrus.Info("Enabling it will replicate data across cluster nodes.")
		logrus.Infof("After HA is enabled, you must maintain at least three %s nodes.\n", controllerRoleName)
	} else {
		logrus.Info("High availability can be enabled once you have three or more nodes.")
		logrus.Info("Enabling it will replicate data across cluster nodes.")
		logrus.Info("After HA is enabled, you must maintain at least three nodes.\n")
	}

	if !flags.assumeYes {
		shouldEnableHA, err := prompts.New().Confirm("Do you want to enable high availability?", true)
		if err != nil {
			return fmt.Errorf("failed to get confirmation: %w", err)
		}
		if !shouldEnableHA {
			return nil
		}
		logrus.Info("")
	}

	loading := spinner.Start()
	defer loading.Close()

	opts := addons.EnableHAOptions{
		AdminConsolePort:   rc.AdminConsolePort(),
		IsAirgap:           jcmd.InstallationSpec.AirGap,
		IsMultiNodeEnabled: jcmd.InstallationSpec.LicenseInfo != nil && jcmd.InstallationSpec.LicenseInfo.IsMultiNodeEnabled,
		EmbeddedConfigSpec: jcmd.InstallationSpec.Config,
		EndUserConfigSpec:  nil, // TODO: add support for end user config spec
		ProxySpec:          rc.ProxySpec(),
		HostCABundlePath:   rc.HostCABundlePath(),
		DataDir:            rc.EmbeddedClusterHomeDirectory(),
		K0sDataDir:         rc.EmbeddedClusterK0sSubDir(),
		SeaweedFSDataDir:   rc.EmbeddedClusterSeaweedFSSubDir(),
		ServiceCIDR:        rc.ServiceCIDR(),
	}

	return addOns.EnableHA(ctx, opts, loading)
}
