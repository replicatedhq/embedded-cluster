package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	k0sconfig "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/airgap"
	"github.com/replicatedhq/embedded-cluster/pkg/config"
	"github.com/replicatedhq/embedded-cluster/pkg/configutils"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/highavailability"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/prompts"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8syaml "sigs.k8s.io/yaml"
)

func JoinCmd(ctx context.Context, name string) *cobra.Command {
	var (
		airgapBundle            string
		enabledHighAvailability bool
		networkInterface        string
		noPrompt                bool
		skipHostPreflights      bool
	)

	cmd := &cobra.Command{
		Use:   "join <url> <token>",
		Short: fmt.Sprintf("Join %s", name),
		Args:  cobra.ExactArgs(2),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if os.Getuid() != 0 {
				return fmt.Errorf("join command must be run as root")
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			logrus.Debugf("checking if %s is already installed", name)
			installed, err := isAlreadyInstalled()

			if err != nil {
				return err
			}

			if installed {
				logrus.Errorf("An installation has been detected on this machine.")
				logrus.Infof("If you want to reinstall you need to remove the existing installation")
				logrus.Infof("first. You can do this by running the following command:")
				logrus.Infof("\n  sudo ./%s reset\n", name)
				return ErrNothingElseToAdd
			}

			channelRelease, err := release.GetChannelRelease()
			if err != nil {
				return fmt.Errorf("unable to read channel release data: %w", err)
			}

			if channelRelease != nil && channelRelease.Airgap && airgapBundle == "" && !noPrompt {
				logrus.Infof("You downloaded an air gap bundle but are performing an online join.")
				logrus.Infof("To do an air gap join, pass the air gap bundle with --airgap-bundle.")
				if !prompts.New().Confirm("Do you want to proceed with an online join?", false) {
					return ErrNothingElseToAdd
				}
			}

			logrus.Debugf("fetching join token remotely")
			jcmd, err := getJoinToken(cmd.Context(), args[0], args[1])
			if err != nil {
				return fmt.Errorf("unable to get join token: %w", err)
			}

			err = configutils.WriteRuntimeConfig(jcmd.InstallationSpec.RuntimeConfig)
			if err != nil {
				return fmt.Errorf("unable to write runtime config: %w", err)
			}

			provider := defaults.NewProviderFromRuntimeConfig(jcmd.InstallationSpec.RuntimeConfig)
			os.Setenv("KUBECONFIG", provider.PathToKubeConfig())
			os.Setenv("TMPDIR", provider.EmbeddedClusterTmpSubDir())

			defer tryRemoveTmpDirContents(provider)

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
				logrus.Errorf("This node's IP address %s is not included in the no-proxy list (%s).", localIP, jcmd.InstallationSpec.Proxy.NoProxy)
				logrus.Infof(`The no-proxy list cannot easily be modified after initial installation.`)
				logrus.Infof(`Recreate the first node and pass all node IP addresses to --no-proxy.`)
				return ErrNothingElseToAdd
			}

			isAirgap := false
			if airgapBundle != "" {
				isAirgap = true
			}

			if isAirgap {
				logrus.Debugf("checking airgap bundle matches binary")
				if err := checkAirgapMatches(airgapBundle); err != nil {
					return err // we want the user to see the error message without a prefix
				}
			}

			metrics.ReportJoinStarted(cmd.Context(), jcmd.InstallationSpec.MetricsBaseURL, jcmd.ClusterID)
			logrus.Debugf("materializing %s binaries", name)
			if err := materializeFiles(cmd, provider); err != nil {
				metrics.ReportJoinFailed(cmd.Context(), jcmd.InstallationSpec.MetricsBaseURL, jcmd.ClusterID, err)
				return err
			}

			opts := addonsApplierOpts{
				noPrompt:     noPrompt,
				license:      "",
				airgapBundle: airgapBundle,
				overrides:    "",
				privateCAs:   nil,
				configValues: "",
			}
			applier, err := getAddonsApplier(cmd, opts, jcmd.InstallationSpec.RuntimeConfig, "", jcmd.InstallationSpec.Proxy)
			if err != nil {
				metrics.ReportJoinFailed(cmd.Context(), jcmd.InstallationSpec.MetricsBaseURL, jcmd.ClusterID, err)
				return err
			}

			logrus.Debugf("configuring sysctl")
			if err := configutils.ConfigureSysctl(provider); err != nil {
				return fmt.Errorf("unable to configure sysctl: %w", err)
			}

			fromCIDR, toCIDR, err := netutils.SplitNetworkCIDR(ecv1beta1.DefaultNetworkCIDR)
			if err != nil {
				return fmt.Errorf("unable to split default network CIDR: %w", err)
			}

			if jcmd.InstallationSpec.Network != nil {
				if jcmd.InstallationSpec.Network.PodCIDR != "" {
					fromCIDR = jcmd.InstallationSpec.Network.PodCIDR
				}
				if jcmd.InstallationSpec.Network.ServiceCIDR != "" {
					toCIDR = jcmd.InstallationSpec.Network.ServiceCIDR
				}
			}

			// jcmd.InstallationSpec.MetricsBaseURL is the replicated.app endpoint url
			replicatedAPIURL := jcmd.InstallationSpec.MetricsBaseURL
			proxyRegistryURL := fmt.Sprintf("https://%s", defaults.ProxyRegistryAddress)
			if err := RunHostPreflights(cmd, provider, applier, replicatedAPIURL, proxyRegistryURL, isAirgap, jcmd.InstallationSpec.Proxy, fromCIDR, toCIDR); err != nil {
				metrics.ReportJoinFailed(cmd.Context(), jcmd.InstallationSpec.MetricsBaseURL, jcmd.ClusterID, err)
				if err == ErrPreflightsHaveFail {
					return ErrNothingElseToAdd
				}
				return err
			}

			logrus.Debugf("configuring network manager")
			if err := configureNetworkManager(cmd, provider); err != nil {
				return fmt.Errorf("unable to configure network manager: %w", err)
			}

			logrus.Debugf("saving token to disk")
			if err := saveTokenToDisk(jcmd.K0sToken); err != nil {
				err := fmt.Errorf("unable to save token to disk: %w", err)
				metrics.ReportJoinFailed(cmd.Context(), jcmd.InstallationSpec.MetricsBaseURL, jcmd.ClusterID, err)
				return err
			}

			logrus.Debugf("installing %s binaries", name)
			if err := installK0sBinary(provider); err != nil {
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
			if err := createSystemdUnitFiles(provider, !strings.Contains(jcmd.K0sJoinCommand, "controller"), jcmd.InstallationSpec.Proxy); err != nil {
				err := fmt.Errorf("unable to create systemd unit files: %w", err)
				metrics.ReportJoinFailed(cmd.Context(), jcmd.InstallationSpec.MetricsBaseURL, jcmd.ClusterID, err)
				return err
			}

			logrus.Debugf("overriding network configuration")
			if err := applyNetworkConfiguration(cmd, jcmd); err != nil {
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
			if err := runK0sInstallCommand(cmd, provider, jcmd.K0sJoinCommand); err != nil {
				err := fmt.Errorf("unable to join node to cluster: %w", err)
				metrics.ReportJoinFailed(cmd.Context(), jcmd.InstallationSpec.MetricsBaseURL, jcmd.ClusterID, err)
				return err
			}

			if err := startAndWaitForK0s(cmd, name, jcmd); err != nil {
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

			enabledHighAvailabilityFlag, err := cmd.Flags().GetBool("enable-ha")
			if err != nil {
				return fmt.Errorf("unable to get enable-ha flag: %w", err)
			}

			if enabledHighAvailabilityFlag {
				if err := maybeEnableHA(cmd.Context(), kcli); err != nil {
					err := fmt.Errorf("unable to enable high availability: %w", err)
					metrics.ReportJoinFailed(cmd.Context(), jcmd.InstallationSpec.MetricsBaseURL, jcmd.ClusterID, err)
					return err
				}
			}

			metrics.ReportJoinSucceeded(cmd.Context(), jcmd.InstallationSpec.MetricsBaseURL, jcmd.ClusterID)
			logrus.Debugf("controller node join finished")
			return nil
		},
	}

	cmd.Flags().StringVar(&airgapBundle, "airgap-bundle", "", "Path to the air gap bundle. If set, the installation will complete without internet access.")
	cmd.Flags().BoolVar(&enabledHighAvailability, "enable-ha", false, "Enable high availability.")
	cmd.Flags().MarkHidden("enable-ha")

	cmd.Flags().StringVar(&networkInterface, "network-interface", "", "The network interface to use for the cluster")
	cmd.Flags().BoolVar(&noPrompt, "no-prompt", false, "Disable interactive prompts.")
	cmd.Flags().BoolVar(&skipHostPreflights, "skip-host-preflights", false, "Skip host preflight checks. This is not recommended.")

	cmd.AddCommand(JoinRunPreflightsCmd(ctx, name))

	return cmd
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
func installK0sBinary(provider *defaults.Provider) error {
	ourbin := provider.PathToEmbeddedClusterBinary("k0s")
	hstbin := defaults.K0sBinaryPath()
	if err := helpers.MoveFile(ourbin, hstbin); err != nil {
		return fmt.Errorf("unable to move k0s binary: %w", err)
	}
	return nil
}

// startK0sService starts the k0s service.
func startK0sService() error {
	if _, err := helpers.RunCommand(defaults.K0sBinaryPath(), "start"); err != nil {
		return fmt.Errorf("unable to start: %w", err)
	}
	return nil
}

func applyNetworkConfiguration(cmd *cobra.Command, jcmd *JoinCommandResponse) error {
	if jcmd.InstallationSpec.Network != nil {
		clusterSpec := config.RenderK0sConfig()

		networkInterfaceFlag, err := cmd.Flags().GetString("network-interface")
		if err != nil {
			return fmt.Errorf("unable to get network-interface flag: %w", err)
		}
		address, err := netutils.FirstValidAddress(networkInterfaceFlag)
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
		err = os.WriteFile(defaults.PathToK0sConfig(), clusterSpecYaml, 0644)
		if err != nil {
			return fmt.Errorf("unable to write cluster spec to /etc/k0s/k0s.yaml: %w", err)
		}
	}
	return nil
}

// startAndWaitForK0s starts the k0s service and waits for the node to be ready.
func startAndWaitForK0s(cmd *cobra.Command, name string, jcmd *JoinCommandResponse) error {
	loading := spinner.Start()
	defer loading.Close()
	loading.Infof("Installing %s node", name)
	logrus.Debugf("starting %s service", name)
	if err := startK0sService(); err != nil {
		err := fmt.Errorf("unable to start service: %w", err)
		metrics.ReportJoinFailed(cmd.Context(), jcmd.InstallationSpec.MetricsBaseURL, jcmd.ClusterID, err)
		return err
	}

	loading.Infof("Waiting for %s node to be ready", name)
	logrus.Debugf("waiting for k0s to be ready")
	if err := waitForK0s(); err != nil {
		err := fmt.Errorf("unable to wait for node: %w", err)
		metrics.ReportJoinFailed(cmd.Context(), jcmd.InstallationSpec.MetricsBaseURL, jcmd.ClusterID, err)
		return err
	}

	loading.Infof("Node installation finished!")
	return nil
}

// applyJoinConfigurationOverrides applies both config overrides received from the kots api.
// Applies first the EmbeddedOverrides and then the EndUserOverrides.
func applyJoinConfigurationOverrides(jcmd *JoinCommandResponse) error {
	patch, err := jcmd.EmbeddedOverrides()
	if err != nil {
		return fmt.Errorf("unable to get embedded overrides: %w", err)
	}
	if len(patch) > 0 {
		if data, err := yaml.Marshal(patch); err != nil {
			return fmt.Errorf("unable to marshal embedded overrides: %w", err)
		} else if err := patchK0sConfig(
			defaults.PathToK0sConfig(), string(data),
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
	} else if err := patchK0sConfig(
		defaults.PathToK0sConfig(), string(data),
	); err != nil {
		return fmt.Errorf("unable to patch config with embedded data: %w", err)
	}
	return nil
}

// patchK0sConfig patches the created k0s config with the unsupported overrides passed in.
func patchK0sConfig(path string, patch string) error {
	if len(patch) == 0 {
		return nil
	}
	finalcfg := k0sconfig.ClusterConfig{
		ObjectMeta: metav1.ObjectMeta{Name: defaults.BinaryName()},
	}
	if _, err := os.Stat(path); err == nil {
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("unable to read node config: %w", err)
		}
		finalcfg = k0sconfig.ClusterConfig{}
		if err := k8syaml.Unmarshal(data, &finalcfg); err != nil {
			return fmt.Errorf("unable to unmarshal node config: %w", err)
		}
	}
	result, err := config.PatchK0sConfig(finalcfg.DeepCopy(), patch)
	if err != nil {
		return fmt.Errorf("unable to patch node config: %w", err)
	}
	if result.Spec.API != nil {
		if finalcfg.Spec == nil {
			finalcfg.Spec = &k0sconfig.ClusterSpec{}
		}
		finalcfg.Spec.API = result.Spec.API
	}
	if result.Spec.Storage != nil {
		if finalcfg.Spec == nil {
			finalcfg.Spec = &k0sconfig.ClusterSpec{}
		}
		finalcfg.Spec.Storage = result.Spec.Storage
	}
	// This is necessary to install the previous version of k0s in e2e tests
	// TODO: remove this once the previous version is > 1.29
	unstructured, err := helpers.K0sClusterConfigTo129Compat(&finalcfg)
	if err != nil {
		return fmt.Errorf("unable to convert cluster config to 1.29 compat: %w", err)
	}
	data, err := k8syaml.Marshal(unstructured)
	if err != nil {
		return fmt.Errorf("unable to marshal node config: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("unable to write node config file: %w", err)
	}
	return nil
}

// runK0sInstallCommand runs the k0s install command as provided by the kots
// adm api.
func runK0sInstallCommand(cmd *cobra.Command, provider *defaults.Provider, fullcmd string) error {
	args := strings.Split(fullcmd, " ")
	args = append(args, "--token-file", "/etc/k0s/join-token")

	networkInterfaceFlag, err := cmd.Flags().GetString("network-interface")
	if err != nil {
		return fmt.Errorf("unable to get network-interface flag: %w", err)
	}
	nodeIP, err := netutils.FirstValidAddress(networkInterfaceFlag)
	if err != nil {
		return fmt.Errorf("unable to find first valid address: %w", err)
	}

	args = append(args, config.AdditionalInstallFlags(provider, nodeIP)...)

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

func maybeEnableHA(ctx context.Context, kcli client.Client) error {
	canEnableHA, err := highavailability.CanEnableHA(ctx, kcli)
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
	return highavailability.EnableHA(ctx, kcli)
}
